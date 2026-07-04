package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/observability"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/logger"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// AccessLog logs one structured line per request and records the same
// request as a Prometheus observation, keyed by the matched route pattern
// (not the raw path, which would blow up cardinality with path parameters).
func AccessLog(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			requestLogger := logger.FromContext(r.Context()).With(
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
			)
			ctx := logger.WithContext(r.Context(), requestLogger)

			next.ServeHTTP(rec, r.WithContext(ctx))

			duration := time.Since(start)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			actor := "anonymous"
			if p, ok := rbac.FromContext(r.Context()); ok {
				actor = p.Email
			}

			requestLogger.Info("http request",
				"status", rec.status,
				"duration_ms", duration.Milliseconds(),
				"route", route,
				"actor", actor,
				"remote_addr", r.RemoteAddr,
			)

			if metrics != nil {
				metrics.ObserveHTTP(route, r.Method, rec.status, duration)
			}
		})
	}
}
