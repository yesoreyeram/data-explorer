// Package httpx holds small helpers shared by every HTTP handler, so
// response shape (JSON envelopes, error codes) is consistent across the
// entire API surface without each handler reinventing it.
package httpx

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const MaxRequestBodyBytes = 1 << 20 // 1MB

var ErrPayloadTooLarge = errors.New("httpx: request body exceeds the maximum allowed size")

type ErrorBody struct {
	Error struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		Remediation string `json:"remediation,omitempty"`
		Detail      string `json:"detail,omitempty"`
	} `json:"error"`
}

type countingReadCloser struct {
	io.ReadCloser
	n int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.n += int64(n)
	return n, err
}

type LimitedDecompressReader struct {
	source       *countingReadCloser
	gzip         *gzip.Reader
	originalSize int64
	ratio        int64
	produced     int64
}

func NewLimitedDecompressReader(body io.ReadCloser, originalSize int64, ratio int) (*LimitedDecompressReader, error) {
	if ratio <= 0 {
		ratio = 100
	}
	source := &countingReadCloser{ReadCloser: body}
	zr, err := gzip.NewReader(source)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	return &LimitedDecompressReader{source: source, gzip: zr, originalSize: originalSize, ratio: int64(ratio)}, nil
}

func (r *LimitedDecompressReader) Read(p []byte) (int, error) {
	n, err := r.gzip.Read(p)
	r.produced += int64(n)
	compressed := r.originalSize
	if r.source.n > compressed {
		compressed = r.source.n
	}
	if compressed <= 0 {
		compressed = 1
	}
	if r.produced > compressed*r.ratio {
		return n, fmt.Errorf("gzip payload exceeded the %d:1 decompression ratio limit", r.ratio)
	}
	return n, err
}

func (r *LimitedDecompressReader) Close() error {
	gzErr := r.gzip.Close()
	srcErr := r.source.Close()
	if gzErr != nil {
		return gzErr
	}
	return srcErr
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorDetailed(w, status, code, message, "", "")
}

func WriteErrorDetailed(w http.ResponseWriter, status int, code, message, remediation, detail string) {
	var body ErrorBody
	body.Error.Code = code
	body.Error.Message = message
	body.Error.Remediation = remediation
	body.Error.Detail = detail
	WriteJSON(w, status, body)
}

type RateLimitBody struct {
	Code         string `json:"code"`
	Quota        int    `json:"quota"`
	Used         int    `json:"used"`
	WindowMS     int64  `json:"window_ms"`
	RetryAfterMS int64  `json:"retry_after_ms"`
	Remediation  string `json:"remediation"`
}

func WriteRateLimit(w http.ResponseWriter, quota, used int, window, retryAfter time.Duration, remediation string) {
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	w.Header().Set("Retry-After", strconvSeconds(retryAfter))
	WriteJSON(w, http.StatusTooManyRequests, RateLimitBody{
		Code:         "rate_limited",
		Quota:        quota,
		Used:         used,
		WindowMS:     window.Milliseconds(),
		RetryAfterMS: retryAfter.Milliseconds(),
		Remediation:  remediation,
	})
}

func WriteDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrPayloadTooLarge) {
		WriteError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds the maximum allowed size")
		return
	}
	WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
}

