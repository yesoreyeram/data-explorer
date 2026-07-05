// Package api wires HTTP routes to handlers and applies the middleware
// chain (request id -> panic recovery -> security headers -> CORS -> access
// log/metrics -> authentication -> per-route authorization) uniformly.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/yesoreyeram/data-explorer/backend/internal/api/handlers"
	custommw "github.com/yesoreyeram/data-explorer/backend/internal/api/middleware"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/config"
	"github.com/yesoreyeram/data-explorer/backend/internal/observability"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

func NewRouter(cfg *config.Config, h *handlers.Handlers, health *handlers.HealthHandler, tokens *auth.TokenManager, metrics *observability.Metrics, generalLimiter, authLimiter custommw.Limiter) http.Handler {
	r := chi.NewRouter()

	r.Use(custommw.RequestID)
	r.Use(custommw.Recover)
	r.Use(custommw.SecurityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.HTTP.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(custommw.AccessLog(metrics))
	r.Use(custommw.Authenticate(tokens))

	// General traffic is limited per authenticated user (falling back to IP);
	// the auth endpoints are limited per IP since there's no principal yet.
	authRateLimit := custommw.RateLimit(authLimiter, custommw.KeyByIP)
	r.Use(custommw.RateLimit(generalLimiter, custommw.KeyByUserOrIP))

	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(authRateLimit).Post("/register", h.Register)
			r.With(authRateLimit).Post("/login", h.Login)
			r.With(authRateLimit).Post("/refresh", h.Refresh)
			r.Post("/logout", h.Logout)
			r.With(custommw.RequireAuth).Get("/me", h.Me)
		})

		r.Route("/users", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermUsersRead)).Get("/", h.ListUsers)
			r.With(custommw.RequirePermission(rbac.PermUsersWrite)).Patch("/{id}/status", h.SetUserStatus)
			r.With(custommw.RequirePermission(rbac.PermRolesWrite)).Put("/{id}/roles", h.SetUserRoles)
		})

		r.Route("/roles", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermRolesRead)).Get("/", h.ListRoles)
		})

		r.Route("/connections", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Get("/", h.ListConnections)
			r.With(custommw.RequirePermission(rbac.PermConnectionsWrite)).Post("/", h.CreateConnection)
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Get("/{id}", h.GetConnection)
			r.With(custommw.RequirePermission(rbac.PermConnectionsWrite)).Put("/{id}", h.UpdateConnection)
			r.With(custommw.RequirePermission(rbac.PermConnectionsWrite)).Delete("/{id}", h.DeleteConnection)
			r.With(custommw.RequirePermission(rbac.PermConnectionsTest)).Post("/{id}/test", h.TestConnection)
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Post("/{id}/query", h.QueryConnection)
		})

		r.Route("/catalog", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Get("/", h.ListCatalog)
		})

		r.Route("/explore", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Post("/query", h.ExploreQuery)
		})

		r.Route("/workflows", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermWorkflowsRead)).Get("/", h.ListWorkflows)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsWrite)).Post("/", h.CreateWorkflow)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsRead)).Get("/{id}", h.GetWorkflow)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsWrite)).Put("/{id}", h.UpdateWorkflow)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsWrite)).Delete("/{id}", h.DeleteWorkflow)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsWrite)).Put("/{id}/schedule", h.SetWorkflowSchedule)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsExecute)).Post("/{id}/execute", h.ExecuteWorkflow)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsRead)).Get("/{id}/executions", h.ListWorkflowExecutions)
			r.With(custommw.RequirePermission(rbac.PermWorkflowsRead)).Get("/{id}/executions/{executionId}", h.GetWorkflowExecution)
		})

		r.Route("/audit-logs", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermAuditRead)).Get("/", h.ListAuditLogs)
		})
	})

	return r
}

// ShutdownTimeout is how long the server waits for in-flight requests to
// finish during a graceful shutdown.
func ShutdownTimeout(cfg *config.Config) time.Duration {
	return cfg.HTTP.ShutdownTimeout
}
