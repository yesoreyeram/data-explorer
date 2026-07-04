// Package folders implements the nested folder hierarchy every stored
// entity (connections, workflows, ...) lives in. It knows nothing about
// those entity types - the FK from e.g. connections.folder_id to
// folders(id) is declared ON DELETE RESTRICT, so the database itself
// refuses to delete a non-empty folder; this package just translates that
// foreign-key violation into a friendly error (see Delete). That's what
// lets any future entity type participate in "can't delete a non-empty
// folder" for free, with zero changes here.
package folders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

var (
	ErrNotFound         = errors.New("folder not found")
	ErrConflict         = errors.New("a folder with this name already exists in this location")
	ErrNotEmpty         = errors.New("folder is not empty")
	ErrParentNotFound   = errors.New("parent folder not found")
	ErrMaxDepthExceeded = errors.New("folder nesting is too deep")
)

// MaxFolderDepth bounds how deeply folders can nest - a guardrail against
// pathological trees, in the same spirit as workflow.MaxNodes/MaxEdges and
// dataframe.DefaultMaxCellBytes elsewhere in this codebase.
const MaxFolderDepth = 20

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const folderColumns = `id, name, description, parent_id, ancestor_ids, depth, tags, readme, metadata,
	created_by, created_at, updated_at`

func scanFolder(row interface {
	Scan(dest ...any) error
}) (domain.Folder, error) {
	var f domain.Folder
	err := row.Scan(&f.ID, &f.Name, &f.Description, &f.ParentID, &f.AncestorIDs, &f.Depth, &f.Tags, &f.Readme, &f.Metadata,
		&f.CreatedBy, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

type createParams struct {
	Name        string
	Description string
	ParentID    *string
	Tags        []string
	Readme      string
	Metadata    json.RawMessage
	CreatedBy   string
}

// Create inserts a folder under ParentID (nil for a root-level folder). The
// caller (Service) is responsible for computing AncestorIDs from the parent
// and enforcing MaxFolderDepth before calling this.
func (r *Repository) Create(ctx context.Context, ancestorIDs []string, p createParams) (domain.Folder, error) {
	if p.Metadata == nil {
		p.Metadata = json.RawMessage(`{}`)
	}
	// pgx encodes a nil Go slice as SQL NULL (not as an empty array), which
	// would violate these NOT NULL columns - the '{}' DEFAULT only applies
	// when a column is omitted from the INSERT entirely, not when NULL is
	// passed explicitly. A root folder has no ancestors, and a caller may
	// not supply any tags, so both need normalizing before binding.
	if ancestorIDs == nil {
		ancestorIDs = []string{}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	row := r.db.QueryRow(ctx,
		`INSERT INTO folders (name, description, parent_id, ancestor_ids, tags, readme, metadata, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+folderColumns,
		p.Name, p.Description, p.ParentID, ancestorIDs, p.Tags, p.Readme, p.Metadata, p.CreatedBy,
	)
	f, err := scanFolder(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Folder{}, ErrConflict
		}
		if isForeignKeyViolation(err) {
			return domain.Folder{}, ErrParentNotFound
		}
		return domain.Folder{}, fmt.Errorf("insert folder: %w", err)
	}
	return f, nil
}

type ListFilter struct {
	Tag string
	Q   string // substring match on name

	// ScopedToFolderIDs, when non-nil, restricts results to folders that
	// are one of these ids or a descendant of one of them - used to filter
	// the list down to what a folder-scoped-only principal can see (see
	// rbac.Principal.GrantedFolderIDs). Leave nil for "no restriction"
	// (callers with a global permission grant skip this entirely).
	ScopedToFolderIDs []string
}

// List returns every matching folder (flat, with ParentID/AncestorIDs so
// callers can build a tree client-side) - matching connections/workflows'
// existing List(ctx) shape of "no pagination, filter client-side," extended
// with optional tag/name/scope filters.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]domain.Folder, error) {
	where := "WHERE 1=1"
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Tag != "" {
		where += " AND " + arg(f.Tag) + " = ANY(tags)"
	}
	if f.Q != "" {
		where += " AND name ILIKE " + arg("%"+f.Q+"%")
	}
	if f.ScopedToFolderIDs != nil {
		// A folder is visible if it IS one of the granted ids, or has one
		// of them as an ancestor (i.e. lives in a granted subtree). id::text
		// avoids a uuid/text[] type mismatch against the plain []string
		// parameter (ancestor_ids is already text[], so no cast needed there).
		p := arg(f.ScopedToFolderIDs)
		where += fmt.Sprintf(" AND (id::text = ANY(%s) OR ancestor_ids && %s)", p, p)
	}

	rows, err := r.db.Query(ctx, `SELECT `+folderColumns+` FROM folders `+where+` ORDER BY depth, name`, args...)
	if err != nil {
		return nil, fmt.Errorf("query folders: %w", err)
	}
	defer rows.Close()

	var out []domain.Folder
	for rows.Next() {
		fold, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fold)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Folder, error) {
	row := r.db.QueryRow(ctx, `SELECT `+folderColumns+` FROM folders WHERE id = $1`, id)
	f, err := scanFolder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Folder{}, ErrNotFound
		}
		return domain.Folder{}, fmt.Errorf("query folder: %w", err)
	}
	return f, nil
}

// ScopeChain returns the folder's own id followed by its ancestor ids (self
// + ancestors) - exactly the set an RBAC scoped-permission check needs to
// test against (see rbac.Principal.HasScoped). One indexed PK lookup, no
// recursion.
func (r *Repository) ScopeChain(ctx context.Context, id string) ([]string, error) {
	var ancestorIDs []string
	err := r.db.QueryRow(ctx, `SELECT ancestor_ids FROM folders WHERE id = $1`, id).Scan(&ancestorIDs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query folder scope chain: %w", err)
	}
	return append([]string{id}, ancestorIDs...), nil
}

type UpdateInput struct {
	Name        string
	Description string
	Tags        []string
	Readme      string
	Metadata    json.RawMessage
}

// Update changes a folder's own metadata (name/description/tags/readme/
// metadata) but never its position in the tree - moving a folder is a
// structurally distinct operation (cycle checks, subtree recompute) handled
// by Move (see move.go), the same way connections/workflows separate plain
// field updates from their own dedicated action endpoints.
func (r *Repository) Update(ctx context.Context, id string, p UpdateInput) (domain.Folder, error) {
	if p.Metadata == nil {
		p.Metadata = json.RawMessage(`{}`)
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	row := r.db.QueryRow(ctx,
		`UPDATE folders SET name = $1, description = $2, tags = $3, readme = $4, metadata = $5, updated_at = now()
		 WHERE id = $6 RETURNING `+folderColumns,
		p.Name, p.Description, p.Tags, p.Readme, p.Metadata, id,
	)
	f, err := scanFolder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Folder{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Folder{}, ErrConflict
		}
		return domain.Folder{}, fmt.Errorf("update folder: %w", err)
	}
	return f, nil
}

// Delete removes a folder. The database enforces "must be empty" for us:
// folders.parent_id, connections.folder_id, and workflows.folder_id (and
// any future entity's folder_id) are all ON DELETE RESTRICT, so a non-empty
// folder fails here with a foreign-key violation - this package never needs
// to know what "content" means for any given entity type. We read the
// violation's TableName to say *what* is blocking the delete (subfolders vs
// connections vs workflows vs some future entity) without ever querying
// those tables ourselves.
func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM folders WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%w: it still has %s", ErrNotEmpty, blockingContentLabel(pgErr.TableName))
		}
		return fmt.Errorf("delete folder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func blockingContentLabel(table string) string {
	switch table {
	case "folders":
		return "subfolders"
	case "connections":
		return "connections"
	case "workflows":
		return "workflows"
	default:
		return "other contents (" + table + ")"
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}
