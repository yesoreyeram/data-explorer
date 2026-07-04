// Package domain contains the core entities shared across the application.
// It intentionally has no dependency on transport (HTTP) or storage (SQL)
// concerns so it can be imported by any layer without creating cycles.
package domain

import (
	"encoding/json"
	"time"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName"`
	PasswordHash string     `json:"-"`
	Status       UserStatus `json:"status"`
	Roles        []Role     `json:"roles,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"isSystem"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Permission struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	// Scopable marks permissions that can be granted on a folder subtree
	// (connections:*, workflows:*, folders:*) via a FolderRoleBinding,
	// rather than only account-wide (users:*, roles:*, audit:read). It's
	// what keeps binding e.g. the admin role to a single folder from
	// silently granting users:write folder-wide - resolving a folder-scoped
	// grant always filters on Scopable, regardless of which role is bound.
	Scopable bool `json:"scopable"`
}

// Folder is a node in the nested folder hierarchy every stored entity
// (connections, workflows, ...) lives in. ParentID nil means a root-level
// folder. AncestorIDs is the materialized, self-exclusive chain of every
// ancestor from the root down to (but not including) this folder - it's
// what lets "is this folder inside that subtree" and "list everything under
// this folder" be simple indexed array lookups instead of a recursive query.
// Depth is derived from AncestorIDs, not stored independently.
type Folder struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ParentID    *string         `json:"parentId,omitempty"`
	AncestorIDs []string        `json:"ancestorIds"`
	Depth       int             `json:"depth"`
	Tags        []string        `json:"tags,omitempty"`
	Readme      string          `json:"readme,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedBy   string          `json:"createdBy"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// FolderRoleBinding grants a user a role's (scopable) permissions, but only
// within one folder and its descendants - the namespace-scoped counterpart
// to the account-wide user_roles assignment.
type FolderRoleBinding struct {
	ID        string    `json:"id"`
	FolderID  string    `json:"folderId"`
	UserID    string    `json:"userId"`
	UserEmail string    `json:"userEmail,omitempty"`
	RoleID    string    `json:"roleId"`
	RoleName  string    `json:"roleName,omitempty"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

type ConnectionType string

const (
	ConnectionTypePostgres ConnectionType = "postgres"
	ConnectionTypeMySQL    ConnectionType = "mysql"
	ConnectionTypeREST     ConnectionType = "rest"
	ConnectionTypeGraphQL  ConnectionType = "graphql"
	ConnectionTypeAWS      ConnectionType = "aws"
	ConnectionTypeGCP      ConnectionType = "gcp"
	ConnectionTypeAzure    ConnectionType = "azure"
)

type ConnectionStatus string

const (
	ConnectionStatusUnverified ConnectionStatus = "unverified"
	ConnectionStatusHealthy    ConnectionStatus = "healthy"
	ConnectionStatusUnhealthy  ConnectionStatus = "unhealthy"
)

// Connection represents a stored, reusable link to an external data source.
// Non-secret configuration lives in Config; anything sensitive (passwords,
// API keys, tokens) is stored encrypted-at-rest and never returned by the API.
type Connection struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Type        ConnectionType   `json:"type"`
	Description string           `json:"description"`
	Config      json.RawMessage  `json:"config"`
	Status      ConnectionStatus `json:"status"`
	// FolderID is the folder this connection lives in - every connection
	// must belong to exactly one (see internal/folders).
	FolderID     string     `json:"folderId"`
	LastTestedAt *time.Time `json:"lastTestedAt,omitempty"`
	LastError    string     `json:"lastError,omitempty"`

	// LastErrorCode/LastErrorRemediation are the structured counterpart to
	// LastError (see connections.HealthError/Classify) - a stable code the
	// UI can key a badge off of, and a concrete next step, instead of just
	// a driver's raw message. Empty when the last check succeeded (or none
	// has run yet).
	LastErrorCode        string `json:"lastErrorCode,omitempty"`
	LastErrorRemediation string `json:"lastErrorRemediation,omitempty"`
	LastCheckDurationMs  int64  `json:"lastCheckDurationMs,omitempty"`

	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ConnectionSecret is the sensitive payload for a connection. It is encrypted
// with AES-256-GCM before it touches the database and is decrypted only
// in-memory, only when a connector needs to dial out.
type ConnectionSecret struct {
	// Generic key/value bag - e.g. {"password": "..."} or {"apiKey": "..."} or {"bearerToken": "..."}
	Values map[string]string `json:"values"`
}

type WorkflowStatus string

const (
	WorkflowStatusDraft     WorkflowStatus = "draft"
	WorkflowStatusPublished WorkflowStatus = "published"
)

type Workflow struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"` // WorkflowDefinition JSON (nodes + edges)
	Status      WorkflowStatus  `json:"status"`
	Version     int             `json:"version"`
	// FolderID is the folder this workflow lives in - every workflow must
	// belong to exactly one (see internal/folders).
	FolderID  string    `json:"folderId"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Schedule: a standard 5-field cron expression (minute hour dom month
	// dow) evaluated by the scheduler - see internal/scheduler. Empty
	// ScheduleCron/false ScheduleEnabled means the workflow only ever runs
	// on manual/API trigger.
	ScheduleCron    string     `json:"scheduleCron,omitempty"`
	ScheduleEnabled bool       `json:"scheduleEnabled"`
	ScheduleNextRun *time.Time `json:"scheduleNextRun,omitempty"`
	ScheduleLastRun *time.Time `json:"scheduleLastRun,omitempty"`
}

type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusSkipped   ExecutionStatus = "skipped"
)

type WorkflowExecution struct {
	ID          string          `json:"id"`
	WorkflowID  string          `json:"workflowId"`
	Status      ExecutionStatus `json:"status"`
	TriggeredBy string          `json:"triggeredBy"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	DurationMs  int64           `json:"durationMs"`
	Error       string          `json:"error,omitempty"`
	NodeResults json.RawMessage `json:"nodeResults,omitempty"` // per-node summary: rows in/out, duration, errors
}

type AuditLog struct {
	ID           string          `json:"id"`
	ActorID      string          `json:"actorId"`
	ActorEmail   string          `json:"actorEmail"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	IPAddress    string          `json:"ipAddress"`
	UserAgent    string          `json:"userAgent"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Outcome      string          `json:"outcome"` // success | failure
	CreatedAt    time.Time       `json:"createdAt"`
}
