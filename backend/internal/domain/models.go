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
}

type ConnectionType string

const (
	ConnectionTypePostgres ConnectionType = "postgres"
	ConnectionTypeMySQL    ConnectionType = "mysql"
	ConnectionTypeREST     ConnectionType = "rest"
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
	LastTestedAt *time.Time      `json:"lastTestedAt,omitempty"`
	LastError   string           `json:"lastError,omitempty"`
	CreatedBy   string           `json:"createdBy"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
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
	CreatedBy   string          `json:"createdBy"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
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
