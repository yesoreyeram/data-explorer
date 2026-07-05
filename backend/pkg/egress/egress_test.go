package egress

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func mustGuard(t *testing.T, cfg Config) *Guard {
	t.Helper()
	g, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

func TestValidateAddr(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		addr    string
		wantErr bool
	}{
		// SSRF prizes - denied in every mode.
		{"aws/azure metadata", ModeAllowPrivate, "169.254.169.254", true},
		{"ecs creds", ModeAllowPrivate, "169.254.170.2", true},
		{"loopback v4", ModeAllowPrivate, "127.0.0.1", true},
		{"loopback v6", ModeAllowPrivate, "::1", true},
		{"link-local v6", ModeAllowPrivate, "fe80::1", true},
		{"unspecified v4", ModeAllowPrivate, "0.0.0.0", true},
		{"unspecified v6", ModeAllowPrivate, "::", true},
		{"multicast v4", ModeAllowPrivate, "224.0.0.1", true},
		{"multicast v6", ModeAllowPrivate, "ff02::1", true},
		{"broadcast", ModeAllowPrivate, "255.255.255.255", true},
		{"nat64", ModeAllowPrivate, "64:ff9b::a00:1", true},
		// IPv4-mapped IPv6 forms of denied addresses must not bypass.
		{"mapped metadata", ModeAllowPrivate, "::ffff:169.254.169.254", true},
		{"mapped loopback", ModeAllowPrivate, "::ffff:127.0.0.1", true},

		// Private ranges: allowed under allow-private, denied under public-only.
		{"private 10 allowed", ModeAllowPrivate, "10.1.2.3", false},
		{"private 172 allowed", ModeAllowPrivate, "172.16.5.5", false},
		{"private 192.168 allowed", ModeAllowPrivate, "192.168.1.1", false},
		{"cgnat allowed under private", ModeAllowPrivate, "100.64.0.1", false},
		{"ula allowed under private", ModeAllowPrivate, "fd12::1", false},
		{"private 10 denied public-only", ModePublicOnly, "10.1.2.3", true},
		{"private 192.168 denied public-only", ModePublicOnly, "192.168.1.1", true},
		{"cgnat denied public-only", ModePublicOnly, "100.64.0.1", true},
		{"ula denied public-only", ModePublicOnly, "fd12::1", true},
		{"mapped private denied public-only", ModePublicOnly, "::ffff:10.0.0.1", true},

		// Public addresses: allowed in both.
		{"public v4 allow-private", ModeAllowPrivate, "8.8.8.8", false},
		{"public v4 public-only", ModePublicOnly, "8.8.8.8", false},
		{"public v6 public-only", ModePublicOnly, "2606:4700:4700::1111", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGuard(t, Config{Mode: tc.mode})
			err := g.ValidateAddr(netip.MustParseAddr(tc.addr))
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateAddr(%s) = nil, want denial", tc.addr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateAddr(%s) = %v, want allow", tc.addr, err)
			}
			if tc.wantErr && err != nil && !errors.Is(err, ErrDenied) {
				t.Fatalf("ValidateAddr(%s) error not ErrDenied: %v", tc.addr, err)
			}
		})
	}
}

func TestAllowLoopback(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModeAllowPrivate, AllowLoopback: true})
	if err := g.ValidateAddr(netip.MustParseAddr("127.0.0.1")); err != nil {
		t.Fatalf("loopback should be allowed with AllowLoopback: %v", err)
	}
	// Metadata is still denied even with loopback enabled.
	if err := g.ValidateAddr(netip.MustParseAddr("169.254.169.254")); err == nil {
		t.Fatal("metadata must stay denied even with AllowLoopback")
	}
}

func TestValidateHostMetadataNames(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModeAllowPrivate})
	for _, h := range []string{
		"metadata.google.internal",
		"METADATA.GOOGLE.INTERNAL.",
		"metadata.goog",
		"instance-data",
		"instance-data.ec2.internal",
		"localhost",
	} {
		if err := g.ValidateHost(h, "80"); err == nil {
			t.Fatalf("ValidateHost(%q) = nil, want denial", h)
		}
	}
	if err := g.ValidateHost("api.example.com", "443"); err != nil {
		t.Fatalf("ValidateHost(public name) = %v, want allow", err)
	}
}

func TestAllowlistMode(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModeAllowlist, Allowlist: []string{"api.example.com", "*.corp.example.com", "db.example.com:5432"}})
	cases := []struct {
		host, port string
		ok         bool
	}{
		{"api.example.com", "443", true},
		{"other.com", "443", false},
		{"x.corp.example.com", "443", true},
		{"corp.example.com", "443", false}, // wildcard must not match the bare suffix
		{"db.example.com", "5432", true},
		{"db.example.com", "3306", false}, // wrong port
	}
	for _, c := range cases {
		err := g.ValidateHost(c.host, c.port)
		if c.ok && err != nil {
			t.Errorf("ValidateHost(%s:%s) = %v, want allow", c.host, c.port, err)
		}
		if !c.ok && err == nil {
			t.Errorf("ValidateHost(%s:%s) = nil, want denial", c.host, c.port)
		}
	}
}

func TestAllowlistModeRequiresEntries(t *testing.T) {
	if _, err := New(Config{Mode: ModeAllowlist}); err == nil {
		t.Fatal("allowlist mode with empty list should error")
	}
}

// TestDialContextMultiARecord verifies that a single private record in a DNS
// answer poisons the whole dial (the answer-validation loop DialContext runs).
func TestDialContextMultiARecord(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModePublicOnly})
	answer := []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("10.0.0.1")}
	var denied bool
	for _, a := range answer {
		if err := g.ValidateAddr(a.Unmap()); err != nil {
			denied = true
		}
	}
	if !denied {
		t.Fatal("answer containing a private record must be denied")
	}
}

func TestDialContextRejectsNonTCP(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModeAllowPrivate})
	if _, err := g.DialContext(context.Background(), "unix", "/var/run/x.sock"); err == nil {
		t.Fatal("non-tcp network must be denied")
	}
}

func TestDialContextLiteralMetadataDenied(t *testing.T) {
	g := mustGuard(t, Config{Mode: ModeAllowPrivate})
	if _, err := g.DialContext(context.Background(), "tcp", "169.254.169.254:80"); !errors.Is(err, ErrDenied) {
		t.Fatalf("literal metadata dial = %v, want ErrDenied", err)
	}
}
