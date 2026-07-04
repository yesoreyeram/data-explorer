import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createFolder,
  deleteFolder,
  grantFolderAccess,
  listFolderAccess,
  listFolders,
  moveFolder,
  revokeFolderAccess,
  updateFolder,
  type FolderInput,
} from "../api/folders";
import { listRoles, listUsers } from "../api/users";
import { extractErrorMessage } from "../api/client";
import { useAuthStore } from "../state/authStore";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { FolderTree } from "../components/FolderTree";
import { FolderSelect } from "../components/FolderSelect";
import { Modal } from "../components/Modal";
import { Badge, Button, Field, Input, Select, Textarea } from "../components/ui";
import { IconPlus, IconTrash } from "../components/icons";
import type { Folder } from "../api/types";

export function FoldersPage() {
  const queryClient = useQueryClient();
  const hasScopedPermission = useAuthStore((s) => s.hasScopedPermission);

  const { data: allFolders = [], isLoading, error } = useQuery({ queryKey: ["folders"], queryFn: () => listFolders() });
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [createTarget, setCreateTarget] = useState<{ parentId?: string } | null>(null);
  const [moveTarget, setMoveTarget] = useState<Folder | null>(null);

  useEffect(() => {
    if (!selectedId && allFolders.length > 0) setSelectedId(allFolders[0].id);
  }, [allFolders, selectedId]);

  const selected = allFolders.find((f) => f.id === selectedId);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["folders"] });

  const createMutation = useMutation({
    mutationFn: (input: FolderInput) => createFolder(input),
    onSuccess: (f) => {
      invalidate();
      setSelectedId(f.id);
    },
  });
  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input: FolderInput }) => updateFolder(id, input),
    onSuccess: invalidate,
  });
  const moveMutation = useMutation({
    mutationFn: ({ id, parentId }: { id: string; parentId: string | null }) => moveFolder(id, parentId),
    onSuccess: invalidate,
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteFolder(id),
    onSuccess: () => {
      invalidate();
      setSelectedId(undefined);
    },
  });

  const canWriteSelected = selected ? hasScopedPermission(PERMISSIONS.foldersWrite, [selected.id, ...selected.ancestorIds]) : false;

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Folders</h1>
          <p className="panel-subtitle">Organize connections and workflows into a nested folder hierarchy.</p>
        </div>
        <PermissionGate permission={PERMISSIONS.foldersWrite}>
          <Button variant="primary" onClick={() => setCreateTarget({})}>
            <IconPlus width={14} height={14} /> New root folder
          </Button>
        </PermissionGate>
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <div style={{ display: "grid", gridTemplateColumns: "280px 1fr", gap: 16, alignItems: "start" }}>
        <div className="card" style={{ padding: 8 }}>
          {isLoading ? <p className="field-hint">Loading…</p> : <FolderTree folders={allFolders} selectedId={selectedId} onSelect={(f) => setSelectedId(f.id)} />}
        </div>

        <div>
          {!selected && <div className="empty-state">Select a folder to view its details.</div>}
          {selected && (
            <FolderDetailPanel
              folder={selected}
              allFolders={allFolders}
              canWrite={canWriteSelected}
              onSave={(input) => updateMutation.mutateAsync({ id: selected.id, input })}
              onCreateSubfolder={() => setCreateTarget({ parentId: selected.id })}
              onMove={() => setMoveTarget(selected)}
              onDelete={async () => {
                if (!confirm(`Delete folder "${selected.name}"? It must be empty.`)) return;
                try {
                  await deleteMutation.mutateAsync(selected.id);
                } catch (err) {
                  alert(extractErrorMessage(err));
                }
              }}
            />
          )}
        </div>
      </div>

      {createTarget && (
        <CreateFolderModal
          parentId={createTarget.parentId}
          onClose={() => setCreateTarget(null)}
          onSubmit={(input) => createMutation.mutateAsync(input)}
        />
      )}

      {moveTarget && (
        <Modal title={`Move "${moveTarget.name}"`} onClose={() => setMoveTarget(null)}>
          <MoveFolderForm
            folder={moveTarget}
            allFolders={allFolders}
            onSubmit={async (parentId) => {
              try {
                await moveMutation.mutateAsync({ id: moveTarget.id, parentId });
                setMoveTarget(null);
              } catch (err) {
                alert(extractErrorMessage(err));
              }
            }}
            onCancel={() => setMoveTarget(null)}
          />
        </Modal>
      )}
    </div>
  );
}

