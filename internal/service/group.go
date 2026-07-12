package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// GroupService orchestrates domain.Group against a ports.GroupRepository.
// See PersonService for the conventions this follows.
type GroupService struct {
	repo ports.GroupRepository
}

// NewGroupService constructs a GroupService backed by repo.
func NewGroupService(repo ports.GroupRepository) *GroupService {
	return &GroupService{repo: repo}
}

// Create validates g and persists it.
func (s *GroupService) Create(ctx context.Context, g *domain.Group) (*domain.Group, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Get returns the Group with the given ID, or ports.ErrNotFound.
func (s *GroupService) Get(ctx context.Context, id string) (*domain.Group, error) {
	return s.repo.Get(ctx, id)
}

// Update validates g and persists it in place of the existing record.
func (s *GroupService) Update(ctx context.Context, g *domain.Group) (*domain.Group, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Delete removes the Group with the given ID, or returns ports.ErrNotFound.
func (s *GroupService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of Group records.
func (s *GroupService) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Group, string, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}
