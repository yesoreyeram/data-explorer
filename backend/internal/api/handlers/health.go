package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

type HealthHandler struct {
	db *pgxpool.Pool
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

// Healthz reports liveness: the process is up and able to handle requests.
// It never touches the database, so a slow/unreachable DB doesn't cause an
// orchestrator to kill an otherwise-healthy process.
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports readiness: whether this instance can currently serve real
// traffic, which does depend on the database being reachable.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "reason": "database unreachable"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
