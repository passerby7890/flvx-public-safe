package socket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	coverRootDir           = "/etc/flux_agent/cover"
	coverNginxConfig       = "/etc/nginx/conf.d/flvx-cover.conf"
	coverDefaultSite       = "/etc/nginx/sites-enabled/default"
	coverDefaultSiteBackup = "/etc/flux_agent/cover/default-site.disabled"
	coverLegacyDefaultSite = "/etc/nginx/sites-enabled/default.flvx-disabled"
)

type CoverSiteSyncRequest struct {
	Enabled     bool                   `json:"enabled"`
	PublicPort  int                    `json:"publicPort"`
	LocalListen string                 `json:"localListen"`
	Profiles    []CoverSiteSyncProfile `json:"profiles"`
}

type CoverSiteSyncProfile struct {
	TunnelID        int64    `json:"tunnelId"`
	SiteLabel       string   `json:"siteLabel"`
	Domains         []string `json:"domains"`
	CertProfile     string   `json:"certProfile"`
	FullchainPEM    string   `json:"fullchainPem"`
	PrivateKeyPEM   string   `json:"privateKeyPem"`
	TemplateProfile string   `json:"templateProfile"`
	UpstreamOrigin  string   `json:"upstreamOrigin"`
	StaticHTML      string   `json:"staticHtml"`
}

func (w *WebSocketReporter) handleSyncCoverSite(data interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("serialize cover site data failed: %w", err)
	}
	var req CoverSiteSyncRequest
	if err := json.Unmarshal(jsonData, &req); err != nil {
		return nil, fmt.Errorf("parse cover site request failed: %w", err)
	}
	req.LocalListen = strings.TrimSpace(req.LocalListen)
	if req.LocalListen == "" {
		req.LocalListen = "127.0.0.1:10443"
	}
	if err := validateCoverListen(req.LocalListen); err != nil {
		return nil, err
	}

	if !req.Enabled || len(req.Profiles) == 0 {
		if err := removeManagedCoverConfig(); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"enabled":     false,
			"localListen": req.LocalListen,
			"profiles":    0,
			"message":     "cover site disabled",
		}, nil
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("cover site sync only supports linux nodes")
	}
	if err := ensureNginxInstalled(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(coverRootDir, "certs"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(coverRootDir, "sites"), 0755); err != nil {
		return nil, err
	}
	if err := disableNginxDefaultSite(); err != nil {
		return nil, err
	}

	activeProfiles := make([]CoverSiteSyncProfile, 0, len(req.Profiles))
	for _, profile := range req.Profiles {
		normalized, err := normalizeCoverProfile(profile)
		if err != nil {
			return nil, err
		}
		if err := writeCoverProfileFiles(normalized); err != nil {
			return nil, err
		}
		activeProfiles = append(activeProfiles, normalized)
	}
	if len(activeProfiles) == 0 {
		if err := removeManagedCoverConfig(); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"enabled":     false,
			"localListen": req.LocalListen,
			"profiles":    0,
			"message":     "no active cover profiles",
		}, nil
	}

	configText, err := renderCoverNginxConfig(req.LocalListen, activeProfiles)
	if err != nil {
		return nil, err
	}
	if err := writeCoverNginxConfig([]byte(configText)); err != nil {
		return nil, err
	}
	if err := reloadOrStartNginx(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled":     true,
		"localListen": req.LocalListen,
		"profiles":    len(activeProfiles),
		"message":     "cover site synced",
	}, nil
}

func validateCoverListen(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid cover local listen address")
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "::1" && strings.ToLower(host) != "localhost" {
		return fmt.Errorf("cover nginx must listen on loopback address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid cover local listen port")
	}
	return nil
}

