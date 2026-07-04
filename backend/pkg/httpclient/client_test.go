package httpclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryOn503ThenSucceeds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{Retry: RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected eventual 200, got %d", resp.StatusCode)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryPreservesRequestBody(t *testing.T) {
	attempts := 0
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		lastBody = buf.String()
		if attempts < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{Retry: RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"hello":"world"}`)))
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if lastBody != `{"hello":"world"}` {
		t.Fatalf("expected body preserved across retry, got %q", lastBody)
	}
}

func TestMaxResponseBytesTruncates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 1000))
	}))
	defer srv.Close()

	c := New(Config{MaxResponseBytes: 100})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(resp.Body) != 100 || !resp.Truncated {
		t.Fatalf("expected truncated 100-byte body, got len=%d truncated=%v", len(resp.Body), resp.Truncated)
	}
}

func TestMaxRedirectsStopsFollowing(t *testing.T) {
	var srv *httptest.Server
	redirects := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirects++
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	c := New(Config{MaxRedirects: 2})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(context.Background(), req)
	// http.Client surfaces "stopped after N redirects" as an error from Do
	// unless the final response is returned; either signal is acceptable -
	// what matters is it does not loop forever.
	if err == nil && resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect loop to be capped, got status %d err %v", resp.StatusCode, err)
	}
}
