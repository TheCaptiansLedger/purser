// Package imageselection is the datastore-backed adapter for the
// ports.ImageSelectionRepository port — a thin wrapper over the shared
// generic store.Repository[T] translator, keyed on the
// (ownerType, ownerID, imageType) triple that is domain.ImageSelection's
// whole identity rather than an ID of its own. See
// docs/adr/0012-datastore-persistence.md.
package imageselection

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "image_selection"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.ImageSelectionRepository
// adapter.
type Repository struct {
	inner *store.Repository[domain.ImageSelection]
}

var _ ports.ImageSelectionRepository = (*Repository)(nil)

// New constructs a named ports.ImageSelectionRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.New(name, collection, ds, idOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(sel *domain.ImageSelection) string {
	return key(sel.OwnerType, sel.OwnerID, sel.ImageType)
}

// key derives ImageSelection's document ID directly from its identity
// triple — deterministic, so writing the same slot always lands on the
// same document instead of needing a lookup-then-decide-the-ID step.
func key(ownerType, ownerID string, imageType domain.ImageType) string {
	return ownerType + "/" + ownerID + "/" + string(imageType)
}

// Create implements ports.ImageSelectionRepository.
func (r *Repository) Create(ctx context.Context, sel *domain.ImageSelection) error {
	return r.inner.Create(ctx, sel)
}

// Get implements ports.ImageSelectionRepository.
func (r *Repository) Get(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType) (*domain.ImageSelection, error) {
	return r.inner.Get(ctx, key(ownerType, ownerID, imageType))
}

// Update implements ports.ImageSelectionRepository.
func (r *Repository) Update(ctx context.Context, sel *domain.ImageSelection) error {
	return r.inner.Update(ctx, sel)
}

// Delete implements ports.ImageSelectionRepository.
func (r *Repository) Delete(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType) error {
	return r.inner.Delete(ctx, key(ownerType, ownerID, imageType))
}
