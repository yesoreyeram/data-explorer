package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/rbac"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("already exists")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, email, displayName, passwordHash string, roleNames []string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, $2, $3)
		 RETURNING id, email, display_name, status, created_at, updated_at`,
		email, displayName, passwordHash,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}

	if len(roleNames) > 0 {
		_, err = r.db.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id)
			 SELECT $1, id FROM roles WHERE name = ANY($2)`,
			u.ID, roleNames,
		)
		if err != nil {
			return domain.User{}, fmt.Errorf("assign roles: %w", err)
		}
	}

	return u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, status, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, display_name, status, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT id, email, display_name, status, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) SetUserStatus(ctx context.Context, id string, status domain.UserStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET status = $1, updated_at = now() WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUserRolesAndPermissions resolves the flattened role names and permission
// codes for a user in a single round trip, used both at login and whenever a
// fresh access token needs to be minted.
func (r *Repository) GetUserRolesAndPermissions(ctx context.Context, userID string) (roles []string, permissions []string, err error) {
	rows, err := r.db.Query(ctx, `SELECT name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("query roles: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		roles = append(roles, name)
	}
	rows.Close()

	permRows, err := r.db.Query(ctx,
		`SELECT DISTINCT p.code FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("query permissions: %w", err)
	}
	defer permRows.Close()
	for permRows.Next() {
		var code string
		if err := permRows.Scan(&code); err != nil {
			return nil, nil, err
		}
		permissions = append(permissions, code)
	}
	return roles, permissions, permRows.Err()
}

// GetUserFolderGrants resolves the user's folder-scoped role bindings into
// one rbac.FolderGrant per folder, flattened to permission codes exactly
// like GetUserRolesAndPermissions does for the account-wide set - so it can
// be embedded in the JWT the same way, with zero per-request DB hits. The
// scopable filter is what stops binding e.g. the admin role to a folder
// from granting account-wide-only permissions (users:write, roles:write,
// audit:read) within that folder - see 0006_folders.sql.
func (r *Repository) GetUserFolderGrants(ctx context.Context, userID string) ([]rbac.FolderGrant, error) {
	rows, err := r.db.Query(ctx,
		`SELECT frb.folder_id, p.code
		 FROM folder_role_bindings frb
		 JOIN role_permissions rp ON rp.role_id = frb.role_id
		 JOIN permissions p ON p.id = rp.permission_id
		 WHERE frb.user_id = $1 AND p.scopable
		 ORDER BY frb.folder_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query folder grants: %w", err)
	}
	defer rows.Close()

	byFolder := make(map[string][]string)
	var order []string
	for rows.Next() {
		var folderID, code string
		if err := rows.Scan(&folderID, &code); err != nil {
			return nil, err
		}
		if _, seen := byFolder[folderID]; !seen {
			order = append(order, folderID)
		}
		byFolder[folderID] = append(byFolder[folderID], code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	grants := make([]rbac.FolderGrant, 0, len(order))
	for _, folderID := range order {
		grants = append(grants, rbac.FolderGrant{FolderID: folderID, Permissions: byFolder[folderID]})
	}
	return grants, nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, description, is_system FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var out []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem); err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	permRows, err := r.db.Query(ctx,
		`SELECT rp.role_id, p.id, p.code, p.description, p.scopable
		 FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
		 ORDER BY rp.role_id, p.code`)
	if err != nil {
		return nil, fmt.Errorf("query role permissions: %w", err)
	}
	defer permRows.Close()

	byRole := make(map[string][]domain.Permission)
	for permRows.Next() {
		var roleID string
		var perm domain.Permission
		if err := permRows.Scan(&roleID, &perm.ID, &perm.Code, &perm.Description, &perm.Scopable); err != nil {
			return nil, err
		}
		byRole[roleID] = append(byRole[roleID], perm)
	}
	if err := permRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Permissions = byRole[out[i].ID]
	}
	return out, nil
}

func (r *Repository) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.db.Query(ctx, `SELECT id, code, description, scopable FROM permissions ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()

	var out []domain.Permission
	for rows.Next() {
		var perm domain.Permission
		if err := rows.Scan(&perm.ID, &perm.Code, &perm.Description, &perm.Scopable); err != nil {
			return nil, err
		}
		out = append(out, perm)
	}
	return out, rows.Err()
}

// ErrSystemRole guards the three built-in roles (admin/editor/viewer) from
// having their permission set altered via the API - custom roles can be
// created freely, but the system defaults stay a stable, predictable
// baseline.
var ErrSystemRole = errors.New("cannot modify a system role")

// ErrInvalidPermission means one of the supplied permission ids doesn't
// exist - surfaced via a foreign-key violation on role_permissions.
var ErrInvalidPermission = errors.New("one or more permission ids are invalid")

// CreateRole defines a new custom role (always is_system = false - only the
// three seeded roles are system roles) with an initial permission set.
func (r *Repository) CreateRole(ctx context.Context, name, description string, permissionIDs []string) (domain.Role, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Role{}, err
	}
	defer tx.Rollback(ctx)

	var role domain.Role
	err = tx.QueryRow(ctx,
		`INSERT INTO roles (name, description, is_system) VALUES ($1, $2, false)
		 RETURNING id, name, description, is_system`,
		name, description,
	).Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Role{}, ErrConflict
		}
		return domain.Role{}, fmt.Errorf("insert role: %w", err)
	}

	for _, permID := range permissionIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, role.ID, permID); err != nil {
			if isForeignKeyViolation(err) {
				return domain.Role{}, ErrInvalidPermission
			}
			return domain.Role{}, fmt.Errorf("assign permission: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Role{}, err
	}
	role.Permissions = nil // caller can re-list if it needs the hydrated set
	return role, nil
}

// UpdateRole replaces a custom role's description and permission set.
// System roles (admin/editor/viewer) reject this with ErrSystemRole.
func (r *Repository) UpdateRole(ctx context.Context, id, description string, permissionIDs []string) (domain.Role, error) {
	var isSystem bool
	err := r.db.QueryRow(ctx, `SELECT is_system FROM roles WHERE id = $1`, id).Scan(&isSystem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Role{}, ErrNotFound
		}
		return domain.Role{}, fmt.Errorf("query role: %w", err)
	}
	if isSystem {
		return domain.Role{}, ErrSystemRole
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Role{}, err
	}
	defer tx.Rollback(ctx)

	var role domain.Role
	err = tx.QueryRow(ctx,
		`UPDATE roles SET description = $1 WHERE id = $2 RETURNING id, name, description, is_system`,
		description, id,
	).Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem)
	if err != nil {
		return domain.Role{}, fmt.Errorf("update role: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
		return domain.Role{}, fmt.Errorf("clear role permissions: %w", err)
	}
	for _, permID := range permissionIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, id, permID); err != nil {
			if isForeignKeyViolation(err) {
				return domain.Role{}, ErrInvalidPermission
			}
			return domain.Role{}, fmt.Errorf("assign permission: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Role{}, err
	}
	return role, nil
}

func (r *Repository) SetUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear roles: %w", err)
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
			return fmt.Errorf("assign role: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ---- Refresh tokens ----

type RefreshTokenRecord struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (r *Repository) StoreRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, ip, ua string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, tokenHash, expiresAt, ip, ua,
	)
	return err
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (RefreshTokenRecord, error) {
	var rec RefreshTokenRecord
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, tokenHash,
	).Scan(&rec.ID, &rec.UserID, &rec.ExpiresAt, &rec.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshTokenRecord{}, ErrNotFound
		}
		return RefreshTokenRecord{}, err
	}
	return rec, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (r *Repository) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23503"
	}
	return false
}
