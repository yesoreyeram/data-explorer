package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

const (
	oidcStateCookie    = "de_oidc_state"
	oidcVerifierCookie = "de_oidc_verifier"
)

// ListAuthProviders is a public endpoint the login page uses to render the
// available "Sign in with ..." buttons.
func (h *Handlers) ListAuthProviders(w http.ResponseWriter, r *http.Request) {
	var providers []auth.OIDCProviderInfo
	if m := h.Auth.OIDC(); m != nil {
		providers = m.Providers()
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// StartOIDC begins the Authorization Code + PKCE flow: it mints a state and a
// PKCE verifier, stashes them in short-lived cookies, and redirects to the
// provider.
func (h *Handlers) StartOIDC(w http.ResponseWriter, r *http.Request) {
	m := h.Auth.OIDC()
	if m == nil || !m.Enabled() {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "single sign-on is not configured")
		return
	}
	provider := chi.URLParam(r, "provider")
	state := randomToken()
	verifier := oauth2.GenerateVerifier()

	url, err := m.AuthCodeURL(provider, state, verifier)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "unknown identity provider")
		return
	}

	// SameSite=Lax so the cookies survive the top-level redirect back from the
	// provider's domain (Strict would drop them on that cross-site navigation).
	h.setOIDCCookie(w, oidcStateCookie, state)
	h.setOIDCCookie(w, oidcVerifierCookie, verifier)
	http.Redirect(w, r, url, http.StatusFound)
}

// CallbackOIDC completes the flow and establishes a session, then redirects the
// browser to the SPA (which exchanges the refresh cookie for an access token).
func (h *Handlers) CallbackOIDC(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	defer h.clearOIDCCookies(w)

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		h.redirectSSOError(w, r)
		return
	}

	stateCookie, err := r.Cookie(oidcStateCookie)
	verifierCookie, err2 := r.Cookie(oidcVerifierCookie)
	if err != nil || err2 != nil || stateCookie.Value == "" || verifierCookie.Value == "" {
		h.redirectSSOError(w, r)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		h.recordAudit(r, "user.login.oidc", "user", "", audit.OutcomeFailure, map[string]any{"provider": provider, "reason": "state mismatch"})
		h.redirectSSOError(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	user, pair, err := h.Auth.LoginWithOIDC(r.Context(), provider, code, verifierCookie.Value, httpx.ClientIP(r), r.UserAgent())
	if err != nil {
		h.recordAudit(r, "user.login.oidc", "user", "", audit.OutcomeFailure, map[string]any{"provider": provider, "reason": err.Error()})
		if errors.Is(err, auth.ErrAccountSuspended) {
			h.redirectSSOError(w, r)
			return
		}
		h.redirectSSOError(w, r)
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTokenExpiresAt)
	h.recordAudit(r, "user.login.oidc", "user", user.ID, audit.OutcomeSuccess, map[string]any{"provider": provider, "email": user.Email})
	http.Redirect(w, r, h.postLoginRedirect(), http.StatusFound)
}

func (h *Handlers) postLoginRedirect() string {
	if h.OIDCPostLoginRedirect != "" {
		return h.OIDCPostLoginRedirect
	}
	return "/"
}

func (h *Handlers) redirectSSOError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login?sso_error=1", http.StatusFound)
}

func (h *Handlers) setOIDCCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((10 * time.Minute).Seconds()),
	})
}

func (h *Handlers) clearOIDCCookies(w http.ResponseWriter) {
	for _, name := range []string{oidcStateCookie, oidcVerifierCookie} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/api/v1/auth",
			HttpOnly: true, Secure: h.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
}

func randomToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
