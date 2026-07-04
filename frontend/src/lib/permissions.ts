// Mirrors internal/rbac permission codes on the backend. Keep in sync.
export const PERMISSIONS = {
  usersRead: "users:read",
  usersWrite: "users:write",
  rolesRead: "roles:read",
  rolesWrite: "roles:write",
  connectionsRead: "connections:read",
  connectionsWrite: "connections:write",
  connectionsTest: "connections:test",
  workflowsRead: "workflows:read",
  workflowsWrite: "workflows:write",
  workflowsExecute: "workflows:execute",
  auditRead: "audit:read",
  foldersRead: "folders:read",
  foldersWrite: "folders:write",
  foldersManageAccess: "folders:manage_access",
} as const;
