-- Core identity, RBAC, connections, workflows, executions, and audit schema.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE, -- e.g. connections:write
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE, -- SHA-256 of the opaque refresh token; the raw token is never stored
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE connections (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    type             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    config           JSONB NOT NULL DEFAULT '{}',
    secret_encrypted TEXT NOT NULL DEFAULT '', -- AES-256-GCM ciphertext, hex-encoded; empty if no secret
    status           TEXT NOT NULL DEFAULT 'unverified',
    last_tested_at   TIMESTAMPTZ,
    last_error       TEXT NOT NULL DEFAULT '',
    created_by       UUID NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

CREATE TABLE workflows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    definition  JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    status      TEXT NOT NULL DEFAULT 'draft',
    version     INT NOT NULL DEFAULT 1,
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

CREATE TABLE workflow_executions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id  UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'running',
    triggered_by UUID NOT NULL REFERENCES users(id),
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ,
    duration_ms  BIGINT NOT NULL DEFAULT 0,
    error        TEXT NOT NULL DEFAULT '',
    node_results JSONB NOT NULL DEFAULT '[]'
);
CREATE INDEX idx_workflow_executions_workflow_id ON workflow_executions(workflow_id, started_at DESC);

CREATE TABLE audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      TEXT NOT NULL DEFAULT '',
    actor_email   TEXT NOT NULL DEFAULT '',
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL DEFAULT '',
    ip_address    TEXT NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL DEFAULT '',
    metadata      JSONB NOT NULL DEFAULT '{}',
    outcome       TEXT NOT NULL DEFAULT 'success',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
