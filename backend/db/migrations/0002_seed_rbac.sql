-- Seed the fixed permission set and three built-in system roles:
--   admin  - full control, including users/roles and audit log
--   editor - can manage connections/workflows, execute workflows, and read users/roles
--   viewer - read-only access to connections, workflows, users, and roles

INSERT INTO permissions (code, description) VALUES
    ('users:read',        'View users'),
    ('users:write',       'Create, update, suspend users'),
    ('roles:read',        'View roles and permissions'),
    ('roles:write',       'Assign roles to users'),
    ('connections:read',  'View data source connections'),
    ('connections:write', 'Create, update, delete connections'),
    ('connections:test',  'Test-connect to a data source'),
    ('workflows:read',    'View workflows'),
    ('workflows:write',   'Create, update, delete workflows'),
    ('workflows:execute', 'Run workflows'),
    ('audit:read',        'View audit logs');

INSERT INTO roles (name, description, is_system) VALUES
    ('admin',  'Full administrative access', true),
    ('editor', 'Can manage and run connections and workflows', true),
    ('viewer', 'Read-only access, can execute existing workflows', true);

-- admin: all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin';

-- editor: everything except user/role administration
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'editor'
  AND p.code IN ('connections:read','connections:write','connections:test',
                 'workflows:read','workflows:write','workflows:execute',
                 'roles:read','users:read');

-- viewer: read-only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'viewer'
  AND p.code IN ('connections:read','workflows:read','roles:read','users:read');
