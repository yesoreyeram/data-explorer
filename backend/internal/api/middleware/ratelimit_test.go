package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

func TestKeyByUserOrIP(t *testing.T) {
	_ = httpx.ConfigureClientIP("none")

	// Authenticated request keys on the user, not the (spoofable) IP.
	r := &http.Request{Header: http.Header{}, RemoteAddr: "203.0.113.1:9999"}
	r = r.WithContext(rbac.WithPrincipal(context.Background(), rbac.NewPrincipal("u-42", "a@b.c", nil, nil)))
	if got := KeyByUserOrIP(r); got != "user:u-42" {
		t.Fatalf("KeyByUserOrIP(authed) = %q, want user:u-42", got)
	}

	// Anonymous request falls back to the client IP.
	r2 := &http.Request{Header: http.Header{}, RemoteAddr: "203.0.113.2:8888"}
	if got := KeyByUserOrIP(r2); got != "ip:203.0.113.2" {
		t.Fatalf("KeyByUserOrIP(anon) = %q, want ip:203.0.113.2", got)
	}
}

// stubLimiter allows the first n calls per key, then denies.
type stubLimiter struct {
	n    int
	seen map[string]int
}

func (s *stubLimiter) Allow(key string) bool {
	if s.seen == nil {
		s.seen = map[string]int{}
	}
	s.seen[key]++
	return s.seen[key] <= s.n
}

func TestRateLimitMiddleware(t *testing.T) {
	lim := &stubLimiter{n: 1}
	h := RateLimit(lim, func(*http.Request) string { return "k" })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec1 := &statusOnlyRecorder{}
	h.ServeHTTP(rec1, &http.Request{Header: http.Header{}})
	if rec1.status != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.status)
	}
	rec2 := &statusOnlyRecorder{header: http.Header{}}
	h.ServeHTTP(rec2, &http.Request{Header: http.Header{}})
	if rec2.status != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.status)
	}
	if rec2.header.Get("Retry-After") == "" {
		t.Fatal("429 response missing Retry-After header")
	}
}

type statusOnlyRecorder struct {
	status int
	header http.Header
}

func (r *statusOnlyRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}
	return r.header
}
func (r *statusOnlyRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *statusOnlyRecorder) WriteHeader(code int)        { r.status = code }
