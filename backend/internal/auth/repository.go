package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
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
	return out, rows.Err()
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
