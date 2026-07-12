package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ItemPersonService orchestrates domain.ItemPerson against a
// ports.ItemPersonRepository. See EntryPersonService for the composite-key
// conventions this follows.
type ItemPersonService struct {
	repo ports.ItemPersonRepository
}

// NewItemPersonService constructs an ItemPersonService backed by repo.
func NewItemPersonService(repo ports.ItemPersonRepository) *ItemPersonService {
	return &ItemPersonService{repo: repo}
}

// Create validates ip and persists it.
func (s *ItemPersonService) Create(ctx context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error) {
	if err := ip.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, ip); err != nil {
		return nil, err
	}
	return ip, nil
}

// Get returns the ItemPerson for the given composite key, or ports.ErrNotFound.
func (s *ItemPersonService) Get(ctx context.Context, itemID, personID, role string) (*domain.ItemPerson, error) {
	return s.repo.Get(ctx, itemID, personID, role)
}

// Update validates ip and persists it in place of the existing record.
func (s *ItemPersonService) Update(ctx context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error) {
	if err := ip.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ip); err != nil {
		return nil, err
	}
	return ip, nil
}

// Delete removes the ItemPerson for the given composite key, or returns ports.ErrNotFound.
func (s *ItemPersonService) Delete(ctx context.Context, itemID, personID, role string) error {
	return s.repo.Delete(ctx, itemID, personID, role)
}

// List returns a page of ItemPerson records, optionally filtered by
// itemID and/or personID.
func (s *ItemPersonService) List(ctx context.Context, itemID, personID string, pageSize int, pageToken string) ([]*domain.ItemPerson, string, error) {
	return s.repo.List(ctx, itemID, personID, pageSize, pageToken)
}
