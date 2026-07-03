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

export type ConnectionType = "postgres" | "mysql" | "rest" | "graphql";
export type ConnectionStatus = "unverified" | "healthy" | "unhealthy";

// Mirrors backend/internal/connections/connectors.AuthConfig. Which fields
// apply depends on authType - see the backend struct's field comments for
// exactly which secret key each type reads.
export type AuthType =
  | "none"
  | "basic"
  | "bearer"
  | "apiKey"
  | "digest"
  | "oauth2ClientCredentials"
  | "oauth2RefreshToken"
  | "jwt"
  | "workloadIdentity"
  | "kerberos";

export interface AuthConfig {
  authType?: AuthType;

  apiKeyHeader?: string;
  apiKeyLocation?: "header" | "query";
  apiKeyParam?: string;

  oauth2TokenUrl?: string;
  oauth2Scopes?: string[];

  jwtAlgorithm?: "HS256" | "RS256";
  jwtClaims?: Record<string, unknown>;
  jwtTtlSeconds?: number;

  workloadIdentityTokenEndpoint?: string;
  workloadIdentityAudience?: string;
  workloadIdentityScope?: string;
  workloadIdentitySubjectTokenPath?: string;
  workloadIdentitySubjectTokenType?: string;

  kerberosRealm?: string;
  kerberosUsername?: string;
  kerberosSpn?: string;
  kerberosKrb5ConfPath?: string;
  kerberosKeytabPath?: string;
}

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

export type PaginationStrategy = "none" | "offset" | "page" | "cursor" | "linkHeader" | "graphqlRelay";

// Mirrors backend/internal/connections.PaginationSpec.
export interface PaginationSpec {
  strategy: PaginationStrategy;
  itemsPath?: string;
  offsetParam?: string;
  limitParam?: string;
  pageParam?: string;
  pageSizeParam?: string;
  cursorParam?: string;
  cursorPath?: string;
  pageSize?: number;
  graphqlCursorVariable?: string;
  graphqlPageSizeVariable?: string;
  maxPages?: number;
}

export interface GraphQLSpec {
  query: string;
  variables?: Record<string, unknown>;
  operationName?: string;
  dataPath?: string;
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
  pagination?: PaginationSpec;
  graphql?: GraphQLSpec;
}

// ---- dataframe wire format (mirrors backend/pkg/dataframe.Frame's JSON) ----

export type DataFrameFieldType = "string" | "int64" | "float64" | "bool" | "time" | "json" | "null";

export interface DataFrameField {
  name: string;
  type: DataFrameFieldType;
  nullable: boolean;
}

export interface DataFrameSchema {
  fields: DataFrameField[];
}

export interface DataFrameMeta {
  name?: string;
  sourceType?: string;
  sourceId?: string;
  lineage?: string[];
  generatedAt: string;
  durationMs: number;
  rowCount: number;
  columnCount: number;
  truncated: boolean;
  warnings?: string[];
  extra?: Record<string, unknown>;
}

export interface DataFrame {
  schema: DataFrameSchema;
  rows: Record<string, unknown>[];
  meta: DataFrameMeta;
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
