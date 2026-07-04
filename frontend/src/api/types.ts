export type UserStatus = "active" | "suspended";

// Mirrors backend/internal/domain.Permission. Scopable marks permissions
// that can be granted on a folder subtree (connections:*, workflows:*,
// folders:*) via a folder-scoped role binding, rather than only
// account-wide (users:*, roles:*, audit:read).
export interface Permission {
  id: string;
  code: string;
  description: string;
  scopable: boolean;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
  permissions?: Permission[];
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

export type ConnectionType = "postgres" | "mysql" | "rest" | "graphql" | "aws" | "gcp" | "azure";
export type ConnectionStatus = "unverified" | "healthy" | "unhealthy";

export type AWSService = "athena" | "cloudwatchLogs" | "dynamodb" | "s3";
export type GCPService = "bigquery" | "gcs";
export type AzureService = "logAnalytics" | "blobStorage";

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

// ErrorCode mirrors backend/internal/connections.ErrorCode - the fixed
// vocabulary every connector error is classified into (see Classify), so the
// UI can key a badge/icon off a stable value instead of pattern-matching a
// raw driver message.
export type ErrorCode =
  | "timeout"
  | "network_unreachable"
  | "auth_failed"
  | "permission_denied"
  | "not_found"
  | "rate_limited"
  | "invalid_config"
  | "unknown";

export interface Connection {
  id: string;
  name: string;
  type: ConnectionType;
  description: string;
  config: Record<string, unknown>;
  status: ConnectionStatus;
  folderId: string;
  lastTestedAt?: string;
  lastError?: string;
  lastErrorCode?: ErrorCode;
  lastErrorRemediation?: string;
  lastCheckDurationMs?: number;
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

// Mirrors backend/internal/connections.CloudQuerySpec.
export interface CloudQuerySpec {
  query?: string;

  logGroupNames?: string[];
  startTime?: string;
  endTime?: string;

  tableName?: string;
  indexName?: string;
  scan?: boolean;
  keyConditionExpression?: string;
  filterExpression?: string;
  expressionAttributeNames?: Record<string, string>;
  expressionAttributeValues?: Record<string, unknown>;

  bucket?: string;
  key?: string;
  prefix?: string;
  format?: "csv" | "json" | "ndjson";
  delimiter?: string;
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
  cloud?: CloudQuerySpec;
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
  folderId: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  scheduleCron?: string;
  scheduleEnabled: boolean;
  scheduleNextRun?: string;
  scheduleLastRun?: string;
}

export type ExecutionStatus = "running" | "succeeded" | "failed" | "skipped";

export interface NodeExecutionResult {
  nodeId: string;
  nodeType: string;
  nodeName: string;
  rowsOut: number;
  rowCap?: number;
  truncated?: boolean;
  durationMs: number;
  timeoutMs?: number;
  warnings?: string[];
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
  error: { code: string; message: string; remediation?: string; detail?: string };
}

// Mirrors backend/internal/domain.Folder. Every connection/workflow lives
// in exactly one folder; folders nest, and ancestorIds is the materialized
// (self-exclusive) chain from the root down to this folder - what the
// frontend uses to build breadcrumbs without a second request.
export interface Folder {
  id: string;
  name: string;
  description: string;
  parentId?: string;
  ancestorIds: string[];
  depth: number;
  tags?: string[];
  readme?: string;
  metadata?: Record<string, unknown>;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

// Mirrors backend/internal/domain.FolderRoleBinding - grants a user a
// role's (scopable) permissions within one folder and its descendants.
export interface FolderRoleBinding {
  id: string;
  folderId: string;
  userId: string;
  userEmail?: string;
  roleId: string;
  roleName?: string;
  createdBy: string;
  createdAt: string;
}

// Mirrors backend/internal/catalog.Entry - a static, first-party list of
// well-known integrations used purely to prefill a rest/graphql connection
// form. authConfig carries only non-secret fields (matching AuthConfig
// above); the form's secret fields are always left blank regardless.
export interface CatalogEntry {
  id: string;
  name: string;
  description: string;
  category: string;
  type: "rest" | "graphql";
  baseUrl?: string;
  endpoint?: string;
  authType: AuthType;
  authConfig?: Record<string, unknown>;
  docsUrl?: string;
}
