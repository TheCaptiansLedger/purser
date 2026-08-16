// Package libraryentry is the datastore-backed adapter for the
// ports.LibraryEntryRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, exposing List's kind/parentID
// filter as named parameters instead of a generic map. See
// docs/adr/0012-datastore-persistence.md.
package libraryentry

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "library_entry"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.LibraryEntryRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.LibraryEntry]
}

var _ ports.LibraryEntryRepository = (*Repository)(nil)

// New constructs a named ports.LibraryEntryRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(e *domain.LibraryEntry) string { return e.ID }

func indexOf(e *domain.LibraryEntry) map[string]string {
	return map[string]string{"kind": string(e.Kind), "parent_id": e.ParentID}
}

// Create implements ports.LibraryEntryRepository.
func (r *Repository) Create(ctx context.Context, e *domain.LibraryEntry) error {
	return r.inner.Create(ctx, e)
}

// Get implements ports.LibraryEntryRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.LibraryEntry, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.LibraryEntryRepository.
func (r *Repository) Update(ctx context.Context, e *domain.LibraryEntry) error {
	return r.inner.Update(ctx, e)
}

// Delete implements ports.LibraryEntryRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// DeleteBatch implements ports.LibraryEntryRepository.
func (r *Repository) DeleteBatch(ctx context.Context, ids []string) error {
	return r.inner.DeleteBatch(ctx, ids)
}

// List implements ports.LibraryEntryRepository. kind and parentID are
// independent, optional filters — an empty string means "no filter on
// this field."
func (r *Repository) List(ctx context.Context, kind domain.Kind, parentID string, pageSize int, pageToken string) ([]*domain.LibraryEntry, string, error) {
	filter := map[string]string{}
	if kind != "" {
		filter["kind"] = string(kind)
	}
	if parentID != "" {
		filter["parent_id"] = parentID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
