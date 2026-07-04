import { api } from "./client";
import type { Permission, Role, User, UserStatus } from "./types";

export async function listUsers(): Promise<User[]> {
  const res = await api.get<User[]>("/users/");
  return res.data ?? [];
}

export async function setUserStatus(id: string, status: UserStatus): Promise<void> {
  await api.patch(`/users/${id}/status`, { status });
}

export async function setUserRoles(id: string, roleIds: string[]): Promise<void> {
  await api.put(`/users/${id}/roles`, { roleIds });
}

export async function listRoles(): Promise<Role[]> {
  const res = await api.get<Role[]>("/roles/");
  return res.data ?? [];
}

export async function listPermissions(): Promise<Permission[]> {
  const res = await api.get<Permission[]>("/permissions/");
  return res.data ?? [];
}

export interface RoleInput {
  name: string;
  description: string;
  permissionIds: string[];
}

// createRole/updateRole define custom roles - always non-system, so the
// three seeded roles (admin/editor/viewer) stay a stable baseline that
// can't be altered here.
export async function createRole(input: RoleInput): Promise<Role> {
  const res = await api.post<Role>("/roles/", input);
  return res.data;
}

export async function updateRole(id: string, input: RoleInput): Promise<Role> {
  const res = await api.put<Role>(`/roles/${id}`, input);
  return res.data;
}
