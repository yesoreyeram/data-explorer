package folders

import (
	"context"
	"errors"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

var (
	ErrCycle           = errors.New("cannot move a folder into its own descendant")
	ErrSubtreeTooLarge = errors.New("folder subtree is too large to move in one operation")
)

// MaxSubtreeSizeForMove bounds how many descendants a single Move
// recomputes - a guardrail against a pathologically large subtree, same
// spirit as MaxFolderDepth.
const MaxSubtreeSizeForMove = 5000

type descendantRef struct {
	ID          string
	AncestorIDs []string
}

// descendants returns every folder whose ancestor chain includes id - one
// GIN-indexed lookup, no recursion needed since AncestorIDs already
// flattens the whole ancestor chain onto each row.
func (r *Repository) descendants(ctx context.Context, id string) ([]descendantRef, error) {
	rows, err := r.db.Query(ctx, `SELECT id, ancestor_ids FROM folders WHERE $1 = ANY(ancestor_ids)`, id)
	if err != nil {
		return nil, fmt.Errorf("query folder descendants: %w", err)
	}
	defer rows.Close()

	var out []descendantRef
	for rows.Next() {
		var d descendantRef
		if err := rows.Scan(&d.ID, &d.AncestorIDs); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// recomputeAncestorsForMove computes the new AncestorIDs for every
// descendant of a moved folder. oldMoverAncestors is the mover's ancestor
// chain *before* the move; by construction every descendant's AncestorIDs
// begins with exactly oldMoverAncestors + [moverID], so that fixed-length
// prefix is what gets replaced with newPrefix (the mover's new ancestor
// chain - the new parent's own AncestorIDs + the new parent's id, or nil
// when moving to root); each descendant's relative path below the mover is
// left untouched. Pure and DB-free so it can be unit-tested directly.
func recomputeAncestorsForMove(moverID string, oldMoverAncestors, newPrefix []string, descendants []descendantRef) map[string][]string {
	skip := len(oldMoverAncestors) + 1 // old ancestors + the mover itself
	updates := make(map[string][]string, len(descendants))
	for _, d := range descendants {
		var suffix []string
		if skip < len(d.AncestorIDs) {
			suffix = d.AncestorIDs[skip:]
		}
		newAncestors := make([]string, 0, len(newPrefix)+1+len(suffix))
		newAncestors = append(newAncestors, newPrefix...)
		newAncestors = append(newAncestors, moverID)
		newAncestors = append(newAncestors, suffix...)
		updates[d.ID] = newAncestors
	}
	return updates
}

// maxRelativeDepth returns the deepest descendant's distance below the
// mover (0 if it has none, or only direct children).
func maxRelativeDepth(oldMoverAncestors []string, descendants []descendantRef) int {
	skip := len(oldMoverAncestors) + 1
	max := 0
	for _, d := range descendants {
		if rel := len(d.AncestorIDs) - skip; rel > max {
			max = rel
		}
	}
	return max
}

// applyMove executes the already-validated, already-computed move inside
// one transaction: reparent the mover, then rewrite AncestorIDs for every
// descendant. A statement timeout backstops the guardrails above in case a
// subtree turns out larger/slower than expected.
func (r *Repository) applyMove(ctx context.Context, id string, newParentID *string, newPrefix []string, updates map[string][]string) (domain.Folder, error) {
	// See Create's comment: pgx encodes a nil slice as SQL NULL, which
	// would violate ancestor_ids' NOT NULL constraint when moving to root.
	if newPrefix == nil {
		newPrefix = []string{}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Folder{}, fmt.Errorf("begin move transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
		return domain.Folder{}, fmt.Errorf("set move statement timeout: %w", err)
	}

	row := tx.QueryRow(ctx,
		`UPDATE folders SET parent_id = $1, ancestor_ids = $2, updated_at = now() WHERE id = $3 RETURNING `+folderColumns,
		newParentID, newPrefix, id,
	)
	mover, err := scanFolder(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Folder{}, ErrConflict
		}
		return domain.Folder{}, fmt.Errorf("update folder parent: %w", err)
	}

	for descID, ancestors := range updates {
		if _, err := tx.Exec(ctx, `UPDATE folders SET ancestor_ids = $1, updated_at = now() WHERE id = $2`, ancestors, descID); err != nil {
			return domain.Folder{}, fmt.Errorf("update descendant ancestors: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Folder{}, fmt.Errorf("commit move transaction: %w", err)
	}
	return mover, nil
}

// Move relocates folder id under newParentID (nil for root-level).
func (s *Service) Move(ctx context.Context, id string, newParentID *string) (domain.Folder, error) {
	if newParentID != nil && *newParentID == id {
		return domain.Folder{}, ErrCycle
	}

	var newPrefix []string
	if newParentID != nil {
		newParent, err := s.repo.Get(ctx, *newParentID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Folder{}, ErrParentNotFound
			}
			return domain.Folder{}, err
		}
		for _, a := range newParent.AncestorIDs {
			if a == id {
				return domain.Folder{}, ErrCycle
			}
		}
		newPrefix = append(append([]string{}, newParent.AncestorIDs...), newParent.ID)
	}

	mover, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Folder{}, err
	}
	descendants, err := s.repo.descendants(ctx, id)
	if err != nil {
		return domain.Folder{}, err
	}
	if len(descendants) > MaxSubtreeSizeForMove {
		return domain.Folder{}, ErrSubtreeTooLarge
	}

	relDepth := maxRelativeDepth(mover.AncestorIDs, descendants)
	if len(newPrefix)+1+relDepth >= MaxFolderDepth {
		return domain.Folder{}, ErrMaxDepthExceeded
	}

	updates := recomputeAncestorsForMove(id, mover.AncestorIDs, newPrefix, descendants)
	return s.repo.applyMove(ctx, id, newParentID, newPrefix, updates)
}
