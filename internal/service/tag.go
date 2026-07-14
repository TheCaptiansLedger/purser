package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// TagService orchestrates domain.Tag against a ports.TagRepository. See
// PersonService for the conventions this follows.
type TagService struct {
	repo ports.TagRepository
}

// NewTagService constructs a TagService backed by repo.
func NewTagService(repo ports.TagRepository) *TagService {
	return &TagService{repo: repo}
}

// Create assigns t a server-generated ID (see
// docs/adr/0020-server-generated-kernel-entity-ids.md), validates it, and
// persists it. The repository enforces (Scope, Key, Value) uniqueness as
// get-or-create — if a Tag with the same identity already exists, t is
// mutated in place to that existing Tag (its generated ID discarded)
// before being returned, rather than creating a duplicate. See
// docs/adr/0019-tag-identity-and-get-or-create.md.
func (s *TagService) Create(ctx context.Context, t *domain.Tag) (*domain.Tag, error) {
	t.ID = domain.NewID()
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Get returns the Tag with the given ID, or ports.ErrNotFound.
func (s *TagService) Get(ctx context.Context, id string) (*domain.Tag, error) {
	return s.repo.Get(ctx, id)
}

// Update validates t and persists it in place of the existing record. If
// t's (Scope, Key, Value) differs from the existing Tag's, this renames
// its identity — the repository rejects the rename with ports.ErrConflict
// if the new identity is already owned by a different, live Tag. See
// docs/adr/0019-tag-identity-and-get-or-create.md.
func (s *TagService) Update(ctx context.Context, t *domain.Tag) (*domain.Tag, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete removes the Tag with the given ID, or returns ports.ErrNotFound.
func (s *TagService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of Tag records.
func (s *TagService) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Tag, string, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}
