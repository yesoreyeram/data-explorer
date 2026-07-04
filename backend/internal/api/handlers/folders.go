package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/folders"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

// scopeChain returns a folder's own id followed by its ancestor ids -
// exactly what rbac.Principal.HasScoped needs to test a grant on the folder
// itself or any ancestor.
func scopeChain(f domain.Folder) []string {
	return append([]string{f.ID}, f.AncestorIDs...)
}

func (h *Handlers) ListFolders(w http.ResponseWriter, r *http.Request) {
	p := principalOrEmpty(r)
	q := r.URL.Query()
	filter := folders.ListFilter{Tag: q.Get("tag"), Q: q.Get("q")}

	grantedIDs, global := p.GrantedFolderIDs(rbac.PermFoldersRead)
	if !global {
		if len(grantedIDs) == 0 {
			// Authenticated, but no folders:read anywhere (global or
			// scoped) - an empty list, not an error; there's nothing to 403
			// on for a list endpoint with no single target resource.
			httpx.WriteJSON(w, http.StatusOK, []domain.Folder{})
			return
		}
		filter.ScopedToFolderIDs = grantedIDs
	}

	list, err := h.Folders.List(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list folders")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handlers) GetFolder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersRead, scopeChain(f)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to view this folder")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, f)
}

type folderRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ParentID    *string         `json:"parentId,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Readme      string          `json:"readme,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// CreateFolder requires folders:write. A root-level folder (no parentId)
// requires it globally - there's no parent to scope the check against. A
// nested folder requires it scoped to the parent (or an ancestor of the
// parent), so e.g. a "team lead" granted folders:write on their team's
// folder can create subfolders under it without being a global admin.
func (h *Handlers) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req folderRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	p := principalOrEmpty(r)
	if req.ParentID == nil {
		if !p.Has(rbac.PermFoldersWrite) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "creating a root-level folder requires the account-wide folders:write permission")
			return
		}
	} else {
		parent, err := h.Folders.Get(r.Context(), *req.ParentID)
		if err != nil {
			writeFoldersError(w, err)
			return
		}
		if !p.HasScoped(rbac.PermFoldersWrite, scopeChain(parent)) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to create folders here")
			return
		}
	}

	f, err := h.Folders.Create(r.Context(), folders.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
		Tags:        req.Tags,
		Readme:      req.Readme,
		Metadata:    req.Metadata,
		CreatedBy:   p.UserID,
	})
	if err != nil {
		h.recordAudit(r, "folder.create", "folder", "", audit.OutcomeFailure, map[string]any{"name": req.Name, "error": err.Error()})
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.create", "folder", f.ID, audit.OutcomeSuccess, map[string]any{"name": f.Name})
	httpx.WriteJSON(w, http.StatusCreated, f)
}

func (h *Handlers) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req folderRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	existing, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersWrite, scopeChain(existing)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to update this folder")
		return
	}

	f, err := h.Folders.Update(r.Context(), id, folders.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Readme:      req.Readme,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.update", "folder", id, audit.OutcomeSuccess, map[string]any{"name": f.Name})
	httpx.WriteJSON(w, http.StatusOK, f)
}

type moveFolderRequest struct {
	ParentID *string `json:"parentId"`
}

// MoveFolder requires folders:write scoped to both the folder being moved
// and its destination - reparenting is effectively "remove from here" +
// "add there," and both ends should require write access, the same way
// moving a connection into a different folder will (see connections.go).
func (h *Handlers) MoveFolder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req moveFolderRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}

	mover, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersWrite, scopeChain(mover)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to move this folder")
		return
	}
	if req.ParentID != nil {
		dest, err := h.Folders.Get(r.Context(), *req.ParentID)
		if err != nil {
			writeFoldersError(w, err)
			return
		}
		if !p.HasScoped(rbac.PermFoldersWrite, scopeChain(dest)) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to move a folder into this location")
			return
		}
	} else if !p.Has(rbac.PermFoldersWrite) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "moving a folder to root requires the account-wide folders:write permission")
		return
	}

	f, err := h.Folders.Move(r.Context(), id, req.ParentID)
	if err != nil {
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.move", "folder", id, audit.OutcomeSuccess, map[string]any{"newParentId": req.ParentID})
	httpx.WriteJSON(w, http.StatusOK, f)
}

func (h *Handlers) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersWrite, scopeChain(existing)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to delete this folder")
		return
	}

	if err := h.Folders.Delete(r.Context(), id); err != nil {
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.delete", "folder", id, audit.OutcomeSuccess, map[string]any{"name": existing.Name})
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

// ListFolderAccess/GrantFolderAccess/RevokeFolderAccess manage folder-scoped
// role bindings (folder_role_bindings) - who, besides whoever holds the
// permission globally, can act within this folder's subtree. Gated by
// folders:manage_access, itself scopable - so a non-admin "team lead"
// granted manage_access on their team's folder can delegate access within
// it without being a global admin.
func (h *Handlers) ListFolderAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersManageAccess, scopeChain(f)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to view this folder's access grants")
		return
	}

	bindings, err := h.Folders.ListAccess(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list folder access")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bindings)
}

type grantFolderAccessRequest struct {
	UserID string `json:"userId"`
	RoleID string `json:"roleId"`
}

func (h *Handlers) GrantFolderAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req grantFolderAccessRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.UserID == "" || req.RoleID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "userId and roleId are required")
		return
	}

	f, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersManageAccess, scopeChain(f)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to grant access to this folder")
		return
	}

	binding, err := h.Folders.GrantAccess(r.Context(), id, req.UserID, req.RoleID, p.UserID)
	if err != nil {
		h.recordAudit(r, "folder.access.grant", "folder", id, audit.OutcomeFailure, map[string]any{"userId": req.UserID, "roleId": req.RoleID, "error": err.Error()})
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.access.grant", "folder", id, audit.OutcomeSuccess, map[string]any{"userId": req.UserID, "roleId": req.RoleID})
	httpx.WriteJSON(w, http.StatusCreated, binding)
}

func (h *Handlers) RevokeFolderAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	bindingID := chi.URLParam(r, "bindingId")

	f, err := h.Folders.Get(r.Context(), id)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	p := principalOrEmpty(r)
	if !p.HasScoped(rbac.PermFoldersManageAccess, scopeChain(f)) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to revoke access on this folder")
		return
	}

	if err := h.Folders.RevokeAccess(r.Context(), bindingID); err != nil {
		writeFoldersError(w, err)
		return
	}

	h.recordAudit(r, "folder.access.revoke", "folder", id, audit.OutcomeSuccess, map[string]any{"bindingId": bindingID})
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

func writeFoldersError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, folders.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "folder not found")
	case errors.Is(err, folders.ErrParentNotFound):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "parent folder not found")
	case errors.Is(err, folders.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "a folder with this name already exists in this location")
	case errors.Is(err, folders.ErrNotEmpty):
		httpx.WriteError(w, http.StatusConflict, "not_empty", err.Error())
	case errors.Is(err, folders.ErrMaxDepthExceeded):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "folder nesting is too deep")
	case errors.Is(err, folders.ErrCycle):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "cannot move a folder into its own descendant")
	case errors.Is(err, folders.ErrSubtreeTooLarge):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "this folder's subtree is too large to move in one operation")
	case errors.Is(err, folders.ErrTooManyBindings):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "this user already has the maximum number of folder-scoped role bindings")
	case errors.Is(err, folders.ErrInvalidGrant):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "the user or role for this grant does not exist")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "folder operation failed")
	}
}
