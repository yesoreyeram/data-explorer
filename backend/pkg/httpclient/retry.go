package httpclient

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy configures automatic retry of failed requests. The zero value
// disables retries entirely (MaxAttempts of 0 means "try once, don't retry").
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts including the first,
	// e.g. 3 means "try once, then retry up to twice".
	MaxAttempts int
	// BaseDelay and MaxDelay bound an exponential backoff (with full
	// jitter) between attempts: delay = random(0, min(MaxDelay, BaseDelay * 2^attempt)).
	BaseDelay time.Duration
	MaxDelay  time.Duration
	// RetryStatusCodes are response status codes that should be retried
	// (network errors and, by default, 429/502/503/504 are always retried
	// regardless of this list).
	RetryStatusCodes []int
}

// DefaultRetryPolicy is a reasonable starting point for calling
// unpredictable third-party APIs: up to 3 attempts, 200ms-5s backoff.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts: 3,
	BaseDelay:   200 * time.Millisecond,
	MaxDelay:    5 * time.Second,
}

func (p RetryPolicy) shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	for _, code := range p.RetryStatusCodes {
		if code == status {
			return true
		}
	}
	return false
}

type retryTransport struct {
	next   http.RoundTripper
	policy RetryPolicy
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	maxAttempts := t.policy.MaxAttempts
	if maxAttempts <= 1 {
		return t.next.RoundTrip(req)
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		attemptReq := req
		if attempt > 0 {
			attemptReq = req.Clone(req.Context())
			if bodyBytes != nil {
				attemptReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		resp, err = t.next.RoundTrip(attemptReq)

		retriable := err != nil || t.policy.shouldRetryStatus(resp.StatusCode)
		if !retriable || attempt == maxAttempts-1 {
			return resp, err
		}

		delay := retryDelay(t.policy, attempt)
		if resp != nil {
			if ra := retryAfterDelay(resp.Header.Get("Retry-After")); ra > 0 {
				delay = ra
			}
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()
		}

		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
	}

	return resp, err
}

// retryDelay computes exponential backoff with full jitter, per the
// well-established "Exponential Backoff And Jitter" approach (AWS
// Architecture Blog): a random value in [0, min(max, base*2^attempt)]
// avoids every retrying client synchronizing into the same retry burst.
func retryDelay(p RetryPolicy, attempt int) time.Duration {
	base := p.BaseDelay
	if base <= 0 {
		base = DefaultRetryPolicy.BaseDelay
	}
	max := p.MaxDelay
	if max <= 0 {
		max = DefaultRetryPolicy.MaxDelay
	}

	backoff := base << attempt // base * 2^attempt
	if backoff <= 0 || backoff > max {
		backoff = max
	}
	return time.Duration(rand.Int63n(int64(backoff) + 1))
}

func retryAfterDelay(header string) time.Duration {
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(header); err == nil {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		return time.Until(when)
	}
	return 0
}
