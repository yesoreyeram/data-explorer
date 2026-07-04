package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

const refreshCookieName = "de_refresh_token"

type registerRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.DisplayName == "" || len(req.Password) < 12 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "email, displayName, and a password of at least 12 characters are required")
		return
	}

	user, err := h.Auth.Register(r.Context(), req.Email, req.DisplayName, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "an account with this email already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to register user")
		return
	}

	h.recordAudit(r, "user.register", "user", user.ID, audit.OutcomeSuccess, map[string]any{"email": user.Email})
	httpx.WriteJSON(w, http.StatusCreated, user)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
	User        any    `json:"user"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, pair, err := h.Auth.Login(r.Context(), req.Email, req.Password, httpx.ClientIP(r), r.UserAgent())
	if err != nil {
		h.recordAudit(r, "user.login", "user", "", audit.OutcomeFailure, map[string]any{"email": req.Email, "reason": err.Error()})
		if errors.Is(err, auth.ErrAccountSuspended) {
			httpx.WriteError(w, http.StatusForbidden, "account_suspended", "this account has been suspended")
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTokenExpiresAt)
	h.recordAudit(r, "user.login", "user", user.ID, audit.OutcomeSuccess, map[string]any{"email": user.Email})

	httpx.WriteJSON(w, http.StatusOK, loginResponse{
		AccessToken: pair.AccessToken,
		ExpiresAt:   pair.AccessTokenExpiresAt.Format(httpTimeFormat),
		User:        user,
	})
}

func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "no refresh token present")
		return
	}

	user, pair, err := h.Auth.Refresh(r.Context(), cookie.Value, httpx.ClientIP(r), r.UserAgent())
	if err != nil {
		h.clearRefreshCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "session expired, please log in again")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTokenExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, loginResponse{
		AccessToken: pair.AccessToken,
		ExpiresAt:   pair.AccessTokenExpiresAt.Format(httpTimeFormat),
		User:        user,
	})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
		_ = h.Auth.Logout(r.Context(), cookie.Value)
	}
	h.clearRefreshCookie(w)

	p := principalOrEmpty(r)
	h.recordAudit(r, "user.logout", "user", p.UserID, audit.OutcomeSuccess, nil)
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := rbac.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "not logged in")
		return
	}
	user, err := h.AuthRepository.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"roles":       p.Roles,
		"permissions": p.PermissionList(),
	})
}

func (h *Handlers) setRefreshCookie(w http.ResponseWriter, value string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (h *Handlers) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

const httpTimeFormat = "2006-01-02T15:04:05Z07:00"
