import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import { createWorkflow, deleteWorkflow, listWorkflows } from "../api/workflows";
import { extractErrorMessage } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { IconPlus, IconTrash, IconWorkflow } from "../components/icons";
import { Modal } from "../components/Modal";

export function WorkflowsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: workflows = [], isLoading, error } = useQuery({ queryKey: ["workflows"], queryFn: listWorkflows });

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: () => createWorkflow({ name, description, definition: { nodes: [], edges: [] } }),
    onSuccess: (wf) => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
      navigate(`/workflows/${wf.id}`);
    },
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteWorkflow(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workflows"] }),
  });

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    try {
      await createMutation.mutateAsync();
      setCreating(false);
      setName("");
      setDescription("");
    } catch (err) {
      setFormError(extractErrorMessage(err));
    }
  }

  async function handleDelete(id: string, wfName: string) {
    if (!confirm(`Delete workflow "${wfName}"? This cannot be undone.`)) return;
    await deleteMutation.mutateAsync(id);
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Workflows</h1>
          <p className="panel-subtitle">Pull, transform, and combine data with a visual pipeline.</p>
        </div>
        <PermissionGate permission={PERMISSIONS.workflowsWrite}>
          <button className="btn btn-primary" type="button" onClick={() => setCreating(true)}>
            <IconPlus width={14} height={14} /> New workflow
          </button>
        </PermissionGate>
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      {isLoading ? (
        <p>Loading…</p>
      ) : workflows.length === 0 ? (
        <div className="empty-state">
          <IconWorkflow width={28} height={28} />
          <p>No workflows yet. Create one to start stitching data together.</p>
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))", gap: 12 }}>
          {workflows.map((wf) => (
            <div key={wf.id} className="card" style={{ cursor: "pointer" }} onClick={() => navigate(`/workflows/${wf.id}`)}>
              <div className="card-body">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                  <strong>{wf.name}</strong>
                  <StatusBadge status={wf.status} />
                </div>
                <p style={{ color: "var(--text-secondary)", fontSize: 12, minHeight: 32 }}>{wf.description || "No description"}</p>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: 11, color: "var(--text-tertiary)" }}>
                  <span>{wf.definition.nodes?.length ?? 0} nodes &middot; v{wf.version}</span>
                  <PermissionGate permission={PERMISSIONS.workflowsWrite}>
                    <button
                      className="icon-btn"
                      title="Delete"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(wf.id, wf.name);
                      }}
                    >
                      <IconTrash width={13} height={13} />
                    </button>
                  </PermissionGate>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {creating && (
        <Modal
          title="New workflow"
          onClose={() => setCreating(false)}
          footer={
            <>
              <button className="btn" type="button" onClick={() => setCreating(false)}>
                Cancel
              </button>
              <button className="btn btn-primary" type="submit" form="new-workflow-form" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Creating…" : "Create"}
              </button>
            </>
          }
        >
          {formError && <div className="error-banner">{formError}</div>}
          <form id="new-workflow-form" onSubmit={handleCreate}>
            <div className="field">
              <label htmlFor="wf-name">Name</label>
              <input id="wf-name" className="input" required value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="wf-desc">Description</label>
              <input id="wf-desc" className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
