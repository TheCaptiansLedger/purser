package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ImageService orchestrates domain.Image against a ports.ImageRepository.
// See PersonService for the conventions this follows.
type ImageService struct {
	repo ports.ImageRepository
}

// NewImageService constructs an ImageService backed by repo.
func NewImageService(repo ports.ImageRepository) *ImageService {
	return &ImageService{repo: repo}
}

// Create assigns img a server-generated ID (see
// docs/adr/0020-server-generated-kernel-entity-ids.md), validates it, and
// persists it.
func (s *ImageService) Create(ctx context.Context, img *domain.Image) (*domain.Image, error) {
	img.ID = domain.NewID()
	if err := img.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

// Get returns the Image with the given ID, or ports.ErrNotFound.
func (s *ImageService) Get(ctx context.Context, id string) (*domain.Image, error) {
	return s.repo.Get(ctx, id)
}

// Update validates img and persists it in place of the existing record.
func (s *ImageService) Update(ctx context.Context, img *domain.Image) (*domain.Image, error) {
	if err := img.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

// Delete removes the Image with the given ID, or returns ports.ErrNotFound.
func (s *ImageService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of Image records, optionally filtered by ownerType
// and/or ownerID.
func (s *ImageService) List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	return s.repo.List(ctx, ownerType, ownerID, pageSize, pageToken)
}
