package middleware

import (
	"net/http"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/logger"
)

// Recover turns a panic in any downstream handler into a 500 response
// instead of crashing the process, and logs the panic with a stack trace and
// the request id for debugging.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log := logger.FromContext(r.Context())
				log.Error("panic recovered", "panic", rec, "request_id", RequestIDFromContext(r.Context()), "path", r.URL.Path)
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
