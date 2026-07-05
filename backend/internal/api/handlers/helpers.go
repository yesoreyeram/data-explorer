package handlers

import (
	"net/http"

	"github.com/yesoreyeram/data-explorer/backend/internal/api/middleware"
	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

// recordAudit builds a consistent audit.Event from the current request
// (actor, IP, user agent, correlation id) and hands it to the audit service.
func (h *Handlers) recordAudit(r *http.Request, action, resourceType, resourceID string, outcome audit.Outcome, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["requestId"] = middleware.RequestIDFromContext(r.Context())

	evt := audit.Event{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    httpx.ClientIP(r),
		UserAgent:    r.UserAgent(),
		Outcome:      outcome,
		Metadata:     metadata,
	}
	if p, ok := rbac.FromContext(r.Context()); ok {
		evt.ActorID = p.UserID
		evt.ActorEmail = p.Email
	}
	h.Audit.Record(r.Context(), evt)
}

func principalOrEmpty(r *http.Request) rbac.Principal {
	p, _ := rbac.FromContext(r.Context())
	return p
}

func (h *Handlers) recordGuardrailTrip(r *http.Request, limitType string, metadata map[string]any) {
	if limitType == "" {
		return
	}
	h.recordAudit(r, "guardrail.trip."+limitType, "guardrail", limitType, audit.OutcomeFailure, metadata)
}
