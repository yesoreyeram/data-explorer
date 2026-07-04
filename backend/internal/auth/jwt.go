package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// FolderGrantClaim mirrors rbac.FolderGrant with short JSON keys (f/p) since
// this claim repeats once per folder-scoped binding a user holds, unlike
// the flat Permissions array above.
type FolderGrantClaim struct {
	FolderID    string   `json:"f"`
	Permissions []string `json:"p"`
}

type AccessClaims struct {
	Email        string             `json:"email"`
	Roles        []string           `json:"roles"`
	Permissions  []string           `json:"permissions"`
	FolderGrants []FolderGrantClaim `json:"folderGrants,omitempty"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	signingKey []byte
	accessTTL  time.Duration
}

func NewTokenManager(signingKey string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{signingKey: []byte(signingKey), accessTTL: accessTTL}
}

// IssueAccessToken embeds the caller's flattened permission set - both
// account-wide and folder-scoped - directly in the JWT so every downstream
// request can authorize with a pure in-memory check instead of a database
// round trip.
func (tm *TokenManager) IssueAccessToken(userID, email string, roles, permissions []string, folderGrants []rbac.FolderGrant) (string, time.Time, error) {
	expiresAt := time.Now().Add(tm.accessTTL)
	claims := AccessClaims{
		Email:        email,
		Roles:        roles,
		Permissions:  permissions,
		FolderGrants: toFolderGrantClaims(folderGrants),
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

func toFolderGrantClaims(grants []rbac.FolderGrant) []FolderGrantClaim {
	if len(grants) == 0 {
		return nil
	}
	out := make([]FolderGrantClaim, len(grants))
	for i, g := range grants {
		out[i] = FolderGrantClaim{FolderID: g.FolderID, Permissions: g.Permissions}
	}
	return out
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

	folderGrants := make([]rbac.FolderGrant, len(claims.FolderGrants))
	for i, g := range claims.FolderGrants {
		folderGrants[i] = rbac.FolderGrant{FolderID: g.FolderID, Permissions: g.Permissions}
	}
	return rbac.NewPrincipal(claims.Subject, claims.Email, claims.Roles, claims.Permissions, folderGrants), nil
}
