import { api } from "./client";
import type { Connection, ConnectionType, DataFrame, ErrorCode, QuerySpec } from "./types";

export interface ConnectionInput {
  name: string;
  type: ConnectionType;
  description: string;
  config: Record<string, unknown>;
  secret?: Record<string, string>;
  folderId: string;
}

export async function listConnections(): Promise<Connection[]> {
  const res = await api.get<Connection[]>("/connections/");
  return res.data ?? [];
}

export async function getConnection(id: string): Promise<Connection> {
  const res = await api.get<Connection>(`/connections/${id}`);
  return res.data;
}

export async function createConnection(input: ConnectionInput): Promise<Connection> {
  const res = await api.post<Connection>("/connections/", input);
  return res.data;
}

export async function updateConnection(id: string, input: ConnectionInput): Promise<Connection> {
  const res = await api.put<Connection>(`/connections/${id}`, input);
  return res.data;
}

export async function deleteConnection(id: string): Promise<void> {
  await api.delete(`/connections/${id}`);
}

export interface TestConnectionResult {
  status: Connection["status"];
  lastTestedAt: string;
  durationMs: number;
  healthy: boolean;
  error?: string;
  errorCode?: ErrorCode;
  errorRemediation?: string;
  errorDetail?: string;
}

export async function testConnection(id: string): Promise<TestConnectionResult> {
  const res = await api.post<TestConnectionResult>(`/connections/${id}/test`);
  return res.data;
}

export async function queryConnection(id: string, spec: QuerySpec): Promise<DataFrame> {
  const res = await api.post<DataFrame>(`/connections/${id}/query`, spec);
  return res.data;
}