func ensureNginxInstalled() error {
	if _, err := exec.LookPath("nginx"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("nginx is missing and apt-get is unavailable")
	}
	if _, err := runCoverCommand(2*time.Minute, "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	cmd := exec.Command("apt-get", "install", "-y", "nginx")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install nginx failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func disableNginxDefaultSite() error {
	if err := os.Remove(coverLegacyDefaultSite); err != nil && !os.IsNotExist(err) {
		return err
	}
	info, err := os.Lstat(coverDefaultSite)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	if _, err := os.Lstat(coverDefaultSiteBackup); err == nil {
		return os.Remove(coverDefaultSite)
	}
	return os.Rename(coverDefaultSite, coverDefaultSiteBackup)
}

func normalizeCoverProfile(profile CoverSiteSyncProfile) (CoverSiteSyncProfile, error) {
	profile.CertProfile = safeCoverName(profile.CertProfile)
	if profile.CertProfile == "" {
		return profile, fmt.Errorf("cover cert profile is required")
	}
	if !strings.Contains(profile.FullchainPEM, "BEGIN CERTIFICATE") {
		return profile, fmt.Errorf("cover fullchain is invalid")
	}
	if !strings.Contains(profile.PrivateKeyPEM, "PRIVATE KEY") {
		return profile, fmt.Errorf("cover private key is invalid")
	}
	domains := make([]string, 0, len(profile.Domains))
	seen := make(map[string]struct{}, len(profile.Domains))
	for _, domain := range profile.Domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if strings.ContainsAny(domain, " /\\:#") {
			return profile, fmt.Errorf("invalid cover domain: %s", domain)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	if len(domains) == 0 {
		return profile, fmt.Errorf("cover domains are required")
	}
	profile.Domains = domains
	if strings.TrimSpace(profile.UpstreamOrigin) != "" {
		parsed, err := url.Parse(strings.TrimSpace(profile.UpstreamOrigin))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return profile, fmt.Errorf("invalid cover upstream origin")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return profile, fmt.Errorf("cover upstream must be http or https")
		}
		profile.UpstreamOrigin = parsed.String()
	}
	return profile, nil
}

func writeCoverProfileFiles(profile CoverSiteSyncProfile) error {
	certDir := filepath.Join(coverRootDir, "certs", profile.CertProfile)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return err
	}
	if err := writeFileIfChanged(filepath.Join(certDir, "fullchain.pem"), []byte(profile.FullchainPEM), 0644); err != nil {
		return err
	}
	if err := writeFileIfChanged(filepath.Join(certDir, "privkey.pem"), []byte(profile.PrivateKeyPEM), 0600); err != nil {
		return err
	}

	if strings.TrimSpace(profile.UpstreamOrigin) != "" {
		return nil
	}
	siteName := coverSiteDirName(profile)
	siteDir := filepath.Join(coverRootDir, "sites", siteName)
	if err := os.MkdirAll(siteDir, 0755); err != nil {
		return err
	}
	html := strings.TrimSpace(profile.StaticHTML)
	if html == "" {
		html = "<!doctype html><html><head><meta charset=\"utf-8\"><title>Service</title></head><body><main><h1>Service Online</h1></main></body></html>"
	}
	return writeFileIfChanged(filepath.Join(siteDir, "index.html"), []byte(html), 0644)
}

