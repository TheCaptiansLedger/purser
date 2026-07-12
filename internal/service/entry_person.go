package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// EntryPersonService orchestrates domain.EntryPerson against a
// ports.EntryPersonRepository. See PersonService for the general
// conventions this follows — Get/Update/Delete key on the composite
// (libraryEntryID, personID, role) since EntryPerson has no independent ID.
type EntryPersonService struct {
	repo ports.EntryPersonRepository
}

// NewEntryPersonService constructs an EntryPersonService backed by repo.
func NewEntryPersonService(repo ports.EntryPersonRepository) *EntryPersonService {
	return &EntryPersonService{repo: repo}
}

// Create validates ep and persists it.
func (s *EntryPersonService) Create(ctx context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error) {
	if err := ep.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, ep); err != nil {
		return nil, err
	}
	return ep, nil
}

// Get returns the EntryPerson for the given composite key, or ports.ErrNotFound.
func (s *EntryPersonService) Get(ctx context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error) {
	return s.repo.Get(ctx, libraryEntryID, personID, role)
}

// Update validates ep and persists it in place of the existing record.
func (s *EntryPersonService) Update(ctx context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error) {
	if err := ep.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, ep); err != nil {
		return nil, err
	}
	return ep, nil
}

// Delete removes the EntryPerson for the given composite key, or returns ports.ErrNotFound.
func (s *EntryPersonService) Delete(ctx context.Context, libraryEntryID, personID, role string) error {
	return s.repo.Delete(ctx, libraryEntryID, personID, role)
}

// List returns a page of EntryPerson records, optionally filtered by
// libraryEntryID and/or personID.
func (s *EntryPersonService) List(ctx context.Context, libraryEntryID, personID string, pageSize int, pageToken string) ([]*domain.EntryPerson, string, error) {
	return s.repo.List(ctx, libraryEntryID, personID, pageSize, pageToken)
}
