package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

func (h *Handlers) ListConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := h.Connections.List(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list connections")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, conns)
}

func (h *Handlers) GetConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn, err := h.Connections.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, conn)
}

type connectionRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Config      json.RawMessage   `json:"config"`
	Secret      map[string]string `json:"secret"`
}

func (h *Handlers) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var req connectionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Name == "" || req.Type == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "name and type are required")
		return
	}

	p := principalOrEmpty(r)
	conn, err := h.Connections.Create(r.Context(), connections.CreateInput{
		Name:        req.Name,
		Type:        domain.ConnectionType(req.Type),
		Description: req.Description,
		Config:      req.Config,
		Secret:      req.Secret,
		CreatedBy:   p.UserID,
	})
	if err != nil {
		h.recordAudit(r, "connection.create", "connection", "", audit.OutcomeFailure, map[string]any{"name": req.Name, "error": err.Error()})
		writeConnectionsError(w, err)
		return
	}

	h.recordAudit(r, "connection.create", "connection", conn.ID, audit.OutcomeSuccess, map[string]any{"name": conn.Name, "type": conn.Type})
	httpx.WriteJSON(w, http.StatusCreated, conn)
}

func (h *Handlers) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req connectionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	conn, err := h.Connections.Update(r.Context(), id, connections.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
		Secret:      req.Secret,
	})
	if err != nil {
		writeConnectionsError(w, err)
		return
	}

	h.recordAudit(r, "connection.update", "connection", id, audit.OutcomeSuccess, map[string]any{"name": conn.Name})
	httpx.WriteJSON(w, http.StatusOK, conn)
}

func (h *Handlers) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Connections.Delete(r.Context(), id); err != nil {
		writeConnectionsError(w, err)
		return
	}
	h.recordAudit(r, "connection.delete", "connection", id, audit.OutcomeSuccess, nil)
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.Connections.Test(r.Context(), id)

	outcome := audit.OutcomeSuccess
	meta := map[string]any{}
	if err != nil {
		outcome = audit.OutcomeFailure
		meta["error"] = err.Error()
	}
	h.recordAudit(r, "connection.test", "connection", id, outcome, meta)

	if err != nil {
		if errors.Is(err, connections.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
			return
		}
		if errors.Is(err, connections.ErrRateLimited) {
			httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
			return
		}
		var he *connections.HealthError
		if errors.As(err, &he) {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"healthy":          false,
				"error":            he.Message,
				"errorCode":        string(he.Code),
				"errorRemediation": he.Remediation,
				"errorDetail":      he.Detail(),
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"healthy": false, "error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"healthy": true})
}

func (h *Handlers) QueryConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var spec connections.QuerySpec
	if err := httpx.DecodeJSON(r, &spec); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	result, err := h.Connections.Query(r.Context(), id, spec)

	outcome := audit.OutcomeSuccess
	meta := map[string]any{"rowLimit": spec.RowLimit}
	if err != nil {
		outcome = audit.OutcomeFailure
		meta["error"] = err.Error()
	} else {
		meta["rowCount"] = result.NumRows()
	}
	h.recordAudit(r, "connection.query", "connection", id, outcome, meta)

	if err != nil {
		writeQueryError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// writeQueryError maps a query/test execution error to an HTTP response,
// preserving the structured Code/Remediation/Detail from a classified
// connections.HealthError (see connections.Classify) instead of flattening
// it to a bare message - shared by QueryConnection and ExploreQuery, the two
// handlers that run a connector query rather than a CRUD operation.
func writeQueryError(w http.ResponseWriter, err error) {
	if errors.Is(err, connections.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
		return
	}
	if errors.Is(err, connections.ErrUnsupportedType) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "unsupported connection type")
		return
	}
	if errors.Is(err, connections.ErrRateLimited) {
		httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
		return
	}
	var he *connections.HealthError
	if errors.As(err, &he) {
		httpx.WriteErrorDetailed(w, http.StatusBadGateway, string(he.Code), he.Message, he.Remediation, he.Detail())
		return
	}
	httpx.WriteError(w, http.StatusBadGateway, "query_failed", err.Error())
}

func writeConnectionsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, connections.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
	case errors.Is(err, connections.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "a connection with this name already exists")
	case errors.Is(err, connections.ErrUnsupportedType):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "unsupported connection type")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "connection operation failed")
	}
}
