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

func NewRouter(cfg *config.Config, h *handlers.Handlers, health *handlers.HealthHandler, tokens *auth.TokenManager, metrics *observability.Metrics) http.Handler {
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

	authLimiter := custommw.NewIPRateLimiter(2, 10) // ~2 req/s sustained, burst 10 - blunts credential stuffing
	generalLimiter := custommw.NewIPRateLimiter(20, 60)
	r.Use(generalLimiter.Middleware)

	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Readyz)
	r.Get("/status/shutdown", health.ShutdownStatus)
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(authLimiter.Middleware).Post("/register", h.Register)
			r.With(authLimiter.Middleware).Post("/login", h.Login)
			r.With(authLimiter.Middleware).Post("/refresh", h.Refresh)
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

		r.With(custommw.RequireAuth).Get("/search", h.Search)

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

		r.Route("/admin", func(r chi.Router) {
			r.With(custommw.RequireRole("admin")).Get("/guardrails/stats", h.GuardrailStats)
		})
	})

	return r
}

// ShutdownTimeout is how long the server waits for in-flight requests to
// finish during a graceful shutdown.
func ShutdownTimeout(cfg *config.Config) time.Duration {
	return cfg.HTTP.ShutdownTimeout
}
