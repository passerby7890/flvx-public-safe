package forwarder

import "testing"

func TestTLSSelectHeaderAddsALPNAndBrowserMarker(t *testing.T) {
	header := tlsSelectHeader([]string{"h2", "http/1.1"})

	if got := header.Values(tlsALPNSelectHeader); len(got) != 2 || got[0] != "h2" || got[1] != "http/1.1" {
		t.Fatalf("unexpected ALPN header values: %#v", got)
	}
	if got := header.Get(tlsBrowserSelectHeader); got != "1" {
		t.Fatalf("expected browser marker, got %q", got)
	}
}

func TestTLSSelectHeaderDoesNotMarkEmptyALPNAsBrowser(t *testing.T) {
	header := tlsSelectHeader(nil)

	if got := header.Values(tlsALPNSelectHeader); len(got) != 0 {
		t.Fatalf("expected no ALPN values, got %#v", got)
	}
	if got := header.Get(tlsBrowserSelectHeader); got != "" {
		t.Fatalf("expected no browser marker, got %q", got)
	}
}
