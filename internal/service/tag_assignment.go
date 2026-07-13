package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// TagAssignmentService orchestrates domain.TagAssignment against a
// ports.TagAssignmentRepository. See ItemPersonService for the
// composite-key conventions this follows. There is no Update — nothing
// about a TagAssignment is mutable.
type TagAssignmentService struct {
	repo ports.TagAssignmentRepository
}

// NewTagAssignmentService constructs a TagAssignmentService backed by repo.
func NewTagAssignmentService(repo ports.TagAssignmentRepository) *TagAssignmentService {
	return &TagAssignmentService{repo: repo}
}

// Create validates ta and persists it.
func (s *TagAssignmentService) Create(ctx context.Context, ta *domain.TagAssignment) (*domain.TagAssignment, error) {
	if err := ta.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, ta); err != nil {
		return nil, err
	}
	return ta, nil
}

// Get returns the TagAssignment for the given composite key, or ports.ErrNotFound.
func (s *TagAssignmentService) Get(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error) {
	return s.repo.Get(ctx, tagID, entityType, entityID)
}

// Delete removes the TagAssignment for the given composite key, or returns ports.ErrNotFound.
func (s *TagAssignmentService) Delete(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) error {
	return s.repo.Delete(ctx, tagID, entityType, entityID)
}

// List returns a page of TagAssignment records, optionally filtered by
// tagID and/or (entityType, entityID).
func (s *TagAssignmentService) List(ctx context.Context, tagID string, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.TagAssignment, string, error) {
	return s.repo.List(ctx, tagID, entityType, entityID, pageSize, pageToken)
}
