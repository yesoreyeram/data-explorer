// Package egress is a standalone, dependency-free outbound-connection guard.
//
// It exists to stop server-side request forgery (SSRF): the application dials
// third-party databases and APIs on a user's behalf, and without a guard a
// caller could point it at a cloud metadata endpoint (169.254.169.254),
// loopback, or an internal-only service. The guard resolves DNS itself,
// validates every resulting IP against a policy, and then dials only the
// validated literal IP - so a name that passes the check cannot be re-resolved
// to a different address between check and connect (a DNS-rebinding / TOCTOU
// defense).
//
// It has no dependency on this module's internal/* packages (stdlib only) and
// can be imported and used standalone.
package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Mode selects how strict the guard is about private/internal targets.
type Mode string

const (
	// ModeAllowPrivate permits RFC1918/private targets (so an internal
	// database keeps working) but always denies the targets no connector ever
	// legitimately needs: cloud metadata endpoints, loopback, link-local, and
	// similar. This is the behavior-preserving hardening default.
	ModeAllowPrivate Mode = "allow-private"
	// ModeAllowlist permits only hosts matching a configured allowlist. The
	// always-denied targets above still apply.
	ModeAllowlist Mode = "allowlist"
	// ModePublicOnly denies all private ranges in addition to the always-denied
	// targets - for deployments that only ever reach public APIs.
	ModePublicOnly Mode = "public-only"
)

// ErrDenied is returned (wrapped) when a destination fails policy.
var ErrDenied = errors.New("egress: destination denied by policy")

// Config configures a Guard. The zero value is not usable; call New.
type Config struct {
	Mode      Mode
	Allowlist []string // host or host:port patterns; "*.example.com" wildcards allowed
	// AllowLoopback permits 127.0.0.0/8 and ::1 (and the name "localhost").
	// Off in production; on in tests that dial httptest servers.
	AllowLoopback bool
	// Resolver is used for DNS. Nil uses net.DefaultResolver. Injectable so
	// policy can be tested against controlled DNS answers.
	Resolver *net.Resolver
	// DialTimeout bounds a single dial. Default 10s.
	DialTimeout time.Duration
}

// Guard enforces an egress Config. Safe for concurrent use.
type Guard struct {
	mode          Mode
	allowlist     []hostPattern
	allowLoopback bool
	resolver      *net.Resolver
	dialer        *net.Dialer
}

// alwaysDeny holds prefixes rejected in every mode - the SSRF "prizes" plus
// address families that are never a legitimate connector target. Loopback is
// listed here but skipped when AllowLoopback is set.
var alwaysDeny = mustPrefixes(
	"127.0.0.0/8",        // loopback (skipped when AllowLoopback)
	"::1/128",            // loopback (skipped when AllowLoopback)
	"169.254.0.0/16",     // link-local, incl. 169.254.169.254 (cloud metadata) and 169.254.170.2 (ECS creds)
	"fe80::/10",          // link-local v6
	"0.0.0.0/8",          // "this network" / unspecified
	"::/128",             // unspecified v6
	"64:ff9b::/96",       // NAT64 - can embed a private v4; never a legit literal target
	"224.0.0.0/4",        // multicast v4
	"ff00::/8",           // multicast v6
	"255.255.255.255/32", // broadcast
)

// privateDeny holds prefixes rejected only in ModePublicOnly.
var privateDeny = mustPrefixes(
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10", // CGNAT
	"198.18.0.0/15", // benchmarking
	"fc00::/7",      // unique local (ULA)
	"fec0::/10",     // deprecated site-local
)

// metadataHosts are denied by name before DNS ever runs, since a resolver
// under attacker influence could otherwise answer them with a public IP.
var metadataHosts = map[string]struct{}{
	"metadata.google.internal":   {},
	"metadata.goog":              {},
	"instance-data":              {},
	"instance-data.ec2.internal": {},
}

