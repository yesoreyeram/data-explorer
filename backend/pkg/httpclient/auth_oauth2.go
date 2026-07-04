package httpclient

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2Auth attaches an OAuth2 access token obtained and automatically
// refreshed via golang.org/x/oauth2 - the de-facto standard, well-audited
// Go OAuth2 implementation. Build one with NewOAuth2ClientCredentials or
// NewOAuth2RefreshToken rather than constructing it directly.
type OAuth2Auth struct {
	source oauth2.TokenSource
}

func (a *OAuth2Auth) Authenticate(_ context.Context, req *http.Request) error {
	token, err := a.source.Token()
	if err != nil {
		return err
	}
	token.SetAuthHeader(req)
	return nil
}

// OAuth2ClientCredentialsConfig configures the OAuth2 "client_credentials"
// grant (RFC 6749 §4.4) - the standard machine-to-machine flow where the
// integration itself (not an end user) is the credential holder.
type OAuth2ClientCredentialsConfig struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scopes       []string
	// EndpointParams are extra parameters sent in the token request, e.g.
	// {"audience": {"https://api.example.com"}} for Auth0-style APIs.
	EndpointParams map[string][]string
}

// NewOAuth2ClientCredentials builds an Authenticator that fetches and
// caches an access token via the client_credentials grant, transparently
// re-fetching it once it's near expiry.
func NewOAuth2ClientCredentials(ctx context.Context, cfg OAuth2ClientCredentialsConfig) *OAuth2Auth {
	ccCfg := &clientcredentials.Config{
		ClientID:       cfg.ClientID,
		ClientSecret:   cfg.ClientSecret,
		TokenURL:       cfg.TokenURL,
		Scopes:         cfg.Scopes,
		EndpointParams: cfg.EndpointParams,
	}
	return &OAuth2Auth{source: ccCfg.TokenSource(ctx)}
}

// OAuth2RefreshTokenConfig configures the OAuth2 "refresh_token" grant
// (RFC 6749 §6): the integration was set up once via an authorization-code
// flow elsewhere and holds a long-lived refresh token, which this
// authenticator exchanges for short-lived access tokens as needed.
type OAuth2RefreshTokenConfig struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	RefreshToken string
	Scopes       []string
}

// NewOAuth2RefreshToken builds an Authenticator around a pre-obtained
// refresh token, auto-refreshing the access token as it expires.
func NewOAuth2RefreshToken(ctx context.Context, cfg OAuth2RefreshTokenConfig) *OAuth2Auth {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
		Endpoint:     oauth2.Endpoint{TokenURL: cfg.TokenURL},
	}
	source := oauthCfg.TokenSource(ctx, &oauth2.Token{RefreshToken: cfg.RefreshToken})
	return &OAuth2Auth{source: source}
}
