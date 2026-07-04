-- A folder-scoped role binding: "this user holds this role's (scopable)
-- permissions, but only within this folder and its descendants" - the
-- namespace-scoped counterpart to the existing global user_roles table.
-- Reuses the same roles/role_permissions tables as global assignment (a
-- role is just a named bundle of permission codes either way), so the same
-- role catalog (including custom roles, see Role CRUD) works for both.
CREATE TABLE folder_role_bindings (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- CASCADE, not RESTRICT: a binding is metadata about the folder itself
    -- (like its tags), not "content" - deleting an (already-empty, per the
    -- RESTRICT rules on folders.parent_id / connections.folder_id /
    -- workflows.folder_id) folder must always succeed, so its bindings are
    -- removed along with it rather than blocking the delete.
    folder_id  UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id    UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (folder_id, user_id, role_id)
);
CREATE INDEX idx_folder_role_bindings_user_id ON folder_role_bindings (user_id);
CREATE INDEX idx_folder_role_bindings_folder_id ON folder_role_bindings (folder_id);
