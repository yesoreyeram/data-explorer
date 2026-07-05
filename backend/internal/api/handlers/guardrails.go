package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

type guardrailStat struct {
	LimitType string `json:"limitType"`
	Count     int    `json:"count"`
}

func (h *Handlers) GuardrailStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Audit.GuardrailTrips(r.Context(), time.Now().Add(-24*time.Hour))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load guardrail stats")
		return
	}
	out := make([]guardrailStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, guardrailStat{LimitType: stat.LimitType, Count: stat.Count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LimitType < out[j].LimitType })
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"windowHours": 24, "items": out})
}
