import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { listRoles, listUsers, setUserRoles, setUserStatus } from "../api/users";
import { extractErrorMessage } from "../api/client";
import { useAuthStore } from "../state/authStore";
import { StatusBadge } from "../components/StatusBadge";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { Modal } from "../components/Modal";
import type { User } from "../api/types";

export function UsersPage() {
  const currentUser = useAuthStore((s) => s.user);
  const queryClient = useQueryClient();
  const { data: users = [], isLoading, error } = useQuery({ queryKey: ["users"], queryFn: listUsers });
  const { data: roles = [] } = useQuery({ queryKey: ["roles"], queryFn: listRoles });

  const [roleEditTarget, setRoleEditTarget] = useState<User | null>(null);

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: "active" | "suspended" }) => setUserStatus(id, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });
  const rolesMutation = useMutation({
    mutationFn: ({ id, roleIds }: { id: string; roleIds: string[] }) => setUserRoles(id, roleIds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Users & roles</h1>
          <p className="panel-subtitle">Manage who has access and what they&rsquo;re allowed to do.</p>
        </div>
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Roles</th>
              <th>Status</th>
              <th>Joined</th>
              <th style={{ width: 160 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={6}>Loading…</td>
              </tr>
            )}
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.displayName}</td>
                <td>{u.email}</td>
                <td>
                  {(u.roles ?? []).map((r) => (
                    <span key={r.id || r.name} className="badge badge-neutral" style={{ marginRight: 4 }}>
                      {r.name}
                    </span>
                  ))}
                </td>
                <td>
                  <StatusBadge status={u.status} />
                </td>
                <td>{new Date(u.createdAt).toLocaleDateString()}</td>
                <td>
                  <PermissionGate permission={PERMISSIONS.rolesWrite}>
                    <div style={{ display: "flex", gap: 6 }}>
                      <button className="btn btn-sm" type="button" onClick={() => setRoleEditTarget(u)}>
                        Roles
                      </button>
                      <PermissionGate permission={PERMISSIONS.usersWrite}>
                        <button
                          className="btn btn-sm"
                          type="button"
                          disabled={u.id === currentUser?.id}
                          onClick={() =>
                            statusMutation.mutate({ id: u.id, status: u.status === "active" ? "suspended" : "active" })
                          }
                        >
                          {u.status === "active" ? "Suspend" : "Reactivate"}
                        </button>
                      </PermissionGate>
                    </div>
                  </PermissionGate>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {roleEditTarget && (
        <RoleEditorModal
          user={roleEditTarget}
          allRoles={roles}
          onClose={() => setRoleEditTarget(null)}
          onSave={async (roleIds) => {
            await rolesMutation.mutateAsync({ id: roleEditTarget.id, roleIds });
            setRoleEditTarget(null);
          }}
        />
      )}
    </div>
  );
}

function RoleEditorModal({
  user,
  allRoles,
  onClose,
  onSave,
}: {
  user: User;
  allRoles: { id: string; name: string; description: string }[];
  onClose: () => void;
  onSave: (roleIds: string[]) => Promise<void>;
}) {
  const initialIds = new Set((user.roles ?? []).map((r) => r.id).filter(Boolean));
  const [selected, setSelected] = useState<Set<string>>(initialIds);
  const [saving, setSaving] = useState(false);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <Modal
      title={`Roles for ${user.displayName}`}
      onClose={onClose}
      footer={
        <>
          <button className="btn" type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            type="button"
            disabled={saving}
            onClick={async () => {
              setSaving(true);
              await onSave(Array.from(selected));
              setSaving(false);
            }}
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </>
      }
    >
      {allRoles.map((role) => (
        <label key={role.id} className="checkbox-row" style={{ marginBottom: 8 }}>
          <input type="checkbox" checked={selected.has(role.id)} onChange={() => toggle(role.id)} />
          <span>
            <strong>{role.name}</strong>
            <div className="field-hint">{role.description}</div>
          </span>
        </label>
      ))}
    </Modal>
  );
}
