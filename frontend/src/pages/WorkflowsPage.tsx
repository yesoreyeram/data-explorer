import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import { createWorkflow, deleteWorkflow, listWorkflows } from "../api/workflows";
import { extractErrorMessage } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { IconClock, IconPlus, IconTrash, IconWorkflow } from "../components/icons";
import { Modal } from "../components/Modal";
import { Badge, Button, Card, CardBody, Field, IconButton, Input } from "../components/ui";

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
          <Button variant="primary" onClick={() => setCreating(true)}>
            <IconPlus width={14} height={14} /> New workflow
          </Button>
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
            <Card key={wf.id} style={{ cursor: "pointer" }} onClick={() => navigate(`/workflows/${wf.id}`)}>
              <CardBody>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                  <strong>{wf.name}</strong>
                  <div style={{ display: "flex", gap: 4 }}>
                    {wf.scheduleEnabled && (
                      <Badge>
                        <IconClock width={10} height={10} /> scheduled
                      </Badge>
                    )}
                    <StatusBadge status={wf.status} />
                  </div>
                </div>
                <p style={{ color: "var(--text-secondary)", fontSize: 12, minHeight: 32 }}>{wf.description || "No description"}</p>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: 11, color: "var(--text-tertiary)" }}>
                  <span>{wf.definition.nodes?.length ?? 0} nodes &middot; v{wf.version}</span>
                  <PermissionGate permission={PERMISSIONS.workflowsWrite}>
                    <IconButton
                      label="Delete"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(wf.id, wf.name);
                      }}
                    >
                      <IconTrash width={13} height={13} />
                    </IconButton>
                  </PermissionGate>
                </div>
              </CardBody>
            </Card>
          ))}
        </div>
      )}

      {creating && (
        <Modal
          title="New workflow"
          onClose={() => setCreating(false)}
          footer={
            <>
              <Button onClick={() => setCreating(false)}>Cancel</Button>
              <Button variant="primary" type="submit" form="new-workflow-form" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Creating…" : "Create"}
              </Button>
            </>
          }
        >
          {formError && <div className="error-banner">{formError}</div>}
          <form id="new-workflow-form" onSubmit={handleCreate}>
            <Field htmlFor="wf-name" label="Name">
              <Input id="wf-name" required value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field htmlFor="wf-desc" label="Description">
              <Input id="wf-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
            </Field>
          </form>
        </Modal>
      )}
    </div>
  );
}
