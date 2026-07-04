import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createConnection,
  deleteConnection,
  listConnections,
  testConnection,
  updateConnection,
  type ConnectionInput,
} from "../api/connections";
import { listFolders } from "../api/folders";
import { extractErrorMessage } from "../api/client";
import type { CatalogEntry, Connection } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";
import { FolderSelect } from "../components/FolderSelect";
import { PERMISSIONS } from "../lib/permissions";
import { useAuthStore } from "../state/authStore";
import { IconActivity, IconPlay, IconPlug, IconPlus, IconRefresh, IconTrash } from "../components/icons";
import { ConnectionFormModal } from "./connections/ConnectionFormModal";
import { ConnectionQueryModal } from "./connections/ConnectionQueryModal";
import { ConnectionHealthModal } from "./connections/ConnectionHealthModal";
import { CatalogBrowserModal } from "./connections/CatalogBrowserModal";
import { Button, Field, IconButton } from "../components/ui";

export function ConnectionsPage() {
  const queryClient = useQueryClient();
  const { data: connections = [], isLoading, error } = useQuery({ queryKey: ["connections"], queryFn: listConnections });
  const { data: folders = [] } = useQuery({ queryKey: ["folders"], queryFn: () => listFolders() });
  const hasScopedPermission = useAuthStore((s) => s.hasScopedPermission);
  const hasAnyScopedPermission = useAuthStore((s) => s.hasAnyScopedPermission);

  const [formTarget, setFormTarget] = useState<Connection | "new" | null>(null);
  const [catalogPrefill, setCatalogPrefill] = useState<CatalogEntry | null>(null);
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [queryTarget, setQueryTarget] = useState<Connection | null>(null);
  const [healthTarget, setHealthTarget] = useState<Connection | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [folderFilter, setFolderFilter] = useState("");

  const visibleConnections = folderFilter ? connections.filter((c) => c.folderId === folderFilter) : connections;
  const folderName = (id: string) => folders.find((f) => f.id === id)?.name ?? "";
  // A connection's write/test/read access can be scoped to its folder (see
  // rbac.Principal.HasScoped on the backend) - resolve the folder's scope
  // chain (itself + ancestors) from the already-fetched folders list so
  // each row's action buttons reflect the same check the API enforces,
  // rather than only the account-wide PERMISSIONS flat list.
  const scopeChainFor = (folderId: string): string[] => {
    const folder = folders.find((f) => f.id === folderId);
    return folder ? [folder.id, ...folder.ancestorIds] : [folderId];
  };

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["connections"] });

  const createMutation = useMutation({
    mutationFn: (input: ConnectionInput) => createConnection(input),
    onSuccess: invalidate,
  });
  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input: ConnectionInput }) => updateConnection(id, input),
    onSuccess: invalidate,
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteConnection(id),
    onSuccess: invalidate,
  });

  async function handleTest(id: string) {
    setTestingId(id);
    try {
      await testConnection(id);
    } finally {
      setTestingId(null);
      invalidate();
    }
  }

  async function handleDelete(conn: Connection) {
    if (!confirm(`Delete connection "${conn.name}"? This cannot be undone.`)) return;
    await deleteMutation.mutateAsync(conn.id);
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Connections</h1>
          <p className="panel-subtitle">Reusable, credentialed links to your databases and APIs.</p>
        </div>
        {hasAnyScopedPermission(PERMISSIONS.connectionsWrite) && (
          <div style={{ display: "flex", gap: 8 }}>
            <Button onClick={() => setCatalogOpen(true)}>
              <IconPlug width={14} height={14} /> Browse catalog
            </Button>
            <Button variant="primary" onClick={() => setFormTarget("new")}>
              <IconPlus width={14} height={14} /> New connection
            </Button>
          </div>
        )}
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <Field htmlFor="conn-folder-filter" label="Filter by folder" style={{ maxWidth: 260, marginBottom: 12 }}>
        <FolderSelect id="conn-folder-filter" folders={folders} value={folderFilter} onChange={setFolderFilter} placeholder="All folders" />
      </Field>

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Folder</th>
              <th>Status</th>
              <th>Last tested</th>
              <th style={{ width: 160 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={6}>Loading…</td>
              </tr>
            )}
            {!isLoading && visibleConnections.length === 0 && (
              <tr>
                <td colSpan={6} className="empty-state">
                  No connections yet.
                </td>
              </tr>
            )}
            {visibleConnections.map((c) => (
              <tr key={c.id}>
                <td>
                  <strong>{c.name}</strong>
                  {c.description && <div style={{ color: "var(--text-tertiary)", fontSize: 11.5 }}>{c.description}</div>}
                </td>
                <td>{c.type}</td>
                <td>{folderName(c.folderId)}</td>
                <td>
                  <button
                    style={{ display: "flex", flexDirection: "column", alignItems: "flex-start", gap: 2, background: "none", border: 0, padding: 0, cursor: "pointer" }}
                    onClick={() => setHealthTarget(c)}
                    title="View health details"
                  >
                    <StatusBadge status={c.status} />
                    {c.lastError && <div style={{ color: "var(--danger)", fontSize: 11 }}>{c.lastError}</div>}
                  </button>
                </td>
                <td>{c.lastTestedAt ? new Date(c.lastTestedAt).toLocaleString() : "never"}</td>
                <td>
                  <div style={{ display: "flex", gap: 4 }}>
                    {hasScopedPermission(PERMISSIONS.connectionsTest, scopeChainFor(c.folderId)) && (
                      <IconButton label="Test connection" onClick={() => handleTest(c.id)} disabled={testingId === c.id}>
                        <IconRefresh width={14} height={14} />
                      </IconButton>
                    )}
                    {hasScopedPermission(PERMISSIONS.connectionsTest, scopeChainFor(c.folderId)) && (
                      <IconButton label="View health" onClick={() => setHealthTarget(c)}>
                        <IconActivity width={14} height={14} />
                      </IconButton>
                    )}
                    {hasScopedPermission(PERMISSIONS.connectionsRead, scopeChainFor(c.folderId)) && (
                      <IconButton label="Run query" onClick={() => setQueryTarget(c)}>
                        <IconPlay width={14} height={14} />
                      </IconButton>
                    )}
                    {hasScopedPermission(PERMISSIONS.connectionsWrite, scopeChainFor(c.folderId)) && (
                      <IconButton label="Edit" onClick={() => setFormTarget(c)}>
                        <IconPlug width={14} height={14} />
                      </IconButton>
                    )}
                    {hasScopedPermission(PERMISSIONS.connectionsWrite, scopeChainFor(c.folderId)) && (
                      <IconButton label="Delete" onClick={() => handleDelete(c)}>
                        <IconTrash width={14} height={14} />
                      </IconButton>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {formTarget && (
        <ConnectionFormModal
          connection={formTarget === "new" ? undefined : formTarget}
          catalogEntry={formTarget === "new" ? (catalogPrefill ?? undefined) : undefined}
          onClose={() => {
            setFormTarget(null);
            setCatalogPrefill(null);
          }}
          onSubmit={async (input) => {
            if (formTarget === "new") {
              await createMutation.mutateAsync(input);
            } else {
              await updateMutation.mutateAsync({ id: formTarget.id, input });
            }
          }}
        />
      )}

      {catalogOpen && (
        <CatalogBrowserModal
          onClose={() => setCatalogOpen(false)}
          onSelect={(entry) => {
            setCatalogPrefill(entry);
            setCatalogOpen(false);
            setFormTarget("new");
          }}
        />
      )}

      {queryTarget && <ConnectionQueryModal connection={queryTarget} onClose={() => setQueryTarget(null)} />}

      {healthTarget && <ConnectionHealthModal connection={healthTarget} onClose={() => setHealthTarget(null)} />}
    </div>
  );
}
