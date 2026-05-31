package socket

import (
	"strings"
	"testing"
)

func TestRenderEntryDemuxHAProxyConfigDefaultsUnknownTrafficToAnyTLS(t *testing.T) {
	config, err := renderEntryDemuxHAProxyConfig([]EntryDemuxListener{
		{
			Name:       "forward-123",
			Listen:     "127.0.0.1:11123",
			CoverAddr:  "127.0.0.1:10443",
			AnyTLSAddr: "127.0.0.1:20123",
			Certs: []EntryDemuxCert{
				{Profile: "default-entry-cover", FullchainPEM: "-----BEGIN CERTIFICATE-----\n", PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\n"},
			},
		},
	})
	if err != nil {
		t.Fatalf("render config: %v", err)
	}

	for _, expected := range []string{
		"bind 127.0.0.1:11123 ssl crt /etc/flux_agent/cover/demux-certs alpn http/1.1",
		"tcp-request inspect-delay 800ms",
		"tcp-request content accept if { req.payload(0,4) -m str \"GET \" }",
		"acl http_get     req.payload(0,4)  -m str \"GET \"",
		"use_backend forward-123_cover if http_get",
		"default_backend forward-123_anytls",
		"server cover 127.0.0.1:10443 ssl verify none sni ssl_fc_sni alpn http/1.1",
		"server anytls 127.0.0.1:20123 ssl verify none sni ssl_fc_sni alpn http/1.1",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("expected config to contain %q, got:\n%s", expected, config)
		}
	}
}

func TestNormalizeEntryDemuxListenerRequiresLoopbackBackends(t *testing.T) {
	_, err := normalizeEntryDemuxListener(EntryDemuxListener{
		Name:       "bad",
		Listen:     "127.0.0.1:11123",
		CoverAddr:  "10.0.0.1:10443",
		AnyTLSAddr: "127.0.0.1:20123",
		Certs: []EntryDemuxCert{
			{Profile: "cert", FullchainPEM: "-----BEGIN CERTIFICATE-----\n", PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\n"},
		},
	})
	if err == nil {
		t.Fatal("expected non-loopback cover backend to fail validation")
	}
}
