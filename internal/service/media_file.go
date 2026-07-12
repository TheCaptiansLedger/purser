package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// MediaFileService orchestrates domain.MediaFile against a
// ports.MediaFileRepository. See PersonService for the conventions this
// follows.
type MediaFileService struct {
	repo ports.MediaFileRepository
}

// NewMediaFileService constructs a MediaFileService backed by repo.
func NewMediaFileService(repo ports.MediaFileRepository) *MediaFileService {
	return &MediaFileService{repo: repo}
}

// Create validates m and persists it.
func (s *MediaFileService) Create(ctx context.Context, m *domain.MediaFile) (*domain.MediaFile, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Get returns the MediaFile with the given ID, or ports.ErrNotFound.
func (s *MediaFileService) Get(ctx context.Context, id string) (*domain.MediaFile, error) {
	return s.repo.Get(ctx, id)
}

// Update validates m and persists it in place of the existing record.
func (s *MediaFileService) Update(ctx context.Context, m *domain.MediaFile) (*domain.MediaFile, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete removes the MediaFile with the given ID, or returns ports.ErrNotFound.
func (s *MediaFileService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of MediaFile records.
func (s *MediaFileService) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.MediaFile, string, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}
