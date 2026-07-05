package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	internalhttpx "github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

type Config struct {
	Timeout          time.Duration
	MaxResponseBytes int64
	// MaxRedirects caps automatic redirect following. 0 disables redirects.
	MaxRedirects int
	// DecompressRatio caps the expansion ratio allowed when decompressing a
	// response body (defense against decompression bombs).
	DecompressRatio int
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
	DefaultMaxResponseBytes = 25 * 1024 * 1024
	DefaultMaxRedirects     = 5
	DefaultDecompressRatio  = 100
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
	if c.DecompressRatio <= 0 {
		c.DecompressRatio = DefaultDecompressRatio
	}
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	cfg.setDefaults()
	// Clone the default transport (never mutate the shared global) and, when a
	// guarded dialer is supplied, route every connection through it.
	base := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.DialContext != nil {
		base.DialContext = cfg.DialContext
	}
	transport := http.RoundTripper(base)
	if rtAuth, ok := cfg.Auth.(RoundTripperAuthenticator); ok {
		transport = rtAuth.WrapRoundTripper(transport)
	}
	transport = &retryTransport{next: transport, policy: cfg.Retry}
	httpClient := &http.Client{Transport: transport, Timeout: cfg.Timeout}
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
	bodyReader := io.ReadCloser(resp.Body)
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		limited, err := internalhttpx.NewLimitedDecompressReader(resp.Body, resp.ContentLength, c.cfg.DecompressRatio)
		if err != nil {
			return nil, fmt.Errorf("httpclient: decode gzip response: %w", err)
		}
		bodyReader = limited
	}
	defer bodyReader.Close()
	limited := io.LimitReader(bodyReader, c.cfg.MaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("httpclient: read response body: %w", err)
	}
	truncated := int64(len(body)) > c.cfg.MaxResponseBytes
	if truncated {
		body = body[:c.cfg.MaxResponseBytes]
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body, Truncated: truncated, Request: req}, nil
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Truncated  bool
	Request    *http.Request
}

func (r *Response) IsError() bool { return r.StatusCode >= 400 }
