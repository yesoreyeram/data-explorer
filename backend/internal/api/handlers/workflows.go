package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/yesoreyeram/data-explorer/backend/internal/audit"
	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/platform/httpx"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
)

// workflowScopeChain resolves a workflow's folder ancestry for an
// rbac.Principal.HasScoped check - see connections.go's identical helper.
func (h *Handlers) workflowScopeChain(r *http.Request, folderID string) ([]string, error) {
	return h.Folders.ScopeChain(r.Context(), folderID)
}

func (h *Handlers) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	p := principalOrEmpty(r)
	filter := workflow.ListFilter{}
	grantedIDs, global := p.GrantedFolderIDs(rbac.PermWorkflowsRead)
	if !global {
		if len(grantedIDs) == 0 {
			httpx.WriteJSON(w, http.StatusOK, []domain.Workflow{})
			return
		}
		filter.FolderIDs = grantedIDs
	}

	workflows, err := h.Workflows.List(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list workflows")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, workflows)
}

func (h *Handlers) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wf, err := h.Workflows.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "workflow not found")
		return
	}
	p := principalOrEmpty(r)
	chain, err := h.workflowScopeChain(r, wf.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve workflow's folder")
		return
	}
	if !p.HasScoped(rbac.PermWorkflowsRead, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to view this workflow")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, wf)
}

type workflowRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"`
	Status      string          `json:"status"`
	FolderID    string          `json:"folderId"`
}

func (h *Handlers) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req workflowRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Name == "" || req.FolderID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "name and folderId are required")
		return
	}

	p := principalOrEmpty(r)
	chain, err := h.workflowScopeChain(r, req.FolderID)
	if err != nil {
		writeFoldersError(w, err)
		return
	}
	if !p.HasScoped(rbac.PermWorkflowsWrite, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to create workflows in this folder")
		return
	}

	wf, err := h.Workflows.Create(r.Context(), req.Name, req.Description, req.Definition, req.FolderID, p.UserID)
	if err != nil {
		h.recordAudit(r, "workflow.create", "workflow", "", audit.OutcomeFailure, map[string]any{"name": req.Name, "error": err.Error()})
		writeWorkflowError(w, err)
		return
	}

	h.recordAudit(r, "workflow.create", "workflow", wf.ID, audit.OutcomeSuccess, map[string]any{"name": wf.Name})
	httpx.WriteJSON(w, http.StatusCreated, wf)
}

func (h *Handlers) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req workflowRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	status := domain.WorkflowStatus(req.Status)
	if status == "" {
		status = domain.WorkflowStatusDraft
	}

	existing, err := h.Workflows.Get(r.Context(), id)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	p := principalOrEmpty(r)
	currentChain, err := h.workflowScopeChain(r, existing.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve workflow's folder")
		return
	}
	if !p.HasScoped(rbac.PermWorkflowsWrite, currentChain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to update this workflow")
		return
	}
	if req.FolderID != "" && req.FolderID != existing.FolderID {
		destChain, err := h.workflowScopeChain(r, req.FolderID)
		if err != nil {
			writeFoldersError(w, err)
			return
		}
		if !p.HasScoped(rbac.PermWorkflowsWrite, destChain) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to move a workflow into this folder")
			return
		}
	}
	folderID := req.FolderID
	if folderID == "" {
		folderID = existing.FolderID
	}

	wf, err := h.Workflows.Update(r.Context(), id, req.Name, req.Description, req.Definition, status, folderID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	h.recordAudit(r, "workflow.update", "workflow", id, audit.OutcomeSuccess, map[string]any{"name": wf.Name, "version": wf.Version})
	httpx.WriteJSON(w, http.StatusOK, wf)
}

// authorizeWorkflowAction loads the workflow, checks permission scoped to
// its folder, and writes the appropriate error response if either the
// lookup or the authorization check fails. Returns true iff the caller
// should proceed.
func (h *Handlers) authorizeWorkflowAction(w http.ResponseWriter, r *http.Request, id, permission string) bool {
	wf, err := h.Workflows.Get(r.Context(), id)
	if err != nil {
		writeWorkflowError(w, err)
		return false
	}
	p := principalOrEmpty(r)
	chain, err := h.workflowScopeChain(r, wf.FolderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve workflow's folder")
		return false
	}
	if !p.HasScoped(permission, chain) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to perform this action on this workflow")
		return false
	}
	return true
}

type workflowScheduleRequest struct {
	Cron    string `json:"cron"`
	Enabled bool   `json:"enabled"`
}

func (h *Handlers) SetWorkflowSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req workflowScheduleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteDecodeError(w, err)
		return
	}
	if req.Enabled && req.Cron == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "cron is required when enabling a schedule")
		return
	}
	if !h.authorizeWorkflowAction(w, r, id, rbac.PermWorkflowsWrite) {
		return
	}

	wf, err := h.Workflows.SetSchedule(r.Context(), id, req.Cron, req.Enabled)
	if err != nil {
		h.recordAudit(r, "workflow.schedule.update", "workflow", id, audit.OutcomeFailure, map[string]any{"cron": req.Cron, "enabled": req.Enabled, "error": err.Error()})
		if errors.Is(err, workflow.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "workflow not found")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	h.recordAudit(r, "workflow.schedule.update", "workflow", id, audit.OutcomeSuccess, map[string]any{"cron": req.Cron, "enabled": req.Enabled})
	httpx.WriteJSON(w, http.StatusOK, wf)
}

func (h *Handlers) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !h.authorizeWorkflowAction(w, r, id, rbac.PermWorkflowsWrite) {
		return
	}
	if err := h.Workflows.Delete(r.Context(), id); err != nil {
		writeWorkflowError(w, err)
		return
	}
	h.recordAudit(r, "workflow.delete", "workflow", id, audit.OutcomeSuccess, nil)
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handlers) ExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !h.authorizeWorkflowAction(w, r, id, rbac.PermWorkflowsExecute) {
		return
	}
	p := principalOrEmpty(r)

	execution, output, err := h.Workflows.Execute(r.Context(), id, p.UserID)

	outcome := audit.OutcomeSuccess
	meta := map[string]any{}
	if err != nil {
		outcome = audit.OutcomeFailure
		meta["error"] = err.Error()
	} else {
		meta["rowCount"] = output.NumRows()
		meta["durationMs"] = execution.DurationMs
		h.observeFrameWarnings(output)
	}
	h.recordAudit(r, "workflow.execute", "workflow", id, outcome, meta)

	if err != nil {
		if errors.Is(err, workflow.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "workflow not found")
			return
		}
		// The execution record itself was still persisted with its failure
		// details, so return 200 with the failed execution + error, not 500 -
		// this is an expected, recorded outcome, not a server bug.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"execution": execution, "error": err.Error()})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"execution": execution, "output": output})
}

func (h *Handlers) ListWorkflowExecutions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !h.authorizeWorkflowAction(w, r, id, rbac.PermWorkflowsRead) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	executions, err := h.Workflows.ListExecutions(r.Context(), id, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list executions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, executions)
}

func (h *Handlers) GetWorkflowExecution(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	if !h.authorizeWorkflowAction(w, r, workflowID, rbac.PermWorkflowsRead) {
		return
	}
	id := chi.URLParam(r, "executionId")
	execution, err := h.Workflows.GetExecution(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "execution not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, execution)
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workflow.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "workflow not found")
	case errors.Is(err, workflow.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "a workflow with this name already exists")
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
	}
}
