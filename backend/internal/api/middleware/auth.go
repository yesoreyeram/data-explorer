package middleware

import (
	"net/http"
	"strings"

	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

// Authenticate validates the bearer JWT (if present) and attaches the
// resolved rbac.Principal to the request context. It does not itself reject
// unauthenticated requests - RequireAuth (or a specific permission check)
// does that - so public endpoints (login, register, health) can share the
// same middleware chain.
func Authenticate(tokens *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				next.ServeHTTP(w, r)
				return
			}

			principal, err := tokens.ParseAccessToken(parts[1])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := rbac.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth rejects requests that did not resolve to a Principal.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := rbac.FromContext(r.Context()); !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "a valid access token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission rejects requests whose Principal lacks the given
// permission code. It implies RequireAuth.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := rbac.FromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated", "a valid access token is required")
				return
			}
			if !principal.Has(permission) {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to perform this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
