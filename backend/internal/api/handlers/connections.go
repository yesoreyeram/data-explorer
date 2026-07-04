package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// connectionScopeChain resolves a connection's folder ancestry for an
// rbac.Principal.HasScoped check - a single indexed folder lookup, only
// ever needed for principals that don't already hold the permission
// globally (see rbac.Principal.Has's fast path).
func (h *Handlers) connectionScopeChain(ctx context.Context, folderID string) ([]string, error) {
	return h.Folders.ScopeChain(ctx, folderID)
}

// authorizeConnectionAction loads the connection, checks permission scoped
// to its folder, and writes the appropriate error response if either the
// lookup or the authorization check fails. Returns true iff the caller
// should proceed. Used by actions (Test/Query) that don't otherwise need
// the loaded connection before re-fetching it inside the service call.
func (h *Handlers) authorizeConnectionAction(w http.ResponseWriter, r *http.Request, id, permission string) bool {
	conn, err := h.Connections.Get(r.Context(), id)
	if err != nil {
		writeConnectionsError(w, err)
		return false
	}
	p := principalOrEmpty(r)
	chain, err := h.connectionScopeChain(r.Context(), conn.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve connection's folder")
		return false
	}
	if !p.HasScoped(permission, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to perform this action on this connection")
		return false
	}
	return true
}

func (h *Handlers) ListConnections(w http.ResponseWriter, r *http.Request) {
	p := principalOrEmpty(r)
	filter := connections.ListFilter{}
	grantedIDs, global := p.GrantedFolderIDs(rbac.PermConnectionsRead)
	if !global {
		if len(grantedIDs) == 0 {
			httpx.WriteJSON(w, http.StatusOK, []domain.Connection{})
			return
		}
		filter.FolderIDs = grantedIDs
	}

	conns, err := h.Connections.List(r.Context(), filter)
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
	p := principalOrEmpty(r)
	chain, err := h.connectionScopeChain(r.Context(), conn.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve connection's folder")
		return
	}
	if !p.HasScoped(rbac.PermConnectionsRead, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to view this connection")
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
	FolderID    string            `json:"folderId"`
}

func (h *Handlers) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var req connectionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Name == "" || req.Type == "" || req.FolderID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "name, type, and folderId are required")
		return
	}

	p := principalOrEmpty(r)
	chain, err := h.connectionScopeChain(r.Context(), req.FolderID)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	if !p.HasScoped(rbac.PermConnectionsWrite, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to create connections in this folder")
		return
	}

	conn, err := h.Connections.Create(r.Context(), connections.CreateInput{
		Name:        req.Name,
		Type:        domain.ConnectionType(req.Type),
		Description: req.Description,
		Config:      req.Config,
		Secret:      req.Secret,
		FolderID:    req.FolderID,
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

	existing, err := h.Connections.Get(r.Context(), id)
	if err != nil {
		writeConnectionsError(w, err)
		return
	}
	p := principalOrEmpty(r)
	currentChain, err := h.connectionScopeChain(r.Context(), existing.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve connection's folder")
		return
	}
	if !p.HasScoped(rbac.PermConnectionsWrite, currentChain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to update this connection")
		return
	}
	// Moving a connection to a different folder needs write access at the
	// destination too - otherwise write access to folder A would let a
	// caller relocate a connection into folder B without ever having been
	// granted anything on B.
	if req.FolderID != "" && req.FolderID != existing.FolderID {
		destChain, err := h.connectionScopeChain(r.Context(), req.FolderID)
		if err != nil {
			writeFoldersError(w, err)
			return
		}
		if !p.HasScoped(rbac.PermConnectionsWrite, destChain) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to move a connection into this folder")
			return
		}
	}
	folderID := req.FolderID
	if folderID == "" {
		folderID = existing.FolderID
	}

	conn, err := h.Connections.Update(r.Context(), id, connections.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
		FolderID:    folderID,
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
	existing, err := h.Connections.Get(r.Context(), id)
	if err != nil {
		writeConnectionsError(w, err)
		return
	}
	p := principalOrEmpty(r)
	chain, err := h.connectionScopeChain(r.Context(), existing.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve connection's folder")
		return
	}
	if !p.HasScoped(rbac.PermConnectionsWrite, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to delete this connection")
		return
	}

	if err := h.Connections.Delete(r.Context(), id); err != nil {
		writeConnectionsError(w, err)
		return
	}
	h.recordAudit(r, "connection.delete", "connection", id, audit.OutcomeSuccess, nil)
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !h.authorizeConnectionAction(w, r, id, rbac.PermConnectionsTest) {
		return
	}
	result, err := h.Connections.Test(r.Context(), id)

	outcome := audit.OutcomeSuccess
	meta := map[string]any{"durationMs": result.DurationMs, "status": result.Status}
	if err != nil {
		outcome = audit.OutcomeFailure
		meta["error"] = err.Error()
		if result.ErrorCode != "" {
			meta["errorCode"] = result.ErrorCode
		}
		if result.ErrorRemediation != "" {
			meta["errorRemediation"] = result.ErrorRemediation
		}
	}
	h.recordAudit(r, "connection.test", "connection", id, outcome, meta)

	if err != nil {
		if errors.Is(err, connections.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
			return
		}
		if errors.Is(err, connections.ErrRateLimited) {
			httpx.WriteRateLimit(w, connections.DefaultConnectionRateBurst, connections.DefaultConnectionRateBurst, 2*time.Second, time.Second, "This connection is being called too frequently. Slow down or retry after the indicated delay.")
			return
		}
		var he *connections.HealthError
		if errors.As(err, &he) {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"status":           result.Status,
				"lastTestedAt":     result.LastTestedAt.Format(httpTimeFormat),
				"durationMs":       result.DurationMs,
				"healthy":          false,
				"error":            result.Error,
				"errorCode":        result.ErrorCode,
				"errorRemediation": result.ErrorRemediation,
				"errorDetail":      he.Detail(),
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"healthy": false, "error": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":       result.Status,
		"lastTestedAt": result.LastTestedAt.Format(httpTimeFormat),
		"durationMs":   result.DurationMs,
		"healthy":      true,
	})
}

func (h *Handlers) QueryConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var spec connections.QuerySpec
	if err := httpx.DecodeJSON(r, &spec); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if !h.authorizeConnectionAction(w, r, id, rbac.PermConnectionsRead) {
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
		h.observeFrameWarnings(result)
	}
	h.recordAudit(r, "connection.query", "connection", id, outcome, meta)

	if err != nil {
		writeQueryError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) observeFrameWarnings(frame *dataframe.Frame) {
	if h.Metrics == nil || frame == nil {
		return
	}
	for _, warning := range frame.Meta.Warnings {
		if strings.Contains(warning, "80%") && strings.Contains(warning, "cap") {
			h.Metrics.ObserveGuardrailSoftWarning("http_response_body_bytes", 0.8)
		}
	}
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
		httpx.WriteRateLimit(w, connections.DefaultConnectionRateBurst, connections.DefaultConnectionRateBurst, 2*time.Second, time.Second, "This connection is being called too frequently. Slow down or retry after the indicated delay.")
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
