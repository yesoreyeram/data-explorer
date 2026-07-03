package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
)

func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.AuthRepository.ListUsers(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}

	allRoles, err := h.AuthRepository.ListRoles(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list roles")
		return
	}
	roleIDByName := make(map[string]string, len(allRoles))
	for _, role := range allRoles {
		roleIDByName[role.Name] = role.ID
	}

	for i := range users {
		roleNames, _, err := h.AuthRepository.GetUserRolesAndPermissions(r.Context(), users[i].ID)
		if err == nil {
			for _, name := range roleNames {
				users[i].Roles = append(users[i].Roles, domain.Role{ID: roleIDByName[name], Name: name})
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

type setStatusRequest struct {
	Status domain.UserStatus `json:"status"`
}

func (h *Handlers) SetUserStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Status != domain.UserStatusActive && req.Status != domain.UserStatusSuspended {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "status must be 'active' or 'suspended'")
		return
	}

	if err := h.AuthRepository.SetUserStatus(r.Context(), id, req.Status); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	h.recordAudit(r, "user.status_change", "user", id, audit.OutcomeSuccess, map[string]any{"status": req.Status})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": string(req.Status)})
}

func (h *Handlers) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.AuthRepository.ListRoles(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list roles")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, roles)
}

type setUserRolesRequest struct {
	RoleIDs []string `json:"roleIds"`
}

func (h *Handlers) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setUserRolesRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	if err := h.AuthRepository.SetUserRoles(r.Context(), id, req.RoleIDs); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update roles")
		return
	}

	h.recordAudit(r, "user.roles_change", "user", id, audit.OutcomeSuccess, map[string]any{"roleIds": req.RoleIDs})
	httpx.WriteJSON(w, http.StatusOK, map[string][]string{"roleIds": req.RoleIDs})
}
