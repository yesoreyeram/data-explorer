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
			r.With(custommw.RequirePermission(rbac.PermRolesWrite)).Post("/", h.CreateRole)
			r.With(custommw.RequirePermission(rbac.PermRolesWrite)).Put("/{id}", h.UpdateRole)
		})

		r.Route("/permissions", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermRolesRead)).Get("/", h.ListPermissions)
		})

		// Connections routes use RequireAuth, not RequirePermission: access
		// can be scoped to a folder subtree via folder_role_bindings, so the
		// actual check needs the target connection's folder id/ancestry,
		// which is only known once the handler loads it (or, for Create,
		// once the request body's folderId is known) - see
		// connections.go's per-handler principal.HasScoped calls.
		r.Route("/connections", func(r chi.Router) {
			r.With(custommw.RequireAuth).Get("/", h.ListConnections)
			r.With(custommw.RequireAuth).Post("/", h.CreateConnection)
			r.With(custommw.RequireAuth).Get("/{id}", h.GetConnection)
			r.With(custommw.RequireAuth).Put("/{id}", h.UpdateConnection)
			r.With(custommw.RequireAuth).Delete("/{id}", h.DeleteConnection)
			r.With(custommw.RequireAuth).Post("/{id}/test", h.TestConnection)
			r.With(custommw.RequireAuth).Post("/{id}/query", h.QueryConnection)
		})

		r.Route("/folders", func(r chi.Router) {
			// RequireAuth, not RequirePermission: folder access can be
			// scoped to a specific subtree via folder_role_bindings, so the
			// actual authorization decision needs the target folder's
			// id/ancestry, which is only known once the handler loads it -
			// see folders.go's per-handler principal.HasScoped calls.
			r.With(custommw.RequireAuth).Get("/", h.ListFolders)
			r.With(custommw.RequireAuth).Post("/", h.CreateFolder)
			r.With(custommw.RequireAuth).Get("/{id}", h.GetFolder)
			r.With(custommw.RequireAuth).Put("/{id}", h.UpdateFolder)
			r.With(custommw.RequireAuth).Post("/{id}/move", h.MoveFolder)
			r.With(custommw.RequireAuth).Delete("/{id}", h.DeleteFolder)
			r.With(custommw.RequireAuth).Get("/{id}/access", h.ListFolderAccess)
			r.With(custommw.RequireAuth).Post("/{id}/access", h.GrantFolderAccess)
			r.With(custommw.RequireAuth).Delete("/{id}/access/{bindingId}", h.RevokeFolderAccess)
		})

		r.Route("/catalog", func(r chi.Router) {
			r.With(custommw.RequirePermission(rbac.PermConnectionsRead)).Get("/", h.ListCatalog)
		})

		r.Route("/explore", func(r chi.Router) {
			// RequireAuth: a saved connection's read access may be scoped to
			// its folder - see ExploreQuery's authorizeConnectionAction call.
			// Temporary (never-persisted) connections have no folder to
			// scope, so ExploreQuery itself still requires connections:test
			// globally for that path.
			r.With(custommw.RequireAuth).Post("/query", h.ExploreQuery)
		})

		// Workflows routes use RequireAuth for the same reason connections
		// routes do - see the comment above the /connections route group.
		r.Route("/workflows", func(r chi.Router) {
			r.With(custommw.RequireAuth).Get("/", h.ListWorkflows)
			r.With(custommw.RequireAuth).Post("/", h.CreateWorkflow)
			r.With(custommw.RequireAuth).Get("/{id}", h.GetWorkflow)
			r.With(custommw.RequireAuth).Put("/{id}", h.UpdateWorkflow)
			r.With(custommw.RequireAuth).Delete("/{id}", h.DeleteWorkflow)
			r.With(custommw.RequireAuth).Put("/{id}/schedule", h.SetWorkflowSchedule)
			r.With(custommw.RequireAuth).Post("/{id}/execute", h.ExecuteWorkflow)
			r.With(custommw.RequireAuth).Get("/{id}/executions", h.ListWorkflowExecutions)
			r.With(custommw.RequireAuth).Get("/{id}/executions/{executionId}", h.GetWorkflowExecution)
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
