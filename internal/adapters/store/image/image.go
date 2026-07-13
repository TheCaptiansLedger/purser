// Package image is the datastore-backed adapter for the
// ports.ImageRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, exposing List's ownerType/ownerID
// filter as named parameters instead of a generic map. See
// docs/adr/0012-datastore-persistence.md.
package image

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "image"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.ImageRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.Image]
}

var _ ports.ImageRepository = (*Repository)(nil)

// New constructs a named ports.ImageRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(img *domain.Image) string { return img.ID }

func indexOf(img *domain.Image) map[string]string {
	return map[string]string{"owner_type": img.OwnerType, "owner_id": img.OwnerID}
}

// Create implements ports.ImageRepository.
func (r *Repository) Create(ctx context.Context, img *domain.Image) error {
	return r.inner.Create(ctx, img)
}

// Get implements ports.ImageRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Image, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.ImageRepository.
func (r *Repository) Update(ctx context.Context, img *domain.Image) error {
	return r.inner.Update(ctx, img)
}

// Delete implements ports.ImageRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// List implements ports.ImageRepository. ownerType and ownerID are
// independent, optional filters — an empty string means "no filter on
// this field."
func (r *Repository) List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	filter := map[string]string{}
	if ownerType != "" {
		filter["owner_type"] = ownerType
	}
	if ownerID != "" {
		filter["owner_id"] = ownerID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
