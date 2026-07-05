// Package auth implements registration, login, token refresh, and logout.
//
// Token strategy: short-lived JWT access tokens (default 15m, stateless,
// carries permissions) paired with long-lived opaque refresh tokens (default
// 7d, stored server-side as a SHA-256 hash so a leaked database dump does not
// itself grant sessions, and individually revocable on logout).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/crypto"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountSuspended   = errors.New("account is suspended")
)

type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

var ErrOIDCDisabled = errors.New("single sign-on is not configured")
var ErrEmailNotVerified = errors.New("the identity provider did not verify this email address")

type Service struct {
	repo            *Repository
	tokens          *TokenManager
	refreshTokenTTL time.Duration
	oidc            *OIDCManager
}

func NewService(repo *Repository, tokens *TokenManager, refreshTokenTTL time.Duration) *Service {
	return &Service{repo: repo, tokens: tokens, refreshTokenTTL: refreshTokenTTL}
}

// SetOIDC installs the OIDC manager (SSO providers). Nil leaves SSO off.
func (s *Service) SetOIDC(m *OIDCManager) { s.oidc = m }

// OIDC exposes the manager for the handler layer (auth-URL construction and
// provider listing). Returns nil when SSO is not configured.
func (s *Service) OIDC() *OIDCManager { return s.oidc }

// LoginWithOIDC completes an OIDC callback: it exchanges and verifies the
// authorization code, provisions or links the local user (first login gets the
// least-privilege "viewer" role, matching self-registration), and issues a
// session. It requires a verified email so a provider that doesn't vouch for
// the address can't be used to hijack an existing account by email.
func (s *Service) LoginWithOIDC(ctx context.Context, provider, code, codeVerifier, ip, userAgent string) (domain.User, TokenPair, error) {
	if !s.oidc.Enabled() {
		return domain.User{}, TokenPair{}, ErrOIDCDisabled
	}
	claims, err := s.oidc.Exchange(ctx, provider, code, codeVerifier)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	if claims.Email == "" || !claims.EmailVerified {
		return domain.User{}, TokenPair{}, ErrEmailNotVerified
	}
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	displayName := claims.Name
	if displayName == "" {
		displayName = email
	}

	user, err := s.repo.UpsertFederatedUser(ctx, claims.Issuer, claims.Subject, email, displayName, []string{"viewer"})
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	if user.Status == domain.UserStatusSuspended {
		return domain.User{}, TokenPair{}, ErrAccountSuspended
	}

	pair, err := s.issueTokenPair(ctx, user, ip, userAgent)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

// Register creates a new user and immediately issues a session. New
// self-service signups get the "viewer" role by default (principle of least
// privilege); elevation to editor/admin is an explicit admin action via the
// roles API.
func (s *Service) Register(ctx context.Context, email, displayName, password, ip, userAgent string) (domain.User, TokenPair, error) {
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return domain.User{}, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUser(ctx, email, displayName, hash, []string{"viewer"})
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	pair, err := s.issueTokenPair(ctx, user, ip, userAgent)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

func (s *Service) Login(ctx context.Context, email, password, ip, userAgent string) (domain.User, TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Constant-shape response: don't reveal whether the email exists.
			_, _ = crypto.HashPassword(password) // keep timing comparable to the found-user path
			return domain.User{}, TokenPair{}, ErrInvalidCredentials
		}
		return domain.User{}, TokenPair{}, err
	}

	if user.Status == domain.UserStatusSuspended {
		return domain.User{}, TokenPair{}, ErrAccountSuspended
	}

	ok, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return domain.User{}, TokenPair{}, ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(ctx, user, ip, userAgent)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (domain.User, TokenPair, error) {
	hash := hashToken(refreshToken)
	rec, err := s.repo.GetRefreshToken(ctx, hash)
	if err != nil {
		return domain.User{}, TokenPair{}, ErrInvalidCredentials
	}
	if rec.RevokedAt != nil {
		_ = s.repo.RevokeUserRefreshTokens(ctx, rec.UserID)
		return domain.User{}, TokenPair{}, ErrInvalidCredentials
	}
	if time.Now().After(rec.ExpiresAt) {
		return domain.User{}, TokenPair{}, ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByID(ctx, rec.UserID)
	if err != nil {
		return domain.User{}, TokenPair{}, ErrInvalidCredentials
	}
	if user.Status == domain.UserStatusSuspended {
		return domain.User{}, TokenPair{}, ErrAccountSuspended
	}

	// Rotate: revoke the presented refresh token and issue a brand new pair.
	// This limits the blast radius of a stolen (but not yet used) token.
	_ = s.repo.RevokeRefreshToken(ctx, hash)

	pair, err := s.issueTokenPair(ctx, user, ip, userAgent)
	if err != nil {
		return domain.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.RevokeRefreshToken(ctx, hashToken(refreshToken))
}

func (s *Service) RevokeUserSessions(ctx context.Context, userID string) error {
	return s.repo.RevokeUserRefreshTokens(ctx, userID)
}

func (s *Service) issueTokenPair(ctx context.Context, user domain.User, ip, userAgent string) (TokenPair, error) {
	roles, permissions, err := s.repo.GetUserRolesAndPermissions(ctx, user.ID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("resolve permissions: %w", err)
	}

	access, accessExp, err := s.tokens.IssueAccessToken(user.ID, user.Email, roles, permissions)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue access token: %w", err)
	}

	refresh, err := generateOpaqueToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshExp := time.Now().Add(s.refreshTokenTTL)
	if err := s.repo.StoreRefreshToken(ctx, user.ID, hashToken(refresh), refreshExp, ip, userAgent); err != nil {
		return TokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExp,
	}, nil
}

func (s *Service) Repository() *Repository { return s.repo }

func generateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
