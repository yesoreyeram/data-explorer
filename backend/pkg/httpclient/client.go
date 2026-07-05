// Package httpclient is a standalone, dependency-light HTTP client library
// purpose-built for calling third-party APIs on a user's behalf: it
// supports the full spectrum of authentication schemes those APIs actually
// require (Basic, Bearer, API key, self-signed JWT, OAuth2 client
// credentials/refresh token, Digest challenge-response, RFC 8693 workload
// identity token exchange, and Kerberos/SPNEGO), several pagination
// strategies (including GraphQL cursor pagination), and a small set of
// guardrails (timeouts, response size caps, redirect caps, bounded retry
// with backoff) so a single misbehaving upstream can't take the calling
// process down with it.
//
// This package has no dependency on this module's internal/* packages and
// can be imported and used standalone.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Config controls the guardrails and transport behavior of a Client. Every
// field has a safe default applied by New.
type Config struct {
	// Timeout bounds a single HTTP round trip (connect + write + read).
	Timeout time.Duration
	// MaxResponseBytes caps how much of a response body is read; the rest is
	// discarded. Protects against an upstream streaming an unbounded or
	// malicious response into process memory.
	MaxResponseBytes int64
	// MaxRedirects caps automatic redirect following. 0 disables redirects.
	MaxRedirects int
	// Retry configures automatic retry of failed requests. Zero value
	// disables retries (see DefaultRetryPolicy for a sane starting point).
	Retry RetryPolicy
	// Auth is the authentication strategy applied to every request. Nil
	// means no authentication is added.
	Auth Authenticator
	// DialContext, when set, replaces the transport's dialer. It is the seam
	// through which an egress guard (SSRF defense) validates and pins every
	// outbound connection - including redirects, retries, and pagination, all
	// of which reuse this transport. Nil uses the default dialer.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	// UserAgent, when set, is applied to every request that doesn't already
	// carry one - so an outbound relay identifies itself honestly to upstreams.
	UserAgent string
}

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = 25 * 1024 * 1024 // 25MB
	DefaultMaxRedirects     = 5
)

func (c *Config) setDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.MaxResponseBytes <= 0 {
		c.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if c.MaxRedirects == 0 {
		c.MaxRedirects = DefaultMaxRedirects
	}
}

// Client is a guardrailed, authenticated HTTP client. Safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client. Passing a zero-value Config gets sane production
// defaults (30s timeout, 25MB response cap, 5 redirects, no auth, no retry).
func New(cfg Config) *Client {
	cfg.setDefaults()

	// Clone the default transport (never mutate the shared global) and, when a
	// guarded dialer is supplied, route every connection through it.
	base := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.DialContext != nil {
		base.DialContext = cfg.DialContext
	}
	transport := http.RoundTripper(base)

	// Auth schemes that need to react to a response (Digest's
	// challenge-response handshake) wrap the transport directly; simple
	// mutator-style auth (Basic, Bearer, ...) is instead applied per-request
	// in Do, since it never needs to see the response.
	if rtAuth, ok := cfg.Auth.(RoundTripperAuthenticator); ok {
		transport = rtAuth.WrapRoundTripper(transport)
	}

	transport = &retryTransport{next: transport, policy: cfg.Retry}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
	if cfg.MaxRedirects <= 0 {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("httpclient: stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		}
	}

	return &Client{cfg: cfg, http: httpClient}
}

// DialFunc is the dialer signature shared by http.Transport.DialContext,
// pgconn.Config.DialFunc, and mysql.RegisterDialContext.
type DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

// GuardedHTTPClient builds an *http.Client whose transport dials through
// dialContext. It's used to bring token-endpoint calls (OAuth2, workload
// identity) - which don't go through the main Client - under the same egress
// policy. A nil dialContext yields a plain client with the given timeout.
func GuardedHTTPClient(dialContext DialFunc, timeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if dialContext != nil {
		t.DialContext = dialContext
	}
	return &http.Client{Transport: t, Timeout: timeout}
}

// Do sends req, applying authentication, then reads the response body up to
// MaxResponseBytes and returns it as a *Response. The caller does not need
// to close anything - the underlying body is always fully drained and
// closed by Do.
func (c *Client) Do(ctx context.Context, req *http.Request) (*Response, error) {
	req = req.WithContext(ctx)

	if c.cfg.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}

	if c.cfg.Auth != nil {
		if err := c.cfg.Auth.Authenticate(ctx, req); err != nil {
			return nil, fmt.Errorf("httpclient: authenticate: %w", err)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpclient: request failed: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, c.cfg.MaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("httpclient: read response body: %w", err)
	}

	truncated := int64(len(body)) > c.cfg.MaxResponseBytes
	if truncated {
		body = body[:c.cfg.MaxResponseBytes]
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
		Truncated:  truncated,
		Request:    req,
	}, nil
}

// Response is the fully-buffered result of a Client.Do call.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	// Truncated is true if the response exceeded MaxResponseBytes and was cut short.
	Truncated bool
	Request   *http.Request
}

func (r *Response) IsError() bool { return r.StatusCode >= 400 }
