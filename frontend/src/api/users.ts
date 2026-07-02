import { api } from "./client";
import type { Role, User, UserStatus } from "./types";

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
