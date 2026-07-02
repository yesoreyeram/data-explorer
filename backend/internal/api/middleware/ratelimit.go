package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

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

func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := httpx.ClientIP(r)
		if !l.get(key).Allow() {
			w.Header().Set("Retry-After", "1")
			httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
