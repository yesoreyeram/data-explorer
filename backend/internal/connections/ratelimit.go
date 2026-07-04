package connections

import (
	"sync"

	"golang.org/x/time/rate"
)

// perConnectionLimiter throttles how often any single connection can be
// dialed out to, independent of the API's own per-IP rate limits
// (internal/api/middleware/ratelimit.go). Those protect this server from
// its callers; this protects a connection's downstream system (a customer's
// database, a partner's API) from being hammered by a runaway workflow that
// re-executes the same source node in a tight loop, or by many users
// sharing one connection at once.
type perConnectionLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

// DefaultConnectionRateLimit and DefaultConnectionRateBurst are generous
// enough not to interfere with normal exploration/workflow use, while still
// capping a misbehaving loop well below what would alarm a typical
// downstream API's own rate limiting.
const (
	DefaultConnectionRateLimit = 5 // requests/sec
	DefaultConnectionRateBurst = 10
)

func newPerConnectionLimiter(requestsPerSecond float64, burst int) *perConnectionLimiter {
	return &perConnectionLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

func (l *perConnectionLimiter) Allow(connectionID string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[connectionID]
	if !ok {
		lim = rate.NewLimiter(l.r, l.burst)
		l.limiters[connectionID] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}
