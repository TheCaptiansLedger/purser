package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ExternalIDService orchestrates domain.ExternalID against a
// ports.ExternalIDRepository. See EntryPersonService for the composite-key
// conventions this follows.
type ExternalIDService struct {
	repo ports.ExternalIDRepository
}

// NewExternalIDService constructs an ExternalIDService backed by repo.
func NewExternalIDService(repo ports.ExternalIDRepository) *ExternalIDService {
	return &ExternalIDService{repo: repo}
}

// Create validates e and persists it.
func (s *ExternalIDService) Create(ctx context.Context, e *domain.ExternalID) (*domain.ExternalID, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns the ExternalID for the given composite key, or ports.ErrNotFound.
func (s *ExternalIDService) Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error) {
	return s.repo.Get(ctx, entityType, entityID, source)
}

// GetByValue returns the ExternalID row currently linking source/value to
// an entity of entityType, or ports.ErrNotFound.
func (s *ExternalIDService) GetByValue(ctx context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error) {
	return s.repo.GetByValue(ctx, entityType, source, value)
}

// Update validates e and persists it in place of the existing record.
func (s *ExternalIDService) Update(ctx context.Context, e *domain.ExternalID) (*domain.ExternalID, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Delete removes the ExternalID for the given composite key, or returns ports.ErrNotFound.
func (s *ExternalIDService) Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error {
	return s.repo.Delete(ctx, entityType, entityID, source)
}

// List returns a page of ExternalID records, optionally filtered by
// entityType and/or entityID.
func (s *ExternalIDService) List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.ExternalID, string, error) {
	return s.repo.List(ctx, entityType, entityID, pageSize, pageToken)
}
