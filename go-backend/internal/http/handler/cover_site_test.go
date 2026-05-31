package handler

import "testing"

func TestCoverDomainsOverlap(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "same wildcard", a: "*.example.com", b: "*.example.com", want: true},
		{name: "wildcard covers subdomain", a: "*.example.com", b: "hk.example.com", want: true},
		{name: "wildcard does not cover apex", a: "*.example.com", b: "example.com", want: false},
		{name: "different domains", a: "*.example.com", b: "*.example.net", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coverDomainsOverlap(tc.a, tc.b); got != tc.want {
				t.Fatalf("coverDomainsOverlap(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
