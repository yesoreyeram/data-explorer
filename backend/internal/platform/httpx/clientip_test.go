package httpx

import (
	"net/http"
	"testing"
)

func reqWith(remote, xff string) *http.Request {
	r := &http.Request{Header: http.Header{}, RemoteAddr: remote}
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestClientIP_NoneIgnoresXFF(t *testing.T) {
	if err := ConfigureClientIP("none"); err != nil {
		t.Fatal(err)
	}
	// A spoofed XFF must be ignored; the socket peer (sans port) wins.
	got := ClientIP(reqWith("203.0.113.7:44321", "1.2.3.4"))
	if got != "203.0.113.7" {
		t.Fatalf("none mode = %q, want socket peer 203.0.113.7", got)
	}
}

func TestClientIP_XFFDepth(t *testing.T) {
	if err := ConfigureClientIP("xff-depth:1"); err != nil {
		t.Fatal(err)
	}
	// One trusted proxy: XFF = "client, proxy1"; the (N+1)th from the right
	// with N=1 is "client".
	got := ClientIP(reqWith("10.0.0.9:5000", "9.9.9.9, 10.0.0.9"))
	if got != "9.9.9.9" {
		t.Fatalf("xff-depth:1 = %q, want 9.9.9.9", got)
	}
	// A shorter chain than the configured depth falls back to the peer.
	got = ClientIP(reqWith("10.0.0.9:5000", ""))
	if got != "10.0.0.9" {
		t.Fatalf("xff-depth fallback = %q, want 10.0.0.9", got)
	}
}

func TestClientIP_TrustedCIDRs(t *testing.T) {
	if err := ConfigureClientIP("trusted-cidrs:10.0.0.0/8"); err != nil {
		t.Fatal(err)
	}
	// Peer and the last hop are trusted proxies; the first untrusted hop from
	// the right is the real client.
	got := ClientIP(reqWith("10.0.0.9:5000", "8.8.8.8, 10.1.1.1"))
	if got != "8.8.8.8" {
		t.Fatalf("trusted-cidrs = %q, want 8.8.8.8", got)
	}
	// An attacker prepending a spoofed public IP behind the real client can't
	// win: we return the first untrusted from the right, i.e. the real client.
	got = ClientIP(reqWith("10.0.0.9:5000", "6.6.6.6, 7.7.7.7, 10.1.1.1"))
	if got != "7.7.7.7" {
		t.Fatalf("trusted-cidrs spoof = %q, want 7.7.7.7", got)
	}
}

func TestConfigureClientIP_Invalid(t *testing.T) {
	for _, spec := range []string{"bogus", "xff-depth:-1", "trusted-cidrs:notacidr", "trusted-cidrs:"} {
		if err := ConfigureClientIP(spec); err == nil {
			t.Fatalf("ConfigureClientIP(%q) = nil, want error", spec)
		}
	}
	// Reset to the secure default so other tests are unaffected.
	_ = ConfigureClientIP("none")
}
