// Package mediafile is the datastore-backed adapter for the
// ports.MediaFileRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, exposing List's itemID filter as
// a named parameter instead of a generic map. See
// docs/adr/0012-datastore-persistence.md.
package mediafile

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "media_file"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.MediaFileRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.MediaFile]
}

var _ ports.MediaFileRepository = (*Repository)(nil)

// New constructs a named ports.MediaFileRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(m *domain.MediaFile) string { return m.ID }

func indexOf(m *domain.MediaFile) map[string]string {
	return map[string]string{"item_id": m.ItemID}
}

// Create implements ports.MediaFileRepository.
func (r *Repository) Create(ctx context.Context, m *domain.MediaFile) error {
	return r.inner.Create(ctx, m)
}

// Get implements ports.MediaFileRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.MediaFile, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.MediaFileRepository.
func (r *Repository) Update(ctx context.Context, m *domain.MediaFile) error {
	return r.inner.Update(ctx, m)
}

// Delete implements ports.MediaFileRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// List implements ports.MediaFileRepository. itemID is an optional filter
// — an empty string means "no filter on this field."
func (r *Repository) List(ctx context.Context, itemID string, pageSize int, pageToken string) ([]*domain.MediaFile, string, error) {
	var filter map[string]string
	if itemID != "" {
		filter = map[string]string{"item_id": itemID}
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
