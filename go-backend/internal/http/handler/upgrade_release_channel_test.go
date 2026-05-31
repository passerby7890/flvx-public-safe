package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-backend/internal/store/repo"
)

func TestReleaseChannelFromTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		expects string
	}{
		{name: "stable semantic version", tag: "2.1.4", expects: releaseChannelStable},
		{name: "v prefix should be dev", tag: "v2.1.4", expects: releaseChannelDev},
		{name: "rc release", tag: "2.1.4-rc2", expects: releaseChannelDev},
		{name: "beta release", tag: "2.1.4-beta.1", expects: releaseChannelDev},
		{name: "alpha release", tag: "2.1.4-alpha", expects: releaseChannelDev},
		{name: "non numeric tag", tag: "nightly", expects: releaseChannelDev},
		{name: "empty tag", tag: "", expects: releaseChannelDev},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := releaseChannelFromTag(tc.tag); got != tc.expects {
				t.Fatalf("releaseChannelFromTag(%q) = %q, want %q", tc.tag, got, tc.expects)
			}
		})
	}
}

func TestNormalizeReleaseChannel(t *testing.T) {
	tests := []struct {
		input   string
		expects string
	}{
		{input: "", expects: releaseChannelStable},
		{input: "stable", expects: releaseChannelStable},
		{input: "dev", expects: releaseChannelDev},
		{input: "DEV", expects: releaseChannelDev},
		{input: "patched", expects: releaseChannelPatched},
		{input: "panel", expects: releaseChannelPatched},
		{input: "preview", expects: releaseChannelStable},
	}

	for _, tc := range tests {
		if got := normalizeReleaseChannel(tc.input); got != tc.expects {
			t.Fatalf("normalizeReleaseChannel(%q) = %q, want %q", tc.input, got, tc.expects)
		}
	}
}

func TestNormalizeConfiguredBaseURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "https://panel.example.com:8443/path", want: "https://panel.example.com:8443"},
		{in: "panel.example.com:6366", want: "http://panel.example.com:6366"},
		{in: "[2001:db8::1]:6366", want: "http://[2001:db8::1]:6366"},
	}

	for _, tc := range tests {
		if got := normalizeConfiguredBaseURL(tc.in); got != tc.want {
			t.Fatalf("normalizeConfiguredBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeriveAgentDownloadBaseURLFromNodeAddress(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "203.0.113.10:6365", want: "http://203.0.113.10:6366"},
		{in: "panel.example.com:6365", want: "http://panel.example.com:6366"},
		{in: "https://panel.example.com:6365/api/v1", want: "http://panel.example.com:6366"},
		{in: "[2001:db8::1]:6365", want: "http://[2001:db8::1]:6366"},
	}

	for _, tc := range tests {
		if got := deriveAgentDownloadBaseURLFromNodeAddress(tc.in); got != tc.want {
			t.Fatalf("deriveAgentDownloadBaseURLFromNodeAddress(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolvePreferredAgentUpgradePatched(t *testing.T) {
	r, err := repo.Open(":memory:")
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	defer r.Close()

	h := New(r, "test-jwt-secret")
	req := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/v1/node/upgrade", nil)

	version, downloadURL, checksumURL, local, err := h.resolvePreferredAgentUpgrade("patched", "", req)
	if err != nil {
		t.Fatalf("resolvePreferredAgentUpgrade patched: %v", err)
	}
	if !local {
		t.Fatalf("expected local=true for patched channel")
	}
	if version != panelPatchedAgentVersion {
		t.Fatalf("expected version=%q, got %q", panelPatchedAgentVersion, version)
	}
	if !strings.Contains(downloadURL, "/agent/gost-{ARCH}") {
		t.Fatalf("unexpected download URL: %q", downloadURL)
	}
	if !strings.Contains(checksumURL, "/agent/gost-{ARCH}.sha256") {
		t.Fatalf("unexpected checksum URL: %q", checksumURL)
	}
}

func TestResolvePreferredAgentUpgradePatchedRejectsDifferentVersion(t *testing.T) {
	r, err := repo.Open(":memory:")
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	defer r.Close()

	h := New(r, "test-jwt-secret")
	req := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/v1/node/upgrade", nil)

	_, _, _, _, err = h.resolvePreferredAgentUpgrade("patched", "flvx-v3.2.6-dev1", req)
	if err == nil {
		t.Fatalf("expected error for mismatched patched version")
	}
}
