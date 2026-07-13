package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ItemService orchestrates domain.Item against a ports.ItemRepository.
// See PersonService for the conventions this follows.
type ItemService struct {
	repo ports.ItemRepository
}

// NewItemService constructs an ItemService backed by repo.
func NewItemService(repo ports.ItemRepository) *ItemService {
	return &ItemService{repo: repo}
}

// Create validates i and persists it.
func (s *ItemService) Create(ctx context.Context, i *domain.Item) (*domain.Item, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

// Get returns the Item with the given ID, or ports.ErrNotFound.
func (s *ItemService) Get(ctx context.Context, id string) (*domain.Item, error) {
	return s.repo.Get(ctx, id)
}

// Update validates i and persists it in place of the existing record.
func (s *ItemService) Update(ctx context.Context, i *domain.Item) (*domain.Item, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

// Delete removes the Item with the given ID, or returns ports.ErrNotFound.
func (s *ItemService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of Item records. libraryEntryID, contentType, and
// groupID are independent, optional filters.
func (s *ItemService) List(ctx context.Context, libraryEntryID, contentType, groupID string, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	return s.repo.List(ctx, libraryEntryID, contentType, groupID, pageSize, pageToken)
}
