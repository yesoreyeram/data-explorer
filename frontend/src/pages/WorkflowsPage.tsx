import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import { createWorkflow, deleteWorkflow, listWorkflows } from "../api/workflows";
import { listFolders } from "../api/folders";
import { extractErrorMessage } from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { FolderSelect } from "../components/FolderSelect";
import { PERMISSIONS } from "../lib/permissions";
import { useAuthStore } from "../state/authStore";
import { IconClock, IconPlus, IconTrash, IconWorkflow } from "../components/icons";
import { Modal } from "../components/Modal";
import { Badge, Button, Card, CardBody, Field, IconButton, Input } from "../components/ui";

export function WorkflowsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: workflows = [], isLoading, error } = useQuery({ queryKey: ["workflows"], queryFn: listWorkflows });
  const { data: folders = [] } = useQuery({ queryKey: ["folders"], queryFn: () => listFolders() });
  const hasScopedPermission = useAuthStore((s) => s.hasScopedPermission);
  const hasAnyScopedPermission = useAuthStore((s) => s.hasAnyScopedPermission);

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [folderId, setFolderId] = useState("");
  const [folderFilter, setFolderFilter] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const visibleWorkflows = folderFilter ? workflows.filter((w) => w.folderId === folderFilter) : workflows;
  const folderName = (id: string) => folders.find((f) => f.id === id)?.name ?? "";
  const scopeChainFor = (folderId: string): string[] => {
    const folder = folders.find((f) => f.id === folderId);
    return folder ? [folder.id, ...folder.ancestorIds] : [folderId];
  };

  const createMutation = useMutation({
    mutationFn: () => createWorkflow({ name, description, definition: { nodes: [], edges: [] }, folderId }),
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
    if (!folderId) {
      setFormError("Please choose a folder for this workflow.");
      return;
    }
    setFormError(null);
    try {
      await createMutation.mutateAsync();
      setCreating(false);
      setName("");
      setDescription("");
      setFolderId("");
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
        {hasAnyScopedPermission(PERMISSIONS.workflowsWrite) && (
          <Button variant="primary" onClick={() => setCreating(true)}>
            <IconPlus width={14} height={14} /> New workflow
          </Button>
        )}
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <Field htmlFor="wf-folder-filter" label="Filter by folder" style={{ maxWidth: 260, marginBottom: 12 }}>
        <FolderSelect id="wf-folder-filter" folders={folders} value={folderFilter} onChange={setFolderFilter} placeholder="All folders" />
      </Field>

      {isLoading ? (
        <p>Loading…</p>
      ) : visibleWorkflows.length === 0 ? (
        <div className="empty-state">
          <IconWorkflow width={28} height={28} />
          <p>No workflows yet. Create one to start stitching data together.</p>
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))", gap: 12 }}>
          {visibleWorkflows.map((wf) => (
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
                  <span>
                    {folderName(wf.folderId)} &middot; {wf.definition.nodes?.length ?? 0} nodes &middot; v{wf.version}
                  </span>
                  {hasScopedPermission(PERMISSIONS.workflowsWrite, scopeChainFor(wf.folderId)) && (
                    <IconButton
                      label="Delete"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(wf.id, wf.name);
                      }}
                    >
                      <IconTrash width={13} height={13} />
                    </IconButton>
                  )}
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
            <Field htmlFor="wf-folder" label="Folder">
              <FolderSelect id="wf-folder" folders={folders} value={folderId} onChange={setFolderId} placeholder="Select a folder…" />
            </Field>
          </form>
        </Modal>
      )}
    </div>
  );
}
