export type UserStatus = "active" | "suspended";

export interface Role {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
}

export interface User {
  id: string;
  email: string;
  displayName: string;
  status: UserStatus;
  roles?: Role[];
  createdAt: string;
  updatedAt: string;
}

export type ConnectionType = "postgres" | "mysql" | "rest";
export type ConnectionStatus = "unverified" | "healthy" | "unhealthy";

export interface Connection {
  id: string;
  name: string;
  type: ConnectionType;
  description: string;
  config: Record<string, unknown>;
  status: ConnectionStatus;
  lastTestedAt?: string;
  lastError?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface QuerySpec {
  sql?: string;
  params?: unknown[];
  method?: string;
  path?: string;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  body?: unknown;
  rowLimit?: number;
}

export interface QueryResult {
  columns: string[];
  rows: Record<string, unknown>[];
  rowCount: number;
  truncated: boolean;
}

export type NodeType = "source" | "transform" | "filter" | "join" | "aggregate" | "output";

export interface WorkflowNode {
  id: string;
  type: NodeType;
  name: string;
  config: Record<string, unknown>;
  position?: { x: number; y: number };
}

export interface WorkflowEdge {
  id: string;
  source: string;
  target: string;
  targetHandle?: string;
}

export interface WorkflowDefinition {
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
}

export type WorkflowStatus = "draft" | "published";

export interface Workflow {
  id: string;
  name: string;
  description: string;
  definition: WorkflowDefinition;
  status: WorkflowStatus;
  version: number;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export type ExecutionStatus = "running" | "succeeded" | "failed";

export interface NodeExecutionResult {
  nodeId: string;
  nodeType: string;
  nodeName: string;
  rowsOut: number;
  durationMs: number;
  error?: string;
}

export interface WorkflowExecution {
  id: string;
  workflowId: string;
  status: ExecutionStatus;
  triggeredBy: string;
  startedAt: string;
  finishedAt?: string;
  durationMs: number;
  error?: string;
  nodeResults?: NodeExecutionResult[];
}

export interface AuditLog {
  id: string;
  actorId: string;
  actorEmail: string;
  action: string;
  resourceType: string;
  resourceId: string;
  ipAddress: string;
  userAgent: string;
  metadata?: Record<string, unknown>;
  outcome: "success" | "failure";
  createdAt: string;
}

export interface ApiErrorBody {
  error: { code: string; message: string };
}