func DecodeJSON(r *http.Request, v any) error {
	limited := io.LimitReader(r.Body, MaxRequestBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(body) > MaxRequestBodyBytes {
		return ErrPayloadTooLarge
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// clientIPResolver derives the trustworthy client IP for rate limiting and
// audit. The raw X-Forwarded-For header is attacker-controlled, so it is only
// consulted under an explicitly configured trusted-proxy strategy.
type clientIPResolver struct {
	mode  string         // "none" | "xff-depth" | "trusted-cidrs"
	depth int            // number of trusted proxies for xff-depth
	cidrs []netip.Prefix // trusted proxy ranges for trusted-cidrs
}

// defaultClientIPResolver is secure by default: it ignores X-Forwarded-For and
// uses the socket peer. ConfigureClientIP relaxes this for deployments behind
// a trusted proxy.
var defaultClientIPResolver = &clientIPResolver{mode: "none"}

// ConfigureClientIP sets the process-wide trusted-proxy strategy from a spec:
//   - "none"                 (default) socket peer only; X-Forwarded-For ignored
//   - "xff-depth:N"          the (N+1)th X-Forwarded-For entry from the right,
//     for a fixed chain of N trusted proxies
//   - "trusted-cidrs:a,b,..." walk X-Forwarded-For right-to-left, skipping
//     addresses in the trusted ranges, and take the first untrusted one
func ConfigureClientIP(spec string) error {
	spec = strings.TrimSpace(spec)
	switch {
	case spec == "" || spec == "none":
		defaultClientIPResolver = &clientIPResolver{mode: "none"}
	case strings.HasPrefix(spec, "xff-depth:"):
		n, err := strconv.Atoi(strings.TrimPrefix(spec, "xff-depth:"))
		if err != nil || n < 0 {
			return fmt.Errorf("httpx: invalid xff-depth in %q", spec)
		}
		defaultClientIPResolver = &clientIPResolver{mode: "xff-depth", depth: n}
	case strings.HasPrefix(spec, "trusted-cidrs:"):
		var prefixes []netip.Prefix
		for _, c := range strings.Split(strings.TrimPrefix(spec, "trusted-cidrs:"), ",") {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			p, err := netip.ParsePrefix(c)
			if err != nil {
				return fmt.Errorf("httpx: invalid CIDR %q: %w", c, err)
			}
			prefixes = append(prefixes, p)
		}
		if len(prefixes) == 0 {
			return fmt.Errorf("httpx: trusted-cidrs requires at least one CIDR")
		}
		defaultClientIPResolver = &clientIPResolver{mode: "trusted-cidrs", cidrs: prefixes}
	default:
		return fmt.Errorf("httpx: unknown TRUSTED_PROXY_MODE %q", spec)
	}
	return nil
}

// ClientIP returns the trustworthy client IP for r under the configured
// strategy. It always returns a bare IP (no port), so it is a stable
// rate-limit and audit key.
func ClientIP(r *http.Request) string {
	return defaultClientIPResolver.resolve(r)
}

func (c *clientIPResolver) resolve(r *http.Request) string {
	peer := hostOnly(r.RemoteAddr)
	switch c.mode {
	case "xff-depth":
		list := xffList(r)
		if idx := len(list) - 1 - c.depth; idx >= 0 && idx < len(list) {
			return list[idx]
		}
		return peer
	case "trusted-cidrs":
		// Walk the chain [client ... proxyN, peer] from the nearest hop
		// outward, skipping trusted proxies; the first untrusted hop is the
		// real client.
		chain := append(xffList(r), peer)
		for i := len(chain) - 1; i >= 0; i-- {
			if !c.isTrusted(chain[i]) {
				return chain[i]
			}
		}
		return peer
	default: // "none"
		return peer
	}
}

func (c *clientIPResolver) isTrusted(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, p := range c.cidrs {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// xffList returns the X-Forwarded-For entries as bare IPs, left to right.
func xffList(r *http.Request) []string {
	raw := r.Header.Get("X-Forwarded-For")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if ip := hostOnly(strings.TrimSpace(p)); ip != "" {
			out = append(out, ip)
		}
	}
	return out
}

// hostOnly strips a port if present and validates the result is an IP.
func hostOnly(s string) string {
	if s == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	if _, err := netip.ParseAddr(s); err != nil {
		return ""
	}
	return s
}

func strconvSeconds(d time.Duration) string {
	seconds := int64(d.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

func Drain(body io.ReadCloser) {
	if body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
		_ = body.Close()
	}
}
