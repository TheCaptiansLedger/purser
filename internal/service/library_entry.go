package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// LibraryEntryService orchestrates domain.LibraryEntry against a
// ports.LibraryEntryRepository. See PersonService for the conventions
// this follows.
type LibraryEntryService struct {
	repo ports.LibraryEntryRepository
}

// NewLibraryEntryService constructs a LibraryEntryService backed by repo.
func NewLibraryEntryService(repo ports.LibraryEntryRepository) *LibraryEntryService {
	return &LibraryEntryService{repo: repo}
}

// Create assigns e a server-generated ID (see
// docs/adr/0020-server-generated-kernel-entity-ids.md), validates it, and
// persists it.
func (s *LibraryEntryService) Create(ctx context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error) {
	e.ID = domain.NewID()
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns the LibraryEntry with the given ID, or ports.ErrNotFound.
func (s *LibraryEntryService) Get(ctx context.Context, id string) (*domain.LibraryEntry, error) {
	return s.repo.Get(ctx, id)
}

// Update validates e and persists it in place of the existing record.
func (s *LibraryEntryService) Update(ctx context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Delete removes the LibraryEntry with the given ID, or returns ports.ErrNotFound.
func (s *LibraryEntryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of LibraryEntry records. kind and parentID are
// independent, optional filters.
func (s *LibraryEntryService) List(ctx context.Context, kind domain.Kind, parentID string, pageSize int, pageToken string) ([]*domain.LibraryEntry, string, error) {
	return s.repo.List(ctx, kind, parentID, pageSize, pageToken)
}
