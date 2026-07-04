import { api } from "./client";
import type { Folder, FolderRoleBinding } from "./types";

export interface FolderListParams {
  tag?: string;
  q?: string;
}

export async function listFolders(params: FolderListParams = {}): Promise<Folder[]> {
  const res = await api.get<Folder[]>("/folders/", { params });
  return res.data ?? [];
}

export async function getFolder(id: string): Promise<Folder> {
  const res = await api.get<Folder>(`/folders/${id}`);
  return res.data;
}

export interface FolderInput {
  name: string;
  description: string;
  parentId?: string;
  tags?: string[];
  readme?: string;
  metadata?: Record<string, unknown>;
}

export async function createFolder(input: FolderInput): Promise<Folder> {
  const res = await api.post<Folder>("/folders/", input);
  return res.data;
}

export async function updateFolder(id: string, input: FolderInput): Promise<Folder> {
  const res = await api.put<Folder>(`/folders/${id}`, input);
  return res.data;
}

export async function moveFolder(id: string, parentId: string | null): Promise<Folder> {
  const res = await api.post<Folder>(`/folders/${id}/move`, { parentId });
  return res.data;
}

export async function deleteFolder(id: string): Promise<void> {
  await api.delete(`/folders/${id}`);
}

export async function listFolderAccess(id: string): Promise<FolderRoleBinding[]> {
  const res = await api.get<FolderRoleBinding[]>(`/folders/${id}/access`);
  return res.data ?? [];
}

export async function grantFolderAccess(id: string, userId: string, roleId: string): Promise<FolderRoleBinding> {
  const res = await api.post<FolderRoleBinding>(`/folders/${id}/access`, { userId, roleId });
  return res.data;
}

export async function revokeFolderAccess(id: string, bindingId: string): Promise<void> {
  await api.delete(`/folders/${id}/access/${bindingId}`);
}
