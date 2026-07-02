package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

func (h *Handlers) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := audit.ListFilter{
		ActorID:      q.Get("actorId"),
		Action:       q.Get("action"),
		ResourceType: q.Get("resourceType"),
	}
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(q.Get("offset")); err == nil {
		filter.Offset = offset
	}
	if since := q.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := q.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}

	logs, total, err := h.Audit.List(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list audit logs")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": logs, "total": total})
}
