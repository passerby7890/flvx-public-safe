package handler

import (
	"fmt"
	"sort"
	"strings"

	"go-backend/internal/store/model"
)

const (
	forwardModeDirect = "direct"
	forwardModeSNI    = "sni"
	sniTLSDemuxBase   = 11000
)

func normalizeForwardMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", forwardModeDirect:
		return forwardModeDirect, nil
	case forwardModeSNI:
		return forwardModeSNI, nil
	default:
		return "", fmt.Errorf("invalid forward mode: %s", value)
	}
}

func normalizeStoredForwardMode(value string) string {
	mode, err := normalizeForwardMode(value)
	if err != nil {
		return forwardModeDirect
	}
	return mode
}

func isSNIForwardMode(value string) bool {
	return normalizeStoredForwardMode(value) == forwardModeSNI
}

func parseSNIForwardHosts(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	lines := strings.Split(raw, "\n")
	var hosts []string

	for idx, line := range lines {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		host := strings.TrimSpace(line)
		if host == "" {
			continue
		}

		normHost, err := normalizeSNIForwardHost(host)
		if err != nil {
			return nil, fmt.Errorf("invalid SNI host '%s' on line %d: %w", host, idx+1, err)
		}

		hosts = append(hosts, normHost)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("at least one SNI host is required")
	}

	return hosts, nil
}

func normalizeSNIForwardHost(host string) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.ContainsAny(host, " /\\:#") {
		return "", fmt.Errorf("host must be a plain SNI hostname")
	}
	for _, r := range host {
		if r > 127 {
			return "", fmt.Errorf("host must be ASCII")
		}
	}
	if host == "*" || host == "." || strings.HasSuffix(host, ".") {
		return "", fmt.Errorf("invalid host pattern")
	}

	wildcard := false
	if strings.HasPrefix(host, "*.") {
		wildcard = true
		host = strings.TrimPrefix(host, "*.")
	}
	if strings.Contains(host, "*") {
		return "", fmt.Errorf("wildcard is only allowed as a leading *.")
	}
	if !isValidSNIHostname(host) {
		return "", fmt.Errorf("invalid host pattern")
	}
	if wildcard {
		return "*." + host, nil
	}
	return host, nil
}

func deriveForwardRemoteAddr(mode, remoteAddr, sniRules string) (string, error) {
	if isSNIForwardMode(mode) {
		if _, err := parseSNIForwardHosts(sniRules); err != nil {
			return "", err
		}
	}

	targets, err := normalizeForwardRemoteTargets(remoteAddr)
	if err != nil {
		return "", err
	}
	return strings.Join(targets, ","), nil
}

type sniCoverForwardProfile struct {
	TunnelID    int64
	Domains     []string
	LocalListen string
}

func buildSNISharedForwarderNodes(sniForwards []model.ForwardRecord, coverProfiles []sniCoverForwardProfile) ([]map[string]interface{}, error) {
	var nodes []map[string]interface{}
	coverDomains := collectSNICoverDomains(coverProfiles)

	for _, f := range sniForwards {
		hosts, err := parseSNIForwardHosts(f.SniRules)
		if err != nil {
			continue
		}
		hiddenPort := 20000 + (f.ID % 40000)
		targetAddr := fmt.Sprintf("127.0.0.1:%d", hiddenPort)

		for j, host := range hosts {
			nodeAddr := targetAddr
			if sniHostCoveredByProfile(host, coverDomains) {
				nodeAddr = sniTLSDemuxAddr(f.ID)
			}
			nodes = append(nodes, map[string]interface{}{
				"name": fmt.Sprintf("sni_%d_%d", f.ID, j+1),
				"addr": nodeAddr,
				"filter": map[string]interface{}{
					"host":     host,
					"protocol": "tls",
				},
			})
		}
	}

	// Active probing fallback
	nodes = append(nodes, map[string]interface{}{
		"name": "sni_fallback",
		"addr": "microsoft.com:443",
		"metadata": map[string]interface{}{
			"backup": true,
		},
	})

	return nodes, nil
}

func sniTLSDemuxAddr(forwardID int64) string {
	return fmt.Sprintf("127.0.0.1:%d", sniTLSDemuxBase+int(forwardID%40000))
}

func collectSNICoverDomains(coverProfiles []sniCoverForwardProfile) []string {
	seen := make(map[string]struct{})
	var domains []string
	for _, profile := range coverProfiles {
		for _, domain := range profile.Domains {
			normalized, err := normalizeSNIForwardHost(domain)
			if err != nil || normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			domains = append(domains, normalized)
		}
	}
	sort.Strings(domains)
	return domains
}

func sniHostCoveredByProfile(host string, coverDomains []string) bool {
	normalized, err := normalizeSNIForwardHost(host)
	if err != nil || normalized == "" {
		return false
	}
	for _, domain := range coverDomains {
		if coverDomainsOverlap(domain, normalized) || domain == normalized {
			return true
		}
	}
	return false
}

func parseCoverProfileDomains(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	lines := strings.Split(raw, "\n")
	hosts := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))

	for idx, line := range lines {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		host := strings.TrimSpace(line)
		if host == "" {
			continue
		}
		normHost, err := normalizeSNIForwardHost(host)
		if err != nil {
			return nil, fmt.Errorf("invalid cover domain '%s' on line %d: %w", host, idx+1, err)
		}
		if _, ok := seen[normHost]; ok {
			continue
		}
		seen[normHost] = struct{}{}
		hosts = append(hosts, normHost)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("at least one cover domain is required")
	}
	return hosts, nil
}

func isValidSNIHostname(host string) bool {
	if host == "" || strings.Contains(host, "..") {
		return false
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return false
		}
	}

	return true
}

func normalizeForwardRemoteTargets(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	lines := strings.Split(raw, "\n")
	targets := make([]string, 0, len(lines))

	for idx, line := range lines {
		target := strings.TrimSpace(line)
		if target == "" {
			continue
		}

		normalized := processServerAddress(target)
		if _, _, err := parseTargetAddress(normalized); err != nil {
			return nil, fmt.Errorf("invalid remote target '%s' on line %d: %w", target, idx+1, err)
		}
		targets = append(targets, normalized)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one remote target is required")
	}

	return targets, nil
}
