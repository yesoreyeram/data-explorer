package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	internalhttpx "github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

type Config struct {
	Timeout          time.Duration
	MaxResponseBytes int64
	MaxRedirects     int
	DecompressRatio  int
	Retry            RetryPolicy
	Auth             Authenticator
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
	base := http.DefaultTransport
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

func (c *Client) Do(ctx context.Context, req *http.Request) (*Response, error) {
	req = req.WithContext(ctx)
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
