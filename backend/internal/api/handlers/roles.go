package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/auth"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

func (h *Handlers) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.AuthRepository.ListPermissions(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list permissions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, perms)
}

type roleRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	PermissionIDs []string `json:"permissionIds"`
}

// CreateRole defines a new custom role - always non-system, so it can
// always be later updated or its bindings revoked. Only the three seeded
// roles (admin/editor/viewer) are system roles.
func (h *Handlers) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	role, err := h.AuthRepository.CreateRole(r.Context(), req.Name, req.Description, req.PermissionIDs)
	if err != nil {
		h.recordAudit(r, "role.create", "role", "", audit.OutcomeFailure, map[string]any{"name": req.Name, "error": err.Error()})
		writeRoleError(w, err)
		return
	}

	h.recordAudit(r, "role.create", "role", role.ID, audit.OutcomeSuccess, map[string]any{"name": role.Name})
	httpx.WriteJSON(w, http.StatusCreated, role)
}

// UpdateRole replaces a custom role's description and permission set.
// Rejects (409) if the target is a system role - admin/editor/viewer stay a
// stable, predictable baseline that can't be neutered via the API.
func (h *Handlers) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req roleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	role, err := h.AuthRepository.UpdateRole(r.Context(), id, req.Description, req.PermissionIDs)
	if err != nil {
		writeRoleError(w, err)
		return
	}

	h.recordAudit(r, "role.update", "role", id, audit.OutcomeSuccess, map[string]any{"name": role.Name})
	httpx.WriteJSON(w, http.StatusOK, role)
}

func writeRoleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "role not found")
	case errors.Is(err, auth.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "a role with this name already exists")
	case errors.Is(err, auth.ErrSystemRole):
		httpx.WriteError(w, http.StatusConflict, "system_role", "system roles (admin/editor/viewer) cannot be modified")
	case errors.Is(err, auth.ErrInvalidPermission):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "one or more permission ids are invalid")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "role operation failed")
	}
}