func renderCoverNginxConfig(localListen string, profiles []CoverSiteSyncProfile) (string, error) {
	var b strings.Builder
	b.WriteString("# Managed by FLVX cover site. Do not edit manually.\n")
	for _, profile := range profiles {
		host, port, err := net.SplitHostPort(localListen)
		if err != nil {
			return "", err
		}
		host = strings.Trim(host, "[]")
		listenHost := host
		if strings.Contains(listenHost, ":") {
			listenHost = "[" + listenHost + "]"
		}
		certDir := filepath.Join(coverRootDir, "certs", profile.CertProfile)
		b.WriteString("server {\n")
		b.WriteString(fmt.Sprintf("    listen %s:%s ssl;\n", listenHost, port))
		b.WriteString(fmt.Sprintf("    server_name %s;\n", strings.Join(profile.Domains, " ")))
		b.WriteString(fmt.Sprintf("    ssl_certificate %s;\n", filepath.Join(certDir, "fullchain.pem")))
		b.WriteString(fmt.Sprintf("    ssl_certificate_key %s;\n", filepath.Join(certDir, "privkey.pem")))
		b.WriteString("    ssl_protocols TLSv1.2 TLSv1.3;\n")
		b.WriteString("    ssl_session_cache shared:FLVXCoverSSL:10m;\n")
		b.WriteString("    add_header X-Content-Type-Options nosniff always;\n")
		b.WriteString("    add_header Referrer-Policy strict-origin-when-cross-origin always;\n")
		if strings.TrimSpace(profile.UpstreamOrigin) != "" {
			upstreamURL, err := url.Parse(strings.TrimSpace(profile.UpstreamOrigin))
			if err != nil || upstreamURL.Host == "" {
				return "", fmt.Errorf("invalid cover upstream origin")
			}
			upstreamHost := upstreamURL.Host
			upstreamSNI := upstreamURL.Hostname()
			b.WriteString("    location / {\n")
			b.WriteString(fmt.Sprintf("        proxy_pass %s;\n", strings.TrimSpace(profile.UpstreamOrigin)))
			b.WriteString("        proxy_http_version 1.1;\n")
			b.WriteString(fmt.Sprintf("        proxy_set_header Host %s;\n", upstreamHost))
			b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
			b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
			b.WriteString("        proxy_set_header X-Forwarded-Proto https;\n")
			b.WriteString("        proxy_ssl_server_name on;\n")
			if upstreamSNI != "" {
				b.WriteString(fmt.Sprintf("        proxy_ssl_name %s;\n", upstreamSNI))
			}
			b.WriteString("    }\n")
		} else {
			siteDir := filepath.Join(coverRootDir, "sites", coverSiteDirName(profile))
			b.WriteString(fmt.Sprintf("    root %s;\n", siteDir))
			b.WriteString("    index index.html;\n")
			b.WriteString("    location / { try_files $uri $uri/ /index.html; }\n")
		}
		b.WriteString("}\n\n")
	}
	return b.String(), nil
}

func writeCoverNginxConfig(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(coverNginxConfig), 0755); err != nil {
		return err
	}
	previous, hadPrevious := os.ReadFile(coverNginxConfig)
	if hadPrevious == nil && bytes.Equal(previous, data) {
		return nil
	}
	if err := os.WriteFile(coverNginxConfig, data, 0644); err != nil {
		return err
	}
	if _, err := runCoverCommand(30*time.Second, "nginx", "-t"); err != nil {
		if hadPrevious == nil {
			_ = os.WriteFile(coverNginxConfig, previous, 0644)
		} else {
			_ = os.Remove(coverNginxConfig)
		}
		return err
	}
	return nil
}

func removeManagedCoverConfig() error {
	_, nginxErr := exec.LookPath("nginx")
	if err := os.Remove(coverNginxConfig); err != nil && !os.IsNotExist(err) {
		return err
	}
	if nginxErr == nil {
		if _, err := runCoverCommand(30*time.Second, "nginx", "-t"); err != nil {
			return err
		}
		if err := reloadOrStartNginx(); err != nil {
			return err
		}
	}
	return nil
}

func reloadOrStartNginx() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		if _, err := runCoverCommand(30*time.Second, "systemctl", "reload", "nginx"); err == nil {
			return nil
		}
		if _, err := runCoverCommand(30*time.Second, "systemctl", "restart", "nginx"); err == nil {
			return nil
		}
	}
	if _, err := runCoverCommand(30*time.Second, "nginx", "-s", "reload"); err == nil {
		return nil
	}
	_, err := runCoverCommand(30*time.Second, "nginx")
	return err
}

func runCoverCommand(timeout time.Duration, name string, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return text, ctx.Err()
	}
	if err != nil {
		if text == "" {
			return text, err
		}
		return text, fmt.Errorf("%w: %s", err, text)
	}
	return text, nil
}

func writeFileIfChanged(path string, data []byte, perm os.FileMode) error {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, data) {
		return os.Chmod(path, perm)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func coverSiteDirName(profile CoverSiteSyncProfile) string {
	if len(profile.Domains) > 0 {
		return safeCoverName(strings.TrimPrefix(profile.Domains[0], "*."))
	}
	if profile.TunnelID > 0 {
		return fmt.Sprintf("tunnel-%d", profile.TunnelID)
	}
	return "default"
}

func safeCoverName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), ".-")
}
