package service

import (
	"context"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
)

// PerformerProfileService orchestrates afterdark.PerformerProfile against
// a ports.PerformerProfileRepository. See PersonService for the
// conventions this follows — AfterDark's only module-specific service,
// added with zero edits to any kernel service file.
type PerformerProfileService struct {
	repo ports.PerformerProfileRepository
}

// NewPerformerProfileService constructs a PerformerProfileService backed
// by repo.
func NewPerformerProfileService(repo ports.PerformerProfileRepository) *PerformerProfileService {
	return &PerformerProfileService{repo: repo}
}

// Create validates p and persists it.
func (s *PerformerProfileService) Create(ctx context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns the PerformerProfile for the given person ID, or ports.ErrNotFound.
func (s *PerformerProfileService) Get(ctx context.Context, personID string) (*afterdark.PerformerProfile, error) {
	return s.repo.Get(ctx, personID)
}

// Update validates p and persists it in place of the existing record.
func (s *PerformerProfileService) Update(ctx context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes the PerformerProfile for the given person ID, or returns ports.ErrNotFound.
func (s *PerformerProfileService) Delete(ctx context.Context, personID string) error {
	return s.repo.Delete(ctx, personID)
}

// List returns a page of PerformerProfile records.
func (s *PerformerProfileService) List(ctx context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerProfile, string, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}
