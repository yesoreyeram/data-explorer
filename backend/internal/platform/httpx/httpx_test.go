package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	huge := strings.Repeat("a", MaxRequestBodyBytes+1)
	body := `{"field":"` + huge + `"}`

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(body)))
	var v map[string]string
	err := DecodeJSON(req, &v)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestDecodeJSONAcceptsNormalBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{"field":"value"}`)))
	var v struct {
		Field string `json:"field"`
	}
	if err := DecodeJSON(req, &v); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if v.Field != "value" {
		t.Fatalf("expected field=value, got %q", v.Field)
	}
}

func TestWriteRateLimitBodyAndRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteRateLimit(rec, 60, 61, time.Minute, 1500*time.Millisecond, "Retry later.")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
	var body RateLimitBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "rate_limited" || body.Quota != 60 || body.Used != 61 || body.WindowMS != 60_000 || body.RetryAfterMS != 1500 {
		t.Fatalf("unexpected rate-limit body: %+v", body)
	}
}

func TestClientIPIgnoresForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.77")

	if got := ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("expected RemoteAddr host, got %q", got)
	}
}
