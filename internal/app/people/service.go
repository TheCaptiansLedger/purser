package people

import (
	"context"
	"database/sql"
	"errors"

	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

// Service handles people (performers, cast, artists, actresses).
type Service struct {
	people ports.PersonRepository
}

func New(people ports.PersonRepository) *Service {
	return &Service{people: people}
}

func (s *Service) CreatePerson(ctx context.Context, p *domain.Person) error {
	if p.Name == "" {
		return errs.Validation("name is required")
	}
	if p.SortName == "" {
		p.SortName = p.Name
	}
	if p.MonitorMode == "" {
		p.MonitorMode = domain.MonitorAll
	}
	return s.people.Save(ctx, p)
}

func (s *Service) GetPerson(ctx context.Context, id string) (*domain.Person, error) {
	p, err := s.people.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return p, err
}

func (s *Service) ListPeople(ctx context.Context, f ports.PersonFilter) ([]*domain.Person, int, error) {
	return s.people.List(ctx, f)
}

func (s *Service) SavePerson(ctx context.Context, p *domain.Person) error {
	if p.Name == "" {
		return errs.Validation("name is required")
	}
	return s.people.Save(ctx, p)
}

func (s *Service) DeletePerson(ctx context.Context, id string) error {
	if _, err := s.GetPerson(ctx, id); err != nil {
		return err
	}
	return s.people.Delete(ctx, id)
}
