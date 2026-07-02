import { api } from "./client";
import type { AuditLog } from "./types";

export interface AuditLogFilter {
  actorId?: string;
  action?: string;
  resourceType?: string;
  since?: string;
  until?: string;
  limit?: number;
  offset?: number;
}

export interface AuditLogPage {
  items: AuditLog[];
  total: number;
}

export async function listAuditLogs(filter: AuditLogFilter = {}): Promise<AuditLogPage> {
  const res = await api.get<AuditLogPage>("/audit-logs/", { params: filter });
  return res.data ?? { items: [], total: 0 };
}
