package connectors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

// AuthConfig is the non-secret authentication configuration shared by the
// REST and GraphQL connection types. It is stored in the connection's
// plaintext `config` JSON. Every actual credential (passwords, tokens,
// keys, client secrets) instead comes from the connection's encrypted
// secret map at dial time - see the field comments below for which secret
// key each AuthType expects.
type AuthConfig struct {
	// AuthType: none | basic | bearer | apiKey | digest |
	// oauth2ClientCredentials | oauth2RefreshToken | jwt | workloadIdentity | kerberos
	AuthType string `json:"authType,omitempty"`

	// apiKey. Secret key: "apiKey".
	APIKeyHeader   string `json:"apiKeyHeader,omitempty"`   // header name, default X-Api-Key
	APIKeyLocation string `json:"apiKeyLocation,omitempty"` // "header" (default) | "query"
	APIKeyParam    string `json:"apiKeyParam,omitempty"`    // query param name when location=query

	// basic. Secret keys: "username", "password".
	// bearer. Secret key: "bearerToken".
	// digest. Secret keys: "username", "password".

	// oauth2ClientCredentials / oauth2RefreshToken.
	// Secret keys: "oauth2ClientId", "oauth2ClientSecret", and for the
	// refresh-token grant, "oauth2RefreshToken".
	OAuth2TokenURL string   `json:"oauth2TokenUrl,omitempty"`
	OAuth2Scopes   []string `json:"oauth2Scopes,omitempty"`
	// OAuth2EndpointParams are extra params sent to the token endpoint on
	// the client_credentials grant only, e.g. {"audience": ["https://api.example.com"]}.
	OAuth2EndpointParams map[string][]string `json:"oauth2EndpointParams,omitempty"`

	// jwt (self-signed JWT bearer). Secret key: "jwtSigningKey" (HMAC
	// secret for HS256, or a PEM-encoded RSA private key for RS256).
	JWTAlgorithm  string         `json:"jwtAlgorithm,omitempty"` // HS256 (default) | RS256
	JWTClaims     map[string]any `json:"jwtClaims,omitempty"`
	JWTTTLSeconds int            `json:"jwtTtlSeconds,omitempty"`

	// workloadIdentity (RFC 8693 token exchange). Secret key
	// "workloadIdentitySubjectToken" is used verbatim if set; otherwise the
	// token is read fresh from WorkloadIdentitySubjectTokenPath on every
	// request (the common case: a platform-projected token file).
	WorkloadIdentityTokenEndpoint    string `json:"workloadIdentityTokenEndpoint,omitempty"`
	WorkloadIdentityAudience         string `json:"workloadIdentityAudience,omitempty"`
	WorkloadIdentityScope            string `json:"workloadIdentityScope,omitempty"`
	WorkloadIdentitySubjectTokenPath string `json:"workloadIdentitySubjectTokenPath,omitempty"`
	WorkloadIdentitySubjectTokenType string `json:"workloadIdentitySubjectTokenType,omitempty"`

	// kerberos (SPNEGO). Secret key: "password" (or leave unset and provide
	// KerberosKeytabPath, which needs no secret).
	KerberosRealm        string `json:"kerberosRealm,omitempty"`
	KerberosUsername     string `json:"kerberosUsername,omitempty"`
	KerberosSPN          string `json:"kerberosSpn,omitempty"`
	KerberosKRB5ConfPath string `json:"kerberosKrb5ConfPath,omitempty"`
	KerberosKeytabPath   string `json:"kerberosKeytabPath,omitempty"`
}

