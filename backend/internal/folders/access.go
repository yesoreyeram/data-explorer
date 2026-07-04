package folders

import (
	"context"
	"errors"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

var (
	// ErrTooManyBindings guards against unbounded JWT growth - folder
	// grants are embedded in the access token (see rbac.Principal), so a
	// user's total binding count needs a sane cap, the same way
	// workflow.MaxNodes/MaxEdges bound a different kind of unbounded input.
	ErrTooManyBindings = errors.New("this user already has the maximum number of folder-scoped role bindings")
	ErrInvalidGrant    = errors.New("the user or role for this grant does not exist")
)

// MaxRoleBindingsPerUser bounds how many folder-scoped bindings a single
// user can hold in total (across all folders) - see ErrTooManyBindings.
const MaxRoleBindingsPerUser = 200

// ListAccess returns every folder-scoped role binding on folderID, with the
// grantee's email and role name resolved for display.
func (r *Repository) ListAccess(ctx context.Context, folderID string) ([]domain.FolderRoleBinding, error) {
	rows, err := r.db.Query(ctx,
		`SELECT frb.id, frb.folder_id, frb.user_id, u.email, frb.role_id, ro.name, frb.created_by, frb.created_at
		 FROM folder_role_bindings frb
		 JOIN users u ON u.id = frb.user_id
		 JOIN roles ro ON ro.id = frb.role_id
		 WHERE frb.folder_id = $1
		 ORDER BY frb.created_at DESC`, folderID)
	if err != nil {
		return nil, fmt.Errorf("query folder access: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderRoleBinding
	for rows.Next() {
		var b domain.FolderRoleBinding
		if err := rows.Scan(&b.ID, &b.FolderID, &b.UserID, &b.UserEmail, &b.RoleID, &b.RoleName, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GrantAccess binds userID to roleID scoped to folderID (and its
// descendants). Idempotent-ish: granting the same (folder, user, role)
// triple twice is a conflict (see the table's UNIQUE constraint), not a
// silent no-op, so callers see they've already granted this exact binding.
func (r *Repository) GrantAccess(ctx context.Context, folderID, userID, roleID, createdBy string) (domain.FolderRoleBinding, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM folder_role_bindings WHERE user_id = $1`, userID).Scan(&count); err != nil {
		return domain.FolderRoleBinding{}, fmt.Errorf("count existing bindings: %w", err)
	}
	if count >= MaxRoleBindingsPerUser {
		return domain.FolderRoleBinding{}, ErrTooManyBindings
	}

	var b domain.FolderRoleBinding
	err := r.db.QueryRow(ctx,
		`INSERT INTO folder_role_bindings (folder_id, user_id, role_id, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, folder_id, user_id, role_id, created_by, created_at`,
		folderID, userID, roleID, createdBy,
	).Scan(&b.ID, &b.FolderID, &b.UserID, &b.RoleID, &b.CreatedBy, &b.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.FolderRoleBinding{}, ErrConflict
		}
		if isForeignKeyViolation(err) {
			return domain.FolderRoleBinding{}, ErrInvalidGrant
		}
		return domain.FolderRoleBinding{}, fmt.Errorf("insert folder access grant: %w", err)
	}
	return b, nil
}

func (r *Repository) RevokeAccess(ctx context.Context, bindingID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM folder_role_bindings WHERE id = $1`, bindingID)
	if err != nil {
		return fmt.Errorf("delete folder access grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListAccess(ctx context.Context, folderID string) ([]domain.FolderRoleBinding, error) {
	return s.repo.ListAccess(ctx, folderID)
}

func (s *Service) GrantAccess(ctx context.Context, folderID, userID, roleID, createdBy string) (domain.FolderRoleBinding, error) {
	return s.repo.GrantAccess(ctx, folderID, userID, roleID, createdBy)
}

func (s *Service) RevokeAccess(ctx context.Context, bindingID string) error {
	return s.repo.RevokeAccess(ctx, bindingID)
}
