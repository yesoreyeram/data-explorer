package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// SubjectTokenSource returns the local credential a WorkloadIdentityAuth
// exchanges for an access token - e.g. a Kubernetes projected service
// account token, a GitHub Actions OIDC token, or any other short-lived
// identity document the workload already holds.
type SubjectTokenSource func(ctx context.Context) (token string, tokenType string, err error)

// FileSubjectTokenSource reads the subject token from a file each time it's
// needed (matching how Kubernetes/most cloud SDKs project workload identity
// tokens: a file that's periodically rewritten with a fresh short-lived
// token, rather than a single static value).
func FileSubjectTokenSource(path, tokenType string) SubjectTokenSource {
	return func(context.Context) (string, string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", fmt.Errorf("read subject token file: %w", err)
		}
		return strings.TrimSpace(string(data)), tokenType, nil
	}
}

// StaticSubjectTokenSource always returns the same token - useful for
// testing or for tokens supplied once at process startup via environment
// variable.
func StaticSubjectTokenSource(token, tokenType string) SubjectTokenSource {
	return func(context.Context) (string, string, error) { return token, tokenType, nil }
}

const (
	TokenTypeJWT           = "urn:ietf:params:oauth:token-type:jwt"
	TokenTypeAccessToken   = "urn:ietf:params:oauth:token-type:access_token"
	TokenTypeIDToken       = "urn:ietf:params:oauth:token-type:id_token"
	grantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
)

// WorkloadIdentityConfig configures an OAuth 2.0 Token Exchange (RFC 8693)
// - the standards-based mechanism underlying AWS IAM Roles for Service
// Accounts, GCP Workload Identity Federation, and Azure Workload Identity:
// a workload proves who it is with a locally-issued token (from its cloud
// or CI platform) and exchanges it, at a security token service, for a
// short-lived access token scoped to the target API - with no long-lived
// secret ever stored by the caller.
type WorkloadIdentityConfig struct {
	// TokenEndpoint is the STS/token-exchange endpoint URL.
	TokenEndpoint string
	// SubjectToken supplies the local credential to exchange.
	SubjectToken SubjectTokenSource
	// Audience and Scope are passed through to the token endpoint as-is;
	// which one a given provider expects varies (RFC 8693 defines both).
	Audience string
	Scope    string
	// RequestedTokenType defaults to an access token.
	RequestedTokenType string
	// ClientID/ClientSecret authenticate the caller to the token endpoint
	// itself, if the endpoint requires it (many federation setups don't -
	// the subject token is itself the proof of identity).
	ClientID     string
	ClientSecret string
}

// WorkloadIdentityAuth exchanges a subject token for an access token per
// RFC 8693, caching the result until shortly before it expires.
type WorkloadIdentityAuth struct {
	cfg    WorkloadIdentityConfig
	client *http.Client

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

func NewWorkloadIdentityAuth(cfg WorkloadIdentityConfig) *WorkloadIdentityAuth {
	if cfg.RequestedTokenType == "" {
		cfg.RequestedTokenType = TokenTypeAccessToken
	}
	return &WorkloadIdentityAuth{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

func (a *WorkloadIdentityAuth) Authenticate(ctx context.Context, req *http.Request) error {
	token, err := a.token(ctx)
	if err != nil {
		return fmt.Errorf("httpclient: workload identity federation: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (a *WorkloadIdentityAuth) token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cached != "" && time.Now().Before(a.cachedExp.Add(-30*time.Second)) {
		return a.cached, nil
	}

	subjectToken, subjectTokenType, err := a.cfg.SubjectToken(ctx)
	if err != nil {
		return "", fmt.Errorf("obtain subject token: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", grantTypeTokenExchange)
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", subjectTokenType)
	form.Set("requested_token_type", a.cfg.RequestedTokenType)
	if a.cfg.Audience != "" {
		form.Set("audience", a.cfg.Audience)
	}
	if a.cfg.Scope != "" {
		form.Set("scope", a.cfg.Scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if a.cfg.ClientID != "" {
		req.SetBasicAuth(a.cfg.ClientID, a.cfg.ClientSecret)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("token endpoint response had no access_token")
	}

	ttl := 5 * time.Minute
	if parsed.ExpiresIn > 0 {
		ttl = time.Duration(parsed.ExpiresIn) * time.Second
	}
	a.cached = parsed.AccessToken
	a.cachedExp = time.Now().Add(ttl)
	return a.cached, nil
}
