// Package ratelimit provides a shared, Redis-backed rate limiter so a
// horizontally-scaled deployment enforces one budget across all instances,
// instead of each process keeping its own in-memory buckets.
package ratelimit

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// tokenBucket is an atomic token-bucket refill+consume in one round trip. It
// stores the current token count and last-update timestamp per key, refilling
// at `rate` tokens/second up to `burst`, and consuming one token per call.
var tokenBucket = redis.NewScript(`
local rate   = tonumber(ARGV[1])
local burst  = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local data   = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])
if tokens == nil then tokens = burst; ts = now_ms end
local delta = math.max(0, now_ms - ts) / 1000.0
tokens = math.min(burst, tokens + delta * rate)
local allowed = 0
if tokens >= 1 then tokens = tokens - 1; allowed = 1 end
redis.call('HMSET', KEYS[1], 'tokens', tokens, 'ts', now_ms)
redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate * 1000) + 1000)
return allowed
`)

// RedisLimiter implements the middleware Limiter interface against Redis.
type RedisLimiter struct {
	client *redis.Client
	rate   float64
	burst  int
	prefix string
	log    *slog.Logger
}

// New builds a RedisLimiter. rate is sustained requests/second and burst is the
// bucket capacity; prefix namespaces the keys (e.g. "rl:gen:").
func New(client *redis.Client, ratePerSecond float64, burst int, prefix string, log *slog.Logger) *RedisLimiter {
	return &RedisLimiter{client: client, rate: ratePerSecond, burst: burst, prefix: prefix, log: log}
}

// Allow reports whether the request keyed by key may proceed. On a Redis error
// it fails open (allows the request) so a Redis outage degrades abuse control
// rather than availability - the outage is logged.
func (l *RedisLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := tokenBucket.Run(ctx, l.client, []string{l.prefix + key},
		l.rate, l.burst, time.Now().UnixMilli()).Int()
	if err != nil {
		if l.log != nil {
			l.log.Warn("rate limiter degraded: redis error, allowing request", "error", err)
		}
		return true
	}
	return res == 1
}
