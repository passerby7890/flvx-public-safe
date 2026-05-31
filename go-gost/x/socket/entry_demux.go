package socket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	entryDemuxConfigPath = "/etc/flux_agent/cover/flvx-entry-demux.cfg"
	entryDemuxUnitPath   = "/etc/systemd/system/flvx-entry-demux.service"
	entryDemuxCertDir    = "/etc/flux_agent/cover/demux-certs"
	entryDemuxPIDPath    = "/run/flvx-entry-demux.pid"
	entryDemuxService    = "flvx-entry-demux.service"
)

type EntryDemuxSyncRequest struct {
	Enabled   bool                 `json:"enabled"`
	Listeners []EntryDemuxListener `json:"listeners"`
}

type EntryDemuxListener struct {
	Name       string            `json:"name"`
	Listen     string            `json:"listen"`
	CoverAddr  string            `json:"coverAddr"`
	AnyTLSAddr string            `json:"anytlsAddr"`
	Certs      []EntryDemuxCert  `json:"certs"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type EntryDemuxCert struct {
	Profile       string `json:"profile"`
	FullchainPEM  string `json:"fullchainPem"`
	PrivateKeyPEM string `json:"privateKeyPem"`
}

func (w *WebSocketReporter) handleSyncEntryDemux(data interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("serialize entry demux data failed: %w", err)
	}
	var req EntryDemuxSyncRequest
	if err := json.Unmarshal(jsonData, &req); err != nil {
		return nil, fmt.Errorf("parse entry demux request failed: %w", err)
	}

	if !req.Enabled || len(req.Listeners) == 0 {
		if err := removeManagedEntryDemux(); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"enabled":   false,
			"listeners": 0,
			"message":   "entry demux disabled",
		}, nil
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("entry demux sync only supports linux nodes")
	}
	if err := ensureHAProxyInstalled(); err != nil {
		return nil, err
	}

	listeners := make([]EntryDemuxListener, 0, len(req.Listeners))
	for _, listener := range req.Listeners {
		normalized, err := normalizeEntryDemuxListener(listener)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, normalized)
	}
	if len(listeners) == 0 {
		if err := removeManagedEntryDemux(); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"enabled":   false,
			"listeners": 0,
			"message":   "no active entry demux listeners",
		}, nil
	}

	if err := writeEntryDemuxCerts(listeners); err != nil {
		return nil, err
	}
	configText, err := renderEntryDemuxHAProxyConfig(listeners)
	if err != nil {
		return nil, err
	}
	if err := writeEntryDemuxConfig([]byte(configText)); err != nil {
		return nil, err
	}
	if err := writeEntryDemuxSystemdUnit(); err != nil {
		return nil, err
	}
	if err := restartEntryDemux(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled":   true,
		"listeners": len(listeners),
		"message":   "entry demux synced",
	}, nil
}

func normalizeEntryDemuxListener(listener EntryDemuxListener) (EntryDemuxListener, error) {
	listener.Name = safeCoverName(listener.Name)
	if listener.Name == "" {
		return listener, fmt.Errorf("entry demux listener name is required")
	}
	if err := validateLoopbackListen(listener.Listen); err != nil {
		return listener, fmt.Errorf("invalid entry demux listen for %s: %w", listener.Name, err)
	}
	if err := validateLoopbackListen(listener.CoverAddr); err != nil {
		return listener, fmt.Errorf("invalid entry demux cover backend for %s: %w", listener.Name, err)
	}
	if err := validateLoopbackListen(listener.AnyTLSAddr); err != nil {
		return listener, fmt.Errorf("invalid entry demux anytls backend for %s: %w", listener.Name, err)
	}
	if len(listener.Certs) == 0 {
		return listener, fmt.Errorf("entry demux listener %s requires a certificate", listener.Name)
	}

	certs := make([]EntryDemuxCert, 0, len(listener.Certs))
	seen := make(map[string]struct{}, len(listener.Certs))
	for _, cert := range listener.Certs {
		cert.Profile = safeCoverName(cert.Profile)
		if cert.Profile == "" {
			return listener, fmt.Errorf("entry demux certificate profile is required")
		}
		if !strings.Contains(cert.FullchainPEM, "BEGIN CERTIFICATE") {
			return listener, fmt.Errorf("entry demux certificate %s fullchain is invalid", cert.Profile)
		}
		if !strings.Contains(cert.PrivateKeyPEM, "PRIVATE KEY") {
			return listener, fmt.Errorf("entry demux certificate %s private key is invalid", cert.Profile)
		}
		if _, ok := seen[cert.Profile]; ok {
			continue
		}
		seen[cert.Profile] = struct{}{}
		certs = append(certs, cert)
	}
	listener.Certs = certs
	return listener, nil
}

func validateLoopbackListen(addr string) error {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return fmt.Errorf("address must be host:port")
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "::1" && strings.ToLower(host) != "localhost" {
		return fmt.Errorf("address must be loopback")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func writeEntryDemuxCerts(listeners []EntryDemuxListener) error {
	if err := os.MkdirAll(entryDemuxCertDir, 0755); err != nil {
		return err
	}
	seen := make(map[string]EntryDemuxCert)
	for _, listener := range listeners {
		for _, cert := range listener.Certs {
			if _, ok := seen[cert.Profile]; ok {
				continue
			}
			seen[cert.Profile] = cert
		}
	}
	for _, cert := range seen {
		pem := strings.TrimSpace(cert.FullchainPEM) + "\n" + strings.TrimSpace(cert.PrivateKeyPEM) + "\n"
		if err := writeFileIfChanged(filepath.Join(entryDemuxCertDir, cert.Profile+".pem"), []byte(pem), 0600); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(entryDemuxCertDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pem") {
			continue
		}
		profile := strings.TrimSuffix(entry.Name(), ".pem")
		if _, ok := seen[profile]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(entryDemuxCertDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func renderEntryDemuxHAProxyConfig(listeners []EntryDemuxListener) (string, error) {
	if len(listeners) == 0 {
		return "", fmt.Errorf("entry demux listeners are required")
	}

	var b strings.Builder
	b.WriteString("# Managed by FLVX entry demux. Do not edit manually.\n")
	b.WriteString("global\n")
	b.WriteString("    maxconn 8192\n")
	b.WriteString("\n")
	b.WriteString("defaults\n")
	b.WriteString("    mode tcp\n")
	b.WriteString("    timeout connect 5s\n")
	b.WriteString("    timeout client 1h\n")
	b.WriteString("    timeout server 1h\n")
	b.WriteString("    timeout tunnel 1h\n")
	b.WriteString("\n")

	for _, listener := range listeners {
		name := safeCoverName(listener.Name)
		if name == "" {
			return "", fmt.Errorf("invalid listener name")
		}
		b.WriteString(fmt.Sprintf("frontend %s\n", name))
		b.WriteString(fmt.Sprintf("    bind %s ssl crt %s alpn http/1.1\n", listener.Listen, entryDemuxCertDir))
		b.WriteString("    tcp-request inspect-delay 800ms\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,4) -m str \"GET \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,5) -m str \"HEAD \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,5) -m str \"POST \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,4) -m str \"PUT \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,6) -m str \"PATCH \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,6) -m str \"TRACE \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,7) -m str \"DELETE \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,8) -m str \"OPTIONS \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,8) -m str \"CONNECT \" }\n")
		b.WriteString("    tcp-request content accept if { req.payload(0,24) -m str \"PRI * HTTP/2.0\\r\\n\\r\\nSM\\r\\n\\r\\n\" }\n")
		b.WriteString("    acl http_get     req.payload(0,4)  -m str \"GET \"\n")
		b.WriteString("    acl http_head    req.payload(0,5)  -m str \"HEAD \"\n")
		b.WriteString("    acl http_post    req.payload(0,5)  -m str \"POST \"\n")
		b.WriteString("    acl http_put     req.payload(0,4)  -m str \"PUT \"\n")
		b.WriteString("    acl http_patch   req.payload(0,6)  -m str \"PATCH \"\n")
		b.WriteString("    acl http_trace   req.payload(0,6)  -m str \"TRACE \"\n")
		b.WriteString("    acl http_delete  req.payload(0,7)  -m str \"DELETE \"\n")
		b.WriteString("    acl http_options req.payload(0,8)  -m str \"OPTIONS \"\n")
		b.WriteString("    acl http_connect req.payload(0,8)  -m str \"CONNECT \"\n")
		b.WriteString("    acl http2_pref   req.payload(0,24) -m str \"PRI * HTTP/2.0\\r\\n\\r\\nSM\\r\\n\\r\\n\"\n")
		b.WriteString(fmt.Sprintf("    use_backend %s_cover if http_get or http_head or http_post or http_put or http_patch or http_trace or http_delete or http_options or http_connect or http2_pref\n", name))
		b.WriteString(fmt.Sprintf("    default_backend %s_anytls\n", name))
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("backend %s_cover\n", name))
		b.WriteString(fmt.Sprintf("    server cover %s ssl verify none sni ssl_fc_sni alpn http/1.1\n", listener.CoverAddr))
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("backend %s_anytls\n", name))
		b.WriteString(fmt.Sprintf("    server anytls %s ssl verify none sni ssl_fc_sni alpn http/1.1\n", listener.AnyTLSAddr))
		b.WriteString("\n")
	}
	return b.String(), nil
}

func writeEntryDemuxConfig(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(entryDemuxConfigPath), 0755); err != nil {
		return err
	}
	previous, hadPrevious := os.ReadFile(entryDemuxConfigPath)
	if hadPrevious == nil && bytes.Equal(previous, data) {
		return nil
	}
	if err := os.WriteFile(entryDemuxConfigPath, data, 0644); err != nil {
		return err
	}
	if _, err := runEntryDemuxCommand(30*time.Second, "haproxy", "-c", "-f", entryDemuxConfigPath); err != nil {
		if hadPrevious == nil {
			_ = os.WriteFile(entryDemuxConfigPath, previous, 0644)
		} else {
			_ = os.Remove(entryDemuxConfigPath)
		}
		return err
	}
	return nil
}

func writeEntryDemuxSystemdUnit() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	unit := `[Unit]
Description=FLVX entry TLS demux
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/sbin/haproxy -W -db -f /etc/flux_agent/cover/flvx-entry-demux.cfg -p /run/flvx-entry-demux.pid
ExecReload=/bin/kill -USR2 $MAINPID
Restart=always
RestartSec=2s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`
	if err := writeFileIfChanged(entryDemuxUnitPath, []byte(unit), 0644); err != nil {
		return err
	}
	_, err := runEntryDemuxCommand(30*time.Second, "systemctl", "daemon-reload")
	return err
}

func restartEntryDemux() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		_, _ = runEntryDemuxCommand(30*time.Second, "systemctl", "enable", entryDemuxService)
		if _, err := runEntryDemuxCommand(30*time.Second, "systemctl", "restart", entryDemuxService); err == nil {
			return nil
		}
	}
	_, _ = runEntryDemuxCommand(10*time.Second, "pkill", "-F", entryDemuxPIDPath)
	_, err := runEntryDemuxCommand(30*time.Second, "haproxy", "-f", entryDemuxConfigPath, "-D", "-p", entryDemuxPIDPath)
	return err
}

func removeManagedEntryDemux() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		_, _ = runEntryDemuxCommand(30*time.Second, "systemctl", "disable", "--now", entryDemuxService)
	}
	_, _ = runEntryDemuxCommand(10*time.Second, "pkill", "-F", entryDemuxPIDPath)
	if err := os.Remove(entryDemuxConfigPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		_, _ = runEntryDemuxCommand(30*time.Second, "systemctl", "daemon-reload")
	}
	return nil
}

func ensureHAProxyInstalled() error {
	if _, err := exec.LookPath("haproxy"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("haproxy is missing and apt-get is unavailable")
	}
	if _, err := runEntryDemuxCommand(2*time.Minute, "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	cmd := exec.Command("apt-get", "install", "-y", "haproxy")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install haproxy failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runEntryDemuxCommand(timeout time.Duration, name string, args ...string) (string, error) {
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
