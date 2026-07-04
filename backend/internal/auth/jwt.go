package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type AccessClaims struct {
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	signingKey []byte
	accessTTL  time.Duration
}

func NewTokenManager(signingKey string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{signingKey: []byte(signingKey), accessTTL: accessTTL}
}

// IssueAccessToken embeds the caller's flattened permission set directly in
// the JWT so every downstream request can authorize with a pure in-memory
// check instead of a database round trip.
func (tm *TokenManager) IssueAccessToken(userID, email string, roles, permissions []string) (string, time.Time, error) {
	expiresAt := time.Now().Add(tm.accessTTL)
	claims := AccessClaims{
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "data-explorer",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tm.signingKey)
	return signed, expiresAt, err
}

func (tm *TokenManager) ParseAccessToken(tokenString string) (rbac.Principal, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return tm.signingKey, nil
	})
	if err != nil || !token.Valid {
		return rbac.Principal{}, ErrInvalidToken
	}

	return rbac.NewPrincipal(claims.Subject, claims.Email, claims.Roles, claims.Permissions), nil
}
