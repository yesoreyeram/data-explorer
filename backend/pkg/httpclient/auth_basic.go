package httpclient

import (
	"context"
	"net/http"
)

// BasicAuth implements RFC 7617 HTTP Basic authentication.
type BasicAuth struct {
	Username string
	Password string
}

func (a BasicAuth) Authenticate(_ context.Context, req *http.Request) error {
	req.SetBasicAuth(a.Username, a.Password)
	return nil
}

// BearerAuth attaches a static bearer token, e.g. a long-lived personal
// access token. For a token that must be fetched/refreshed, see OAuth2Auth
// or JWTAuth instead.
type BearerAuth struct {
	Token string
}

func (a BearerAuth) Authenticate(_ context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.Token)
	return nil
}

// APIKeyLocation controls where APIKeyAuth places the key.
type APIKeyLocation string

const (
	APIKeyInHeader APIKeyLocation = "header"
	APIKeyInQuery  APIKeyLocation = "query"
)

// APIKeyAuth attaches a static API key as a header or query parameter,
// e.g. `X-Api-Key: <key>` or `?api_key=<key>`.
type APIKeyAuth struct {
	Name     string // header or query parameter name, e.g. "X-Api-Key"
	Key      string
	Location APIKeyLocation // defaults to APIKeyInHeader
}

func (a APIKeyAuth) Authenticate(_ context.Context, req *http.Request) error {
	name := a.Name
	if name == "" {
		name = "X-Api-Key"
	}
	if a.Location == APIKeyInQuery {
		q := req.URL.Query()
		q.Set(name, a.Key)
		req.URL.RawQuery = q.Encode()
		return nil
	}
	req.Header.Set(name, a.Key)
	return nil
}
