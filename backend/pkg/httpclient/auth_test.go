package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{Auth: BasicAuth{Username: "alice", Password: "secret"}})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotUser != "alice" || gotPass != "secret" {
		t.Fatalf("expected alice/secret, got %s/%s", gotUser, gotPass)
	}
}

func TestBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	c := New(Config{Auth: BearerAuth{Token: "tok123"}})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	c.Do(context.Background(), req)

	if gotAuth != "Bearer tok123" {
		t.Fatalf("expected 'Bearer tok123', got %q", gotAuth)
	}
}

func TestAPIKeyAuthHeaderAndQuery(t *testing.T) {
	var gotHeader, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Api-Key")
		gotQuery = r.URL.Query().Get("api_key")
	}))
	defer srv.Close()

	c := New(Config{Auth: APIKeyAuth{Name: "X-Api-Key", Key: "k1", Location: APIKeyInHeader}})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	c.Do(context.Background(), req)
	if gotHeader != "k1" {
		t.Fatalf("expected header key k1, got %q", gotHeader)
	}

	c2 := New(Config{Auth: APIKeyAuth{Name: "api_key", Key: "k2", Location: APIKeyInQuery}})
	req2, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	c2.Do(context.Background(), req2)
	if gotQuery != "k2" {
		t.Fatalf("expected query key k2, got %q", gotQuery)
	}
}

func TestJWTAuthSignsAndCaches(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	auth := &JWTAuth{
		SigningMethod: jwt.SigningMethodHS256,
		Key:           []byte("test-secret"),
		Claims:        map[string]any{"sub": "svc-account"},
		TTL:           time.Minute,
	}
	c := New(Config{Auth: auth})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		c.Do(context.Background(), req)
	}

	if len(seen) != 2 || seen[0] == "" || seen[0] != seen[1] {
		t.Fatalf("expected the same cached bearer token reused across calls, got %v", seen)
	}
}

func TestDigestAuthChallengeResponse(t *testing.T) {
	const username, password, realm, nonce = "alice", "secret123", "test-realm", "abc123nonce"

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="`+realm+`", qop="auth", nonce="`+nonce+`", algorithm=MD5`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		params := parseDigestChallenge(auth)
		if params["username"] != username {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{Auth: DigestAuth{Username: username, Password: password}})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after digest handshake, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts (challenge + authenticated retry), got %d", attempts)
	}
}