function CreateFolderModal({
  parentId,
  onClose,
  onSubmit,
}: {
  parentId?: string;
  onClose: () => void;
  onSubmit: (input: FolderInput) => Promise<unknown>;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit() {
    if (!name.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onSubmit({ name, description, parentId });
      onClose();
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title={parentId ? "New subfolder" : "New root folder"}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={saving || !name.trim()} onClick={handleSubmit}>
            {saving ? "Creating…" : "Create"}
          </Button>
        </>
      }
    >
      {error && <div className="error-banner">{error}</div>}
      <Field htmlFor="folder-name" label="Name">
        <Input id="folder-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
      </Field>
      <Field htmlFor="folder-desc" label="Description">
        <Input id="folder-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
      </Field>
    </Modal>
  );
}

function MoveFolderForm({
  folder,
  allFolders,
  onSubmit,
  onCancel,
}: {
  folder: Folder;
  allFolders: Folder[];
  onSubmit: (parentId: string | null) => Promise<void>;
  onCancel: () => void;
}) {
  const [parentId, setParentId] = useState(folder.parentId ?? "");
  const [saving, setSaving] = useState(false);

  return (
    <>
      <Field htmlFor="move-target" label="New parent folder" hint="Leave blank to move to the root level.">
        <FolderSelect id="move-target" folders={allFolders} value={parentId} onChange={setParentId} excludeId={folder.id} placeholder="(root level)" />
      </Field>
      <div className="modal-footer" style={{ marginTop: 12 }}>
        <Button onClick={onCancel}>Cancel</Button>
        <Button
          variant="primary"
          disabled={saving}
          onClick={async () => {
            setSaving(true);
            await onSubmit(parentId || null);
            setSaving(false);
          }}
        >
          {saving ? "Moving…" : "Move"}
        </Button>
      </div>
    </>
  );
}

function FolderDetailPanel({
  folder,
  allFolders,
  canWrite,
  onSave,
  onCreateSubfolder,
  onMove,
  onDelete,
}: {
  folder: Folder;
  allFolders: Folder[];
  canWrite: boolean;
  onSave: (input: FolderInput) => Promise<unknown>;
  onCreateSubfolder: () => void;
  onMove: () => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(folder.name);
  const [description, setDescription] = useState(folder.description);
  const [tagsText, setTagsText] = useState((folder.tags ?? []).join(", "));
  const [readme, setReadme] = useState(folder.readme ?? "");
  const [metadataText, setMetadataText] = useState(JSON.stringify(folder.metadata ?? {}, null, 2));
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setEditing(false);
    setName(folder.name);
    setDescription(folder.description);
    setTagsText((folder.tags ?? []).join(", "));
    setReadme(folder.readme ?? "");
    setMetadataText(JSON.stringify(folder.metadata ?? {}, null, 2));
    setSaveError(null);
  }, [folder]);

  const breadcrumb = folder.ancestorIds
    .map((id) => allFolders.find((f) => f.id === id)?.name ?? "…")
    .concat(folder.name)
    .join(" / ");

  async function handleSave() {
    let metadata: Record<string, unknown>;
    try {
      metadata = metadataText.trim() ? JSON.parse(metadataText) : {};
    } catch {
      setSaveError("Metadata must be valid JSON.");
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      await onSave({
        name,
        description,
        tags: tagsText
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
        readme,
        metadata,
      });
      setEditing(false);
    } catch (err) {
      setSaveError(extractErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="card" style={{ padding: 16 }}>
      <p className="field-hint" style={{ marginTop: 0 }}>{breadcrumb}</p>

      {saveError && <div className="error-banner">{saveError}</div>}

      {!editing ? (
        <>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
            <div>
              <h2 style={{ margin: "0 0 4px" }}>{folder.name}</h2>
              {folder.description && <p style={{ margin: 0, color: "var(--text-secondary)" }}>{folder.description}</p>}
            </div>
            {canWrite && (
              <div style={{ display: "flex", gap: 6 }}>
                <Button size="sm" onClick={onCreateSubfolder}>
                  <IconPlus width={12} height={12} /> Subfolder
                </Button>
                <Button size="sm" onClick={onMove}>
                  Move
                </Button>
                <Button size="sm" onClick={() => setEditing(true)}>
                  Edit
                </Button>
                <Button size="sm" onClick={onDelete}>
                  <IconTrash width={12} height={12} />
                </Button>
              </div>
            )}
          </div>

          {(folder.tags ?? []).length > 0 && (
            <div style={{ margin: "10px 0", display: "flex", gap: 4, flexWrap: "wrap" }}>
              {folder.tags!.map((t) => (
                <Badge key={t}>{t}</Badge>
              ))}
            </div>
          )}

          <h4>README</h4>
          <pre className="folder-readme">{folder.readme || "(no README yet)"}</pre>

          <h4>Metadata</h4>
          <pre className="folder-readme">{JSON.stringify(folder.metadata ?? {}, null, 2)}</pre>
        </>
      ) : (
        <>
          <Field htmlFor="fd-name" label="Name">
            <Input id="fd-name" value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field htmlFor="fd-desc" label="Description">
            <Input id="fd-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
          </Field>
          <Field htmlFor="fd-tags" label="Tags" hint="Comma-separated.">
            <Input id="fd-tags" value={tagsText} onChange={(e) => setTagsText(e.target.value)} />
          </Field>
          <Field htmlFor="fd-readme" label="README" hint="Plain text/markdown source, rendered as-is.">
            <Textarea id="fd-readme" rows={8} value={readme} onChange={(e) => setReadme(e.target.value)} />
          </Field>
          <Field htmlFor="fd-metadata" label="Metadata (JSON)">
            <Textarea id="fd-metadata" rows={6} value={metadataText} onChange={(e) => setMetadataText(e.target.value)} />
          </Field>
          <div style={{ display: "flex", gap: 8 }}>
            <Button variant="primary" disabled={saving} onClick={handleSave}>
              {saving ? "Saving…" : "Save"}
            </Button>
            <Button onClick={() => setEditing(false)}>Cancel</Button>
          </div>
        </>
      )}

      <FolderAccessSection folder={folder} canManage={canWrite} />
    </div>
  );
}

function FolderAccessSection({ folder, canManage }: { folder: Folder; canManage: boolean }) {
  const queryClient = useQueryClient();
  const hasScopedPermission = useAuthStore((s) => s.hasScopedPermission);
  const canManageAccess = hasScopedPermission(PERMISSIONS.foldersManageAccess, [folder.id, ...folder.ancestorIds]);

  const { data: bindings = [] } = useQuery({
    queryKey: ["folder-access", folder.id],
    queryFn: () => listFolderAccess(folder.id),
    enabled: canManageAccess,
  });
  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: listUsers, enabled: canManageAccess });
  const { data: roles = [] } = useQuery({ queryKey: ["roles"], queryFn: listRoles, enabled: canManageAccess });

  const [granting, setGranting] = useState(false);
  const [userId, setUserId] = useState("");
  const [roleId, setRoleId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["folder-access", folder.id] });

  if (!canManageAccess) return null;

  return (
    <div style={{ marginTop: 20, borderTop: "1px solid var(--border)", paddingTop: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h4 style={{ margin: 0 }}>Access</h4>
        {canManage && (
          <Button size="sm" onClick={() => setGranting((v) => !v)}>
            Grant access
          </Button>
        )}
      </div>

      {error && <div className="error-banner">{error}</div>}

      {granting && (
        <div style={{ display: "flex", gap: 8, alignItems: "flex-end", marginTop: 8 }}>
          <Field htmlFor="grant-user" label="User" style={{ margin: 0, flex: 1 }}>
            <Select id="grant-user" value={userId} onChange={(e) => setUserId(e.target.value)}>
              <option value="">Select a user…</option>
              {users.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.displayName} ({u.email})
                </option>
              ))}
            </Select>
          </Field>
          <Field htmlFor="grant-role" label="Role" style={{ margin: 0, flex: 1 }}>
            <Select id="grant-role" value={roleId} onChange={(e) => setRoleId(e.target.value)}>
              <option value="">Select a role…</option>
              {roles.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </Select>
          </Field>
          <Button
            variant="primary"
            size="sm"
            disabled={!userId || !roleId}
            onClick={async () => {
              setError(null);
              try {
                await grantFolderAccess(folder.id, userId, roleId);
                setUserId("");
                setRoleId("");
                setGranting(false);
                invalidate();
              } catch (err) {
                setError(extractErrorMessage(err));
              }
            }}
          >
            Grant
          </Button>
        </div>
      )}

      <table className="data-table" style={{ marginTop: 8 }}>
        <thead>
          <tr>
            <th>User</th>
            <th>Role</th>
            {canManage && <th style={{ width: 60 }} />}
          </tr>
        </thead>
        <tbody>
          {bindings.length === 0 && (
            <tr>
              <td colSpan={3} className="empty-state">
                No folder-specific access grants - only account-wide roles apply here.
              </td>
            </tr>
          )}
          {bindings.map((b) => (
            <tr key={b.id}>
              <td>{b.userEmail}</td>
              <td>
                <Badge>{b.roleName}</Badge>
              </td>
              {canManage && (
                <td>
                  <Button
                    size="sm"
                    onClick={async () => {
                      await revokeFolderAccess(folder.id, b.id);
                      invalidate();
                    }}
                  >
                    Revoke
                  </Button>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
