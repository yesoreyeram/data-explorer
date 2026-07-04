package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
)

type HealthHandler struct {
	db       *pgxpool.Pool
	workflows *workflow.Service
	shutdown *ShutdownState
}

func NewHealthHandler(db *pgxpool.Pool, workflows *workflow.Service, shutdown *ShutdownState) *HealthHandler {
	return &HealthHandler{db: db, workflows: workflows, shutdown: shutdown}
}

type ShutdownState struct {
	mu       sync.RWMutex
	draining bool
	deadline time.Time
}

func NewShutdownState() *ShutdownState {
	return &ShutdownState{}
}

func (s *ShutdownState) Begin(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.draining = true
	s.deadline = time.Now().Add(timeout)
}

func (s *ShutdownState) Snapshot() (bool, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.draining, s.deadline
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

func (h *HealthHandler) ShutdownStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	draining, deadline := h.shutdown.Snapshot()
	inflight := 0
	if h.workflows != nil {
		inflight = h.workflows.InFlightRuns()
	}
	if !draining {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status":           "serving",
			"draining":         false,
			"inflightRuns":     inflight,
			"remainingDrainMs": 0,
		})
		return
	}

	remaining := time.Until(deadline)
	if remaining < 0 {
		remaining = 0
	}
	httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
		"status":           "draining",
		"draining":         true,
		"inflightRuns":     inflight,
		"remainingDrainMs": remaining.Milliseconds(),
	})
}
