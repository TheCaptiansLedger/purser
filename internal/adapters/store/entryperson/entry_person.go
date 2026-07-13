// Package entryperson is the datastore-backed adapter for the
// ports.EntryPersonRepository port — a thin wrapper over the shared
// generic composite-key translator in internal/adapters/store, exposing
// List's libraryEntryID/personID filter as named parameters instead of a
// generic map. See docs/adr/0012-datastore-persistence.md.
package entryperson

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "entry_person"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.EntryPersonRepository adapter.
type Repository struct {
	inner *store.CompositeRepository[domain.EntryPerson]
}

var _ ports.EntryPersonRepository = (*Repository)(nil)

// New constructs a named ports.EntryPersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.EntryPersonRepository, error) {
	inner, err := store.NewComposite(name, collection, ds, keyOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func keyOf(e *domain.EntryPerson) (string, string, string) {
	return e.LibraryEntryID, e.PersonID, e.Role
}

func indexOf(e *domain.EntryPerson) map[string]string {
	return map[string]string{"k1": e.LibraryEntryID, "k2": e.PersonID}
}

// Create implements ports.EntryPersonRepository.
func (r *Repository) Create(ctx context.Context, e *domain.EntryPerson) error {
	return r.inner.Create(ctx, e)
}

// Get implements ports.EntryPersonRepository.
func (r *Repository) Get(ctx context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error) {
	return r.inner.Get(ctx, libraryEntryID, personID, role)
}

// Update implements ports.EntryPersonRepository.
func (r *Repository) Update(ctx context.Context, e *domain.EntryPerson) error {
	return r.inner.Update(ctx, e)
}

// Delete implements ports.EntryPersonRepository.
func (r *Repository) Delete(ctx context.Context, libraryEntryID, personID, role string) error {
	return r.inner.Delete(ctx, libraryEntryID, personID, role)
}

// List implements ports.EntryPersonRepository. libraryEntryID and personID
// are independent, optional filters — an empty string means "no filter on
// this field."
func (r *Repository) List(ctx context.Context, libraryEntryID, personID string, pageSize int, pageToken string) ([]*domain.EntryPerson, string, error) {
	filter := map[string]string{}
	if libraryEntryID != "" {
		filter["k1"] = libraryEntryID
	}
	if personID != "" {
		filter["k2"] = personID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
