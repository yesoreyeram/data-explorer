package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

// Limiter is a keyed rate limiter. It is implemented by the in-process
// IPRateLimiter and by a shared Redis-backed limiter (see
// internal/adapters/ratelimit), so a horizontally-scaled deployment can
// enforce one budget across instances.
type Limiter interface {
	Allow(key string) bool
}

// IPRateLimiter is a simple in-memory, per-client-IP token bucket. It is
// deliberately lightweight (no external dependency like Redis) since a
// single-process bucket is sufficient to blunt credential-stuffing and
// scripted abuse; horizontally-scaled deployments should front the API with
// a shared limiter (e.g. at the load balancer/API gateway) in addition to
// this one.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

func NewIPRateLimiter(requestsPerSecond float64, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(requestsPerSecond),
		burst:    burst,
	}
	go l.cleanupLoop()
	return l
}

func (l *IPRateLimiter) get(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.limiters[key]
	if !ok {
		lim = rate.NewLimiter(l.r, l.burst)
		l.limiters[key] = lim
	}
	return lim
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for k, lim := range l.limiters {
			if lim.Tokens() >= float64(l.burst) {
				delete(l.limiters, k)
			}
		}
		l.mu.Unlock()
	}
}

// Allow implements Limiter.
func (l *IPRateLimiter) Allow(key string) bool { return l.get(key).Allow() }

// RateLimit builds middleware that rejects a request when limiter.Allow returns
// false for keyFn(r), responding 429 with rate-limit headers.
func RateLimit(limiter Limiter, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow(keyFn(r)) {
				httpx.WriteRateLimit(w, 0, 0, time.Second, time.Second, "Too many requests. Retry after the indicated delay or reduce request frequency.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// KeyByIP keys a limiter on the resolved client IP.
func KeyByIP(r *http.Request) string { return "ip:" + httpx.ClientIP(r) }

// KeyByUserOrIP keys on the authenticated user when present - stronger than IP
// because it survives NAT and can't be shed by rotating source addresses - and
// falls back to the client IP for anonymous requests.
func KeyByUserOrIP(r *http.Request) string {
	if p, ok := rbac.FromContext(r.Context()); ok && p.UserID != "" {
		return "user:" + p.UserID
	}
	return "ip:" + httpx.ClientIP(r)
}
