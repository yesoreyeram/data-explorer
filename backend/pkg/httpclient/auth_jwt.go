package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth mints a self-signed JWT bearer token per the standard "JWT
// Bearer" pattern used by e.g. Google service-account and Snowflake
// key-pair auth: the client signs a short-lived assertion with a private
// key/secret it holds, and the server verifies it against the
// corresponding public key/secret out of band. The signed token is cached
// and only re-minted once it's within RefreshSkew of expiring.
type JWTAuth struct {
	// SigningMethod is e.g. jwt.SigningMethodHS256 or jwt.SigningMethodRS256.
	SigningMethod jwt.SigningMethod
	// Key is the signing key: a []byte secret for HMAC methods, or a
	// *rsa.PrivateKey / crypto.PrivateKey for RSA/ECDSA methods.
	Key any
	// Claims are merged into the token; "iat" and "exp" are always set by
	// this authenticator and will override any values supplied here.
	Claims map[string]any
	// TTL is how long each minted token is valid for. Defaults to 5 minutes.
	TTL time.Duration
	// RefreshSkew mints a new token this long before the cached one expires,
	// to avoid ever sending an on-the-edge-of-expiry token. Defaults to 30s.
	RefreshSkew time.Duration
	// HeaderName defaults to "Authorization"; Scheme defaults to "Bearer".
	HeaderName string
	Scheme     string

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

func (a *JWTAuth) Authenticate(_ context.Context, req *http.Request) error {
	token, err := a.token()
	if err != nil {
		return err
	}
	header := a.HeaderName
	if header == "" {
		header = "Authorization"
	}
	scheme := a.Scheme
	if scheme == "" {
		scheme = "Bearer"
	}
	req.Header.Set(header, scheme+" "+token)
	return nil
}

func (a *JWTAuth) token() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	skew := a.RefreshSkew
	if skew <= 0 {
		skew = 30 * time.Second
	}
	if a.cached != "" && time.Now().Before(a.cachedExp.Add(-skew)) {
		return a.cached, nil
	}

	ttl := a.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now()
	exp := now.Add(ttl)

	claims := jwt.MapClaims{}
	for k, v := range a.Claims {
		claims[k] = v
	}
	claims["iat"] = now.Unix()
	claims["exp"] = exp.Unix()

	tok := jwt.NewWithClaims(a.SigningMethod, claims)
	signed, err := tok.SignedString(a.Key)
	if err != nil {
		return "", fmt.Errorf("httpclient: sign jwt: %w", err)
	}

	a.cached, a.cachedExp = signed, exp
	return signed, nil
}
