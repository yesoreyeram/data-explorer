package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

type exploreConnection struct {
	Type   string            `json:"type"`
	Config json.RawMessage   `json:"config"`
	Secret map[string]string `json:"secret"`
}

type exploreQueryRequest struct {
	// Exactly one of ConnectionID (reuse a saved connection) or Connection
	// (a definition that is never persisted) must be set.
	ConnectionID string                `json:"connectionId,omitempty"`
	Connection   *exploreConnection    `json:"connection,omitempty"`
	Spec         connections.QuerySpec `json:"spec"`
}

// ExploreQuery powers the ad-hoc exploration page: query either an existing
// saved connection, or a connection definition supplied inline that this
// handler never writes to storage - see connections.Service.QueryAdhoc for
// why that path needs its own permission check (connections:test, since it
// dials out to an arbitrary target with credentials the caller supplies
// live, the same risk profile as testing a saved connection).
func (h *Handlers) ExploreQuery(w http.ResponseWriter, r *http.Request) {
	var req exploreQueryRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	if req.ConnectionID == "" && req.Connection == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "connectionId or connection is required")
		return
	}
	if req.ConnectionID != "" && req.Connection != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "connectionId and connection are mutually exclusive")
		return
	}
	if req.Connection != nil && req.Connection.Type == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "connection.type is required")
		return
	}

	p := principalOrEmpty(r)
	if req.Connection != nil && !p.Has(rbac.PermConnectionsTest) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "temporary connections require the connections:test permission")
		return
	}

	var (
		result     *dataframe.Frame
		err        error
		resourceID string
	)
	if req.ConnectionID != "" {
		if !h.authorizeConnectionAction(w, r, req.ConnectionID, rbac.PermConnectionsRead) {
			return
		}
		resourceID = req.ConnectionID
		result, err = h.Connections.Query(r.Context(), req.ConnectionID, req.Spec)
	} else {
		resourceID = "adhoc:" + req.Connection.Type
		result, err = h.Connections.QueryAdhoc(r.Context(), p.UserID, req.Connection.Type, req.Connection.Config, req.Connection.Secret, req.Spec)
	}

	outcome := audit.OutcomeSuccess
	meta := map[string]any{"rowLimit": req.Spec.RowLimit, "adhoc": req.Connection != nil}
	if err != nil {
		outcome = audit.OutcomeFailure
		meta["error"] = err.Error()
	} else {
		meta["rowCount"] = result.NumRows()
		h.observeFrameWarnings(result)
	}
	h.recordAudit(r, "connection.query", "connection", resourceID, outcome, meta)

	if err != nil {
		writeQueryError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
