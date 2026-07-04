// Package httpx holds small helpers shared by every HTTP handler, so
// response shape (JSON envelopes, error codes) is consistent across the
// entire API surface without each handler reinventing it.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// MaxRequestBodyBytes bounds every JSON request body decoded through
// DecodeJSON, protecting the server from a malicious or buggy client
// exhausting memory on a single request.
const MaxRequestBodyBytes = 1 << 20 // 1MB

var ErrPayloadTooLarge = errors.New("httpx: request body exceeds the maximum allowed size")

type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		// Remediation/Detail are optional - populated when the error carries a
		// concrete next step and/or underlying technical detail (see
		// connections.HealthError) beyond the plain code+message every error
		// response has.
		Remediation string `json:"remediation,omitempty"`
		Detail      string `json:"detail,omitempty"`
	} `json:"error"`
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

// WriteErrorDetailed is WriteError plus an actionable remediation step and/or
// underlying technical detail, for errors that carry more than a bare
// code+message (see connections.HealthError).
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

// WriteDecodeError maps a DecodeJSON error to the appropriate response: 413
// for a request that hit the size guardrail, 400 for anything else
// (malformed JSON, unknown fields, ...).
func WriteDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrPayloadTooLarge) {
		WriteError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds the maximum allowed size")
		return
	}
	WriteError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
}

// DecodeJSON reads and decodes a JSON body with a size cap (MaxRequestBodyBytes)
// to prevent a malicious or buggy client from exhausting server memory on
// one request. A body at or over the cap fails fast with ErrPayloadTooLarge
// rather than a confusing "unexpected EOF" from a silently truncated decode.
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

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
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
