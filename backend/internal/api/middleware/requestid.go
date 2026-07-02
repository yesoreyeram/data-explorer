package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey struct{}

const RequestIDHeader = "X-Request-Id"

// RequestID assigns a unique id to every request (reusing one supplied by a
// trusted upstream proxy, if present) so it can be correlated across access
// logs, error responses, and audit log entries.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