// New validates cfg and builds a Guard.
func New(cfg Config) (*Guard, error) {
	switch cfg.Mode {
	case ModeAllowPrivate, ModeAllowlist, ModePublicOnly:
	case "":
		cfg.Mode = ModeAllowPrivate
	default:
		return nil, fmt.Errorf("egress: unknown mode %q", cfg.Mode)
	}

	patterns := make([]hostPattern, 0, len(cfg.Allowlist))
	for _, raw := range cfg.Allowlist {
		p, err := parseHostPattern(raw)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	if cfg.Mode == ModeAllowlist && len(patterns) == 0 {
		return nil, errors.New("egress: allowlist mode requires a non-empty allowlist")
	}

	timeout := cfg.DialTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	resolver := cfg.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	return &Guard{
		mode:          cfg.Mode,
		allowlist:     patterns,
		allowLoopback: cfg.AllowLoopback,
		resolver:      resolver,
		dialer:        &net.Dialer{Timeout: timeout},
	}, nil
}

// DialContext is a drop-in dialer (net.Dialer.DialContext signature) suitable
// for http.Transport.DialContext, pgconn.Config.DialFunc, and
// mysql.RegisterDialContext. It enforces policy, then dials only validated,
// pinned IPs.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if !strings.HasPrefix(network, "tcp") {
		return nil, fmt.Errorf("%w: network %q not permitted", ErrDenied, network)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("egress: bad address %q: %w", addr, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("egress: bad port %q: %w", portStr, err)
	}

	// Name-level checks (metadata hostnames, allowlist membership) apply to
	// both literal IPs and hostnames.
	if err := g.ValidateHost(host, portStr); err != nil {
		return nil, err
	}

	// Literal IP: validate and dial it directly.
	if ip, perr := netip.ParseAddr(host); perr == nil {
		ip = ip.Unmap()
		if err := g.ValidateAddr(ip); err != nil {
			return nil, err
		}
		return g.dial(ctx, network, netip.AddrPortFrom(ip, uint16(port)))
	}

	// Hostname: resolve, validate the entire answer, then dial pinned IPs.
	addrs, err := g.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("egress: resolve %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("egress: %q resolved to no addresses", host)
	}
	validated := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		a = a.Unmap()
		if err := g.ValidateAddr(a); err != nil {
			// One bad record poisons the whole answer (multi-A-record defense).
			return nil, fmt.Errorf("%w: %q resolved to disallowed %s", ErrDenied, host, a)
		}
		validated = append(validated, a)
	}

	var lastErr error
	for _, a := range validated {
		conn, err := g.dial(ctx, network, netip.AddrPortFrom(a, uint16(port)))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (g *Guard) dial(ctx context.Context, network string, ap netip.AddrPort) (net.Conn, error) {
	// Dial the literal validated IP - never the hostname - so there is no
	// second resolution the guard didn't vet.
	return g.dialer.DialContext(ctx, network, ap.String())
}

// ValidateHost applies name-level policy: the metadata-hostname denylist,
// "localhost", and (in allowlist mode) allowlist membership. Passing a literal
// IP string is fine - the metadata checks simply won't match.
func (g *Guard) ValidateHost(host, port string) error {
	name := strings.ToLower(strings.TrimSuffix(host, "."))
	if _, bad := metadataHosts[name]; bad {
		return fmt.Errorf("%w: metadata hostname %q", ErrDenied, host)
	}
	if name == "localhost" && !g.allowLoopback {
		return fmt.Errorf("%w: localhost", ErrDenied)
	}
	if g.mode == ModeAllowlist && !g.hostAllowed(host, port) {
		return fmt.Errorf("%w: %q not in allowlist", ErrDenied, host)
	}
	return nil
}

// ValidateAddr applies IP-level policy for the guard's mode.
func (g *Guard) ValidateAddr(a netip.Addr) error {
	a = a.Unmap()
	if !a.IsValid() {
		return fmt.Errorf("%w: invalid address", ErrDenied)
	}
	loopback := a.IsLoopback()
	if loopback && g.allowLoopback {
		return nil
	}
	if a.IsUnspecified() || a.IsMulticast() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || loopback {
		return fmt.Errorf("%w: %s is loopback/link-local/multicast/unspecified", ErrDenied, a)
	}
	for _, p := range alwaysDeny {
		if p.Contains(a) {
			return fmt.Errorf("%w: %s", ErrDenied, a)
		}
	}
	if g.mode == ModePublicOnly {
		for _, p := range privateDeny {
			if p.Contains(a) {
				return fmt.Errorf("%w: %s is a private address", ErrDenied, a)
			}
		}
	}
	return nil
}

func (g *Guard) hostAllowed(host, port string) bool {
	for _, p := range g.allowlist {
		if p.matches(host, port) {
			return true
		}
	}
	return false
}

// --- allowlist patterns ---

type hostPattern struct {
	host     string // lowercased; may start with "*." for a suffix wildcard
	port     string // "" matches any port
	wildcard bool
}

func parseHostPattern(raw string) (hostPattern, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return hostPattern{}, errors.New("egress: empty allowlist entry")
	}
	host, port := raw, ""
	if h, p, err := net.SplitHostPort(raw); err == nil {
		host, port = h, p
	}
	p := hostPattern{host: host, port: port}
	if strings.HasPrefix(host, "*.") {
		p.wildcard = true
		p.host = host[1:] // ".example.com"
	}
	return p, nil
}

func (p hostPattern) matches(host, port string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if p.port != "" && p.port != port {
		return false
	}
	if p.wildcard {
		return strings.HasSuffix(host, p.host) && host != p.host[1:]
	}
	return host == p.host
}

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}
