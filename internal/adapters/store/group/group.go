// Package group is the datastore-backed adapter for the
// ports.GroupRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, exposing List's libraryEntryID
// filter as a named parameter instead of a generic map. See
// docs/adr/0012-datastore-persistence.md.
package group

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "group"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.GroupRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.Group]
}

var _ ports.GroupRepository = (*Repository)(nil)

// New constructs a named ports.GroupRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(g *domain.Group) string { return g.ID }

func indexOf(g *domain.Group) map[string]string {
	return map[string]string{"library_entry_id": g.LibraryEntryID}
}

// Create implements ports.GroupRepository.
func (r *Repository) Create(ctx context.Context, g *domain.Group) error {
	return r.inner.Create(ctx, g)
}

// Get implements ports.GroupRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Group, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.GroupRepository.
func (r *Repository) Update(ctx context.Context, g *domain.Group) error {
	return r.inner.Update(ctx, g)
}

// Delete implements ports.GroupRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// DeleteBatch implements ports.GroupRepository.
func (r *Repository) DeleteBatch(ctx context.Context, ids []string) error {
	return r.inner.DeleteBatch(ctx, ids)
}

// List implements ports.GroupRepository. libraryEntryID is an optional
// filter — an empty string means "no filter on this field."
func (r *Repository) List(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) ([]*domain.Group, string, error) {
	var filter map[string]string
	if libraryEntryID != "" {
		filter = map[string]string{"library_entry_id": libraryEntryID}
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
