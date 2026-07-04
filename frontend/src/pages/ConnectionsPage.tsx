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
import { extractErrorMessage } from "../api/client";
import type { CatalogEntry, Connection } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { IconPlay, IconPlug, IconPlus, IconRefresh, IconTrash } from "../components/icons";
import { ConnectionFormModal } from "./connections/ConnectionFormModal";
import { ConnectionQueryModal } from "./connections/ConnectionQueryModal";
import { CatalogBrowserModal } from "./connections/CatalogBrowserModal";
import { Button, IconButton } from "../components/ui";

export function ConnectionsPage() {
  const queryClient = useQueryClient();
  const { data: connections = [], isLoading, error } = useQuery({ queryKey: ["connections"], queryFn: listConnections });

  const [formTarget, setFormTarget] = useState<Connection | "new" | null>(null);
  const [catalogPrefill, setCatalogPrefill] = useState<CatalogEntry | null>(null);
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [queryTarget, setQueryTarget] = useState<Connection | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);

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
        <PermissionGate permission={PERMISSIONS.connectionsWrite}>
          <div style={{ display: "flex", gap: 8 }}>
            <Button onClick={() => setCatalogOpen(true)}>
              <IconPlug width={14} height={14} /> Browse catalog
            </Button>
            <Button variant="primary" onClick={() => setFormTarget("new")}>
              <IconPlus width={14} height={14} /> New connection
            </Button>
          </div>
        </PermissionGate>
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Status</th>
              <th>Last tested</th>
              <th style={{ width: 160 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={5}>Loading…</td>
              </tr>
            )}
            {!isLoading && connections.length === 0 && (
              <tr>
                <td colSpan={5} className="empty-state">
                  No connections yet.
                </td>
              </tr>
            )}
            {connections.map((c) => (
              <tr key={c.id}>
                <td>
                  <strong>{c.name}</strong>
                  {c.description && <div style={{ color: "var(--text-tertiary)", fontSize: 11.5 }}>{c.description}</div>}
                </td>
                <td>{c.type}</td>
                <td>
                  <StatusBadge status={c.status} />
                  {c.lastError && <div style={{ color: "var(--danger)", fontSize: 11 }}>{c.lastError}</div>}
                </td>
                <td>{c.lastTestedAt ? new Date(c.lastTestedAt).toLocaleString() : "never"}</td>
                <td>
                  <div style={{ display: "flex", gap: 4 }}>
                    <PermissionGate permission={PERMISSIONS.connectionsTest}>
                      <IconButton label="Test connection" onClick={() => handleTest(c.id)} disabled={testingId === c.id}>
                        <IconRefresh width={14} height={14} />
                      </IconButton>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.connectionsRead}>
                      <IconButton label="Run query" onClick={() => setQueryTarget(c)}>
                        <IconPlay width={14} height={14} />
                      </IconButton>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.connectionsWrite}>
                      <IconButton label="Edit" onClick={() => setFormTarget(c)}>
                        <IconPlug width={14} height={14} />
                      </IconButton>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.connectionsWrite}>
                      <IconButton label="Delete" onClick={() => handleDelete(c)}>
                        <IconTrash width={14} height={14} />
                      </IconButton>
                    </PermissionGate>
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
    </div>
  );
}
