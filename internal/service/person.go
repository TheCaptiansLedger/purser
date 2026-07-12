// Package service orchestrates internal/domain business rules against
// internal/ports repository interfaces — one service type per kernel
// entity (SRP), each constructible with any conforming port implementation
// including a test fake. See docs/adr/0001-hexagonal-architecture.md,
// docs/adr/0002-solid-design-principles.md, and docs/adr/0011-api-design.md.
//
// Services have zero knowledge of any transport (Connect, HTTP) — a
// transport-specific concept like a protobuf FieldMask is translated by
// the driving adapter (internal/api/connect) into a complete domain value
// before it ever reaches a service method.
package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// PersonService orchestrates domain.Person against a ports.PersonRepository.
// It knows nothing about any other kernel entity or metadata provider —
// per ADR-0011, a caller assembles a fully-formed Person before calling
// Create/Update; this service only persists what it's given.
type PersonService struct {
	repo ports.PersonRepository
}

// NewPersonService constructs a PersonService backed by repo, which may be
// any conforming ports.PersonRepository implementation, including a test
// fake.
func NewPersonService(repo ports.PersonRepository) *PersonService {
	return &PersonService{repo: repo}
}

// Create validates p and persists it.
func (s *PersonService) Create(ctx context.Context, p *domain.Person) (*domain.Person, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns the Person with the given ID, or ports.ErrNotFound.
func (s *PersonService) Get(ctx context.Context, id string) (*domain.Person, error) {
	return s.repo.Get(ctx, id)
}

// Update validates p and persists it in place of the existing record. p
// must already reflect the caller's intended final state — partial-update
// (field mask) merging happens in the driving adapter, not here.
func (s *PersonService) Update(ctx context.Context, p *domain.Person) (*domain.Person, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes the Person with the given ID, or returns ports.ErrNotFound.
func (s *PersonService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// List returns a page of Person records. See ports.PersonRepository for
// the pagination contract.
func (s *PersonService) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Person, string, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}
