package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/egress"
)

func guardedRESTOptions(t *testing.T) Options {
	t.Helper()
	g, err := egress.New(egress.Config{Mode: egress.ModeAllowPrivate})
	if err != nil {
		t.Fatalf("egress.New: %v", err)
	}
	return Options{DialContext: g.DialContext}
}

// A public base URL must not let an absolute spec.Path smuggle a request to a
// metadata address: the guard catches it at dial time regardless of the base.
func TestREST_AbsolutePathToMetadataDenied(t *testing.T) {
	r := NewREST(guardedRESTOptions(t))
	cfg := json.RawMessage(`{"baseUrl":"http://example.com"}`)
	_, err := r.Execute(context.Background(), cfg, nil, connections.QuerySpec{
		Path: "http://169.254.169.254/latest/meta-data/",
	})
	if err == nil {
		t.Fatal("expected metadata target to be denied")
	}
	if !errors.Is(err, egress.ErrDenied) {
		t.Fatalf("error is not ErrDenied: %v", err)
	}
}

// The OAuth2 token endpoint is dialed by the auth layer, separately from the
// request client; it must be subject to the same guard.
func TestREST_OAuth2TokenEndpointGuarded(t *testing.T) {
	r := NewREST(guardedRESTOptions(t))
	cfg := json.RawMessage(`{
		"baseUrl":"http://example.com",
		"authType":"oauth2ClientCredentials",
		"oauth2TokenUrl":"http://169.254.169.254/token"
	}`)
	secret := map[string]string{"oauth2ClientId": "id", "oauth2ClientSecret": "sec"}
	_, err := r.Execute(context.Background(), cfg, secret, connections.QuerySpec{Path: "/data"})
	if err == nil {
		t.Fatal("expected token-endpoint dial to be denied")
	}
	if !errors.Is(err, egress.ErrDenied) {
		t.Fatalf("error is not ErrDenied: %v", err)
	}
}

// StrictHeaders must reject reserved/hop-by-hop request headers.
func TestREST_StrictHeadersRejectsReserved(t *testing.T) {
	opts := guardedRESTOptions(t)
	opts.StrictHeaders = true
	r := NewREST(opts)
	cfg := json.RawMessage(`{"baseUrl":"http://example.com"}`)
	_, err := r.Execute(context.Background(), cfg, nil, connections.QuerySpec{
		Path:    "/x",
		Headers: map[string]string{"X-Forwarded-For": "1.2.3.4"},
	})
	if err == nil {
		t.Fatal("expected reserved header to be rejected")
	}
}

// Postgres DSN hygiene: a host smuggling a unix socket / multi-host must fail
// before any dial.
func TestPostgres_DSNHygiene(t *testing.T) {
	p := NewPostgres(guardedRESTOptions(t))
	for _, host := range []string{"/var/run/postgresql", "a.example.com,b.internal", ""} {
		cfg, _ := json.Marshal(map[string]any{"host": host, "database": "db", "user": "u"})
		if _, err := p.dsn(cfg, map[string]string{"password": "p"}); err == nil {
			t.Fatalf("host %q should be rejected", host)
		}
	}
}
