-- Folder framework: a nested hierarchy that every stored entity (connections,
-- workflows, ...) will live in (see 0006), plus the "scopable" flag that lets
-- role-permissions be granted either globally (as today) or scoped to a
-- folder subtree (see 0007).
--
-- Hierarchy representation: parent_id is the source of truth; ancestor_ids
-- materializes the full (self-exclusive) ancestor chain so "is folder A
-- inside folder B's subtree" and "list everything under B" are simple
-- GIN-indexed array lookups instead of a recursive query on every read.
-- depth is derived from ancestor_ids (not stored independently) so the two
-- can never drift out of sync.
CREATE TABLE folders (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    parent_id    UUID REFERENCES folders(id) ON DELETE RESTRICT,
    -- TEXT[] rather than UUID[]: Postgres doesn't (and can't) enforce a FK on
    -- array elements either way, so there's no integrity trade-off, and it
    -- sidesteps any uuid[]<->[]string driver-codec ambiguity - a plain list
    -- of id strings, exactly like tags below.
    ancestor_ids TEXT[] NOT NULL DEFAULT '{}',
    depth        INT GENERATED ALWAYS AS (COALESCE(array_length(ancestor_ids, 1), 0)) STORED,
    tags         TEXT[] NOT NULL DEFAULT '{}',
    readme       TEXT NOT NULL DEFAULT '',
    metadata     JSONB NOT NULL DEFAULT '{}',
    -- Plain TEXT rather than a FK, mirroring workflow_executions.triggered_by
    -- (0003) - migrations can seed a folder (see 0006's "General") before any
    -- user exists to attribute it to.
    created_by   TEXT NOT NULL DEFAULT 'system',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sibling names must be unique, but the same name may recur in different
-- folders (and at different points in the tree) - two partial indexes since
-- parent_id is nullable (root-level folders) and a single UNIQUE(parent_id,
-- name) would treat every NULL as distinct, defeating root-level uniqueness.
CREATE UNIQUE INDEX uq_folders_root_name ON folders (name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX uq_folders_child_name ON folders (parent_id, name) WHERE parent_id IS NOT NULL;

CREATE INDEX idx_folders_parent_id ON folders (parent_id);
CREATE INDEX idx_folders_ancestor_ids ON folders USING GIN (ancestor_ids);
CREATE INDEX idx_folders_tags ON folders USING GIN (tags);

-- scopable marks the permissions that make sense to grant on a folder
-- subtree (connections:*, workflows:*, folders:*) as opposed to
-- account-wide-only permissions (users:*, roles:*, audit:read). This is
-- what stops "grant the admin role scoped to folder X" from silently also
-- granting users:write folder-wide - the join that resolves a folder-scoped
-- grant (see 0007) always filters on scopable, regardless of which role is
-- bound.
ALTER TABLE permissions ADD COLUMN scopable BOOLEAN NOT NULL DEFAULT false;

UPDATE permissions SET scopable = true
WHERE code IN ('connections:read', 'connections:write', 'connections:test',
               'workflows:read', 'workflows:write', 'workflows:execute');

INSERT INTO permissions (code, description, scopable) VALUES
    ('folders:read',          'View folders and their tags/readme/metadata/contents', true),
    ('folders:write',         'Create, rename, tag, move, and delete folders', true),
    ('folders:manage_access', 'Grant or revoke folder-scoped role bindings', true);

-- admin/editor can browse and manage folder structure; only admin can
-- delegate access (grant/revoke folder-scoped bindings) by default - though
-- since folders:manage_access is itself scopable, an admin can later grant a
-- non-admin "manage access" rights within a specific folder subtree.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('admin', 'editor') AND p.code IN ('folders:read', 'folders:write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'admin' AND p.code = 'folders:manage_access';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'viewer' AND p.code = 'folders:read';
