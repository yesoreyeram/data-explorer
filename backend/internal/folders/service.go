package folders

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name        string
	Description string
	ParentID    *string
	Tags        []string
	Readme      string
	Metadata    json.RawMessage
	CreatedBy   string
}

// Create computes the new folder's ancestor chain from its parent (empty
// for a root-level folder) and enforces MaxFolderDepth before inserting -
// the parent's own ancestor chain plus the parent itself, exactly the
// invariant every other folder's AncestorIDs already maintains.
func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Folder, error) {
	var ancestorIDs []string
	if in.ParentID != nil {
		parent, err := s.repo.Get(ctx, *in.ParentID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Folder{}, ErrParentNotFound
			}
			return domain.Folder{}, err
		}
		ancestorIDs = append(append([]string{}, parent.AncestorIDs...), parent.ID)
		if len(ancestorIDs) >= MaxFolderDepth {
			return domain.Folder{}, ErrMaxDepthExceeded
		}
	}

	return s.repo.Create(ctx, ancestorIDs, createParams{
		Name:        in.Name,
		Description: in.Description,
		ParentID:    in.ParentID,
		Tags:        in.Tags,
		Readme:      in.Readme,
		Metadata:    in.Metadata,
		CreatedBy:   in.CreatedBy,
	})
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]domain.Folder, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Folder, error) {
	return s.repo.Get(ctx, id)
}

// ScopeChain returns id followed by its ancestor ids - see
// Repository.ScopeChain.
func (s *Service) ScopeChain(ctx context.Context, id string) ([]string, error) {
	return s.repo.ScopeChain(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (domain.Folder, error) {
	return s.repo.Update(ctx, id, in)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
