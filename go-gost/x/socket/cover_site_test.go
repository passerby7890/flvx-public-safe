package socket

import (
	"strings"
	"testing"
)

func TestRenderCoverNginxConfigUsesUpstreamHostForProxy(t *testing.T) {
	config, err := renderCoverNginxConfig("127.0.0.1:18443", []CoverSiteSyncProfile{
		{
			Domains:        []string{"*.example-entry.test", "example-entry.test"},
			CertProfile:    "default-entry-cover",
			UpstreamOrigin: "https://ezbid.tw",
		},
	})
	if err != nil {
		t.Fatalf("render config: %v", err)
	}
	if !strings.Contains(config, "proxy_set_header Host ezbid.tw;") {
		t.Fatalf("expected upstream Host header, got:\n%s", config)
	}
	if !strings.Contains(config, "proxy_ssl_name ezbid.tw;") {
		t.Fatalf("expected upstream TLS SNI, got:\n%s", config)
	}
	if strings.Contains(config, "proxy_set_header Host $host;") {
		t.Fatalf("must not forward cover domain as upstream Host, got:\n%s", config)
	}
}
