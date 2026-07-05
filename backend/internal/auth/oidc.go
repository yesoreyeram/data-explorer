package auth

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCProviderConfig configures one federated identity provider (Google,
// GitHub-via-broker, Okta, Entra, Auth0, ...). It is verified statelessly:
// the executor validates each ID token against the provider's JWKS - no
// session store, no local signing key.
type OIDCProviderConfig struct {
	Name         string   `json:"name"`   // stable id used in URLs, e.g. "google"
	Label        string   `json:"label"`  // human label for the sign-in button
	Issuer       string   `json:"issuer"` // OIDC issuer URL (discovery root)
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	RedirectURL  string   `json:"redirectUrl"`
	Scopes       []string `json:"scopes,omitempty"` // defaults to openid,email,profile
}

// oidcProvider is a configured, discovery-resolved provider.
type oidcProvider struct {
	cfg      OIDCProviderConfig
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// OIDCManager holds every configured provider. Nil / empty means SSO is off.
type OIDCManager struct {
	providers map[string]*oidcProvider
}

// OIDCClaims is the subset of the ID token this app consumes.
type OIDCClaims struct {
	Subject       string `json:"sub"`
	Issuer        string `json:"iss"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// NewOIDCManager resolves each provider's discovery document and builds its
// verifier. It performs network calls, so call it at startup. The target is
// the operator-configured issuer (an IdP), not user-supplied input, so it is
// not an SSRF vector and is not routed through the per-request egress guard.
func NewOIDCManager(ctx context.Context, configs []OIDCProviderConfig) (*OIDCManager, error) {
	m := &OIDCManager{providers: map[string]*oidcProvider{}}
	for _, c := range configs {
		if c.Name == "" || c.Issuer == "" || c.ClientID == "" || c.RedirectURL == "" {
			return nil, fmt.Errorf("oidc provider %q: name, issuer, clientId and redirectUrl are required", c.Name)
		}
		provider, err := oidc.NewProvider(ctx, c.Issuer)
		if err != nil {
			return nil, fmt.Errorf("oidc provider %q discovery: %w", c.Name, err)
		}
		scopes := c.Scopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "email", "profile"}
		}
		m.providers[c.Name] = &oidcProvider{
			cfg: c,
			oauth: &oauth2.Config{
				ClientID:     c.ClientID,
				ClientSecret: c.ClientSecret,
				RedirectURL:  c.RedirectURL,
				Endpoint:     provider.Endpoint(),
				Scopes:       scopes,
			},
			verifier: provider.Verifier(&oidc.Config{ClientID: c.ClientID}),
		}
	}
	return m, nil
}

// Enabled reports whether any provider is configured.
func (m *OIDCManager) Enabled() bool { return m != nil && len(m.providers) > 0 }

// Providers lists the configured providers (name + label) for the login UI.
func (m *OIDCManager) Providers() []OIDCProviderInfo {
	if m == nil {
		return nil
	}
	out := make([]OIDCProviderInfo, 0, len(m.providers))
	for _, p := range m.providers {
		label := p.cfg.Label
		if label == "" {
			label = p.cfg.Name
		}
		out = append(out, OIDCProviderInfo{Name: p.cfg.Name, Label: label})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// OIDCProviderInfo is the public description of a provider.
type OIDCProviderInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

var ErrOIDCProviderUnknown = errors.New("unknown identity provider")

// AuthCodeURL builds the provider's authorization-request URL for the
// Authorization Code + PKCE flow.
func (m *OIDCManager) AuthCodeURL(provider, state, codeVerifier string) (string, error) {
	p, ok := m.provider(provider)
	if !ok {
		return "", ErrOIDCProviderUnknown
	}
	return p.oauth.AuthCodeURL(state,
		oauth2.S256ChallengeOption(codeVerifier),
		oauth2.AccessTypeOnline,
	), nil
}

// Exchange completes the flow: swaps the code for tokens and verifies the ID
// token, returning the validated claims.
func (m *OIDCManager) Exchange(ctx context.Context, provider, code, codeVerifier string) (OIDCClaims, error) {
	p, ok := m.provider(provider)
	if !ok {
		return OIDCClaims{}, ErrOIDCProviderUnknown
	}
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("token exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return OIDCClaims{}, errors.New("provider response had no id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawID)
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("verify id token: %w", err)
	}
	var claims OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		return OIDCClaims{}, fmt.Errorf("parse id token claims: %w", err)
	}
	claims.Issuer = idToken.Issuer
	claims.Subject = idToken.Subject
	return claims, nil
}

func (m *OIDCManager) provider(name string) (*oidcProvider, bool) {
	if m == nil {
		return nil, false
	}
	p, ok := m.providers[name]
	return p, ok
}
