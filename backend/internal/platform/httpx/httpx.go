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
	"strconv"
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
