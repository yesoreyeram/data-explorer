-- Align system role bundles with FR-02 for existing installations.

DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('editor', 'viewer') AND is_system = true);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'editor'
  AND r.is_system = true
  AND p.code IN ('connections:read','connections:write','connections:test',
                 'workflows:read','workflows:write','workflows:execute',
                 'roles:read','users:read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'viewer'
  AND r.is_system = true
  AND p.code IN ('connections:read','workflows:read','roles:read','users:read');