// buildAuthenticator maps a connection's AuthConfig + decrypted secret map
// to a concrete httpclient.Authenticator. ctx is used only to build
// long-lived token sources (OAuth2) that need a context for their internal
// background refresh bookkeeping - it is not held past this call.
//
// dial is the egress-guarded dialer; it is applied to the OAuth2 and
// workload-identity token endpoints, which make their own outbound calls
// separate from the main request client and would otherwise bypass the guard.
func buildAuthenticator(ctx context.Context, cfg AuthConfig, secret map[string]string, dial httpclient.DialFunc) (httpclient.Authenticator, error) {
	switch cfg.AuthType {
	case "", "none":
		return httpclient.NoAuth{}, nil

	case "basic":
		return httpclient.BasicAuth{Username: secret["username"], Password: secret["password"]}, nil

	case "bearer":
		return httpclient.BearerAuth{Token: secret["bearerToken"]}, nil

	case "apiKey":
		location := httpclient.APIKeyInHeader
		name := cfg.APIKeyHeader
		if cfg.APIKeyLocation == "query" {
			location = httpclient.APIKeyInQuery
			name = cfg.APIKeyParam
		}
		return httpclient.APIKeyAuth{Name: name, Key: secret["apiKey"], Location: location}, nil

	case "digest":
		return httpclient.DigestAuth{Username: secret["username"], Password: secret["password"]}, nil

	case "oauth2ClientCredentials":
		return httpclient.NewOAuth2ClientCredentials(oauth2GuardedCtx(ctx, dial), httpclient.OAuth2ClientCredentialsConfig{
			ClientID:       secret["oauth2ClientId"],
			ClientSecret:   secret["oauth2ClientSecret"],
			TokenURL:       cfg.OAuth2TokenURL,
			Scopes:         cfg.OAuth2Scopes,
			EndpointParams: cfg.OAuth2EndpointParams,
		}), nil

	case "oauth2RefreshToken":
		return httpclient.NewOAuth2RefreshToken(oauth2GuardedCtx(ctx, dial), httpclient.OAuth2RefreshTokenConfig{
			ClientID:     secret["oauth2ClientId"],
			ClientSecret: secret["oauth2ClientSecret"],
			TokenURL:     cfg.OAuth2TokenURL,
			RefreshToken: secret["oauth2RefreshToken"],
			Scopes:       cfg.OAuth2Scopes,
		}), nil

	case "jwt":
		method, key, err := jwtSigningMethodAndKey(cfg.JWTAlgorithm, secret["jwtSigningKey"])
		if err != nil {
			return nil, err
		}
		ttl := defaultJWTTTL
		if cfg.JWTTTLSeconds > 0 {
			ttl = time.Duration(cfg.JWTTTLSeconds) * time.Second
		}
		return &httpclient.JWTAuth{SigningMethod: method, Key: key, Claims: cfg.JWTClaims, TTL: ttl}, nil

	case "workloadIdentity":
		var source httpclient.SubjectTokenSource
		if token := secret["workloadIdentitySubjectToken"]; token != "" {
			source = httpclient.StaticSubjectTokenSource(token, cfg.WorkloadIdentitySubjectTokenType)
		} else if cfg.WorkloadIdentitySubjectTokenPath != "" {
			source = httpclient.FileSubjectTokenSource(cfg.WorkloadIdentitySubjectTokenPath, cfg.WorkloadIdentitySubjectTokenType)
		} else {
			return nil, fmt.Errorf("workloadIdentity auth requires either a subject token secret or WorkloadIdentitySubjectTokenPath")
		}
		return httpclient.NewWorkloadIdentityAuth(httpclient.WorkloadIdentityConfig{
			TokenEndpoint: cfg.WorkloadIdentityTokenEndpoint,
			SubjectToken:  source,
			Audience:      cfg.WorkloadIdentityAudience,
			Scope:         cfg.WorkloadIdentityScope,
			ClientID:      secret["workloadIdentityClientId"],
			ClientSecret:  secret["workloadIdentityClientSecret"],
			HTTPClient:    guardedTokenClient(dial, 15*time.Second),
		}), nil

	case "kerberos":
		return httpclient.NewKerberosAuth(httpclient.KerberosConfig{
			KRB5ConfPath: cfg.KerberosKRB5ConfPath,
			Realm:        cfg.KerberosRealm,
			Username:     cfg.KerberosUsername,
			Password:     secret["password"],
			KeytabPath:   cfg.KerberosKeytabPath,
			SPN:          cfg.KerberosSPN,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported authType %q", cfg.AuthType)
	}
}

// oauth2GuardedCtx injects an egress-guarded HTTP client into ctx under the
// oauth2.HTTPClient key so the token endpoint is dialed through the guard.
// With a nil dial it returns ctx unchanged (default client).
func oauth2GuardedCtx(ctx context.Context, dial httpclient.DialFunc) context.Context {
	if dial == nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, httpclient.GuardedHTTPClient(dial, 30*time.Second))
}

// guardedTokenClient builds an egress-guarded client for token endpoints that
// don't take a context (workload identity). Nil dial yields a plain client.
func guardedTokenClient(dial httpclient.DialFunc, timeout time.Duration) *http.Client {
	if dial == nil {
		return nil
	}
	return httpclient.GuardedHTTPClient(dial, timeout)
}

const defaultJWTTTL = 5 * time.Minute

func jwtSigningMethodAndKey(algorithm, signingKey string) (jwt.SigningMethod, any, error) {
	switch algorithm {
	case "", "HS256":
		if signingKey == "" {
			return nil, nil, fmt.Errorf("jwt auth requires a jwtSigningKey secret")
		}
		return jwt.SigningMethodHS256, []byte(signingKey), nil
	case "RS256":
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(signingKey))
		if err != nil {
			return nil, nil, fmt.Errorf("parse RS256 private key: %w", err)
		}
		return jwt.SigningMethodRS256, key, nil
	default:
		return nil, nil, fmt.Errorf("unsupported jwtAlgorithm %q", algorithm)
	}
}
