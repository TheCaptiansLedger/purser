// Package itemperson is the datastore-backed adapter for the
// ports.ItemPersonRepository port — a thin wrapper over the shared generic
// composite-key translator in internal/adapters/store, exposing List's
// itemID/personID filter as named parameters instead of a generic map. See
// docs/adr/0012-datastore-persistence.md.
package itemperson

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "item_person"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.ItemPersonRepository adapter.
type Repository struct {
	inner *store.CompositeRepository[domain.ItemPerson]
}

var _ ports.ItemPersonRepository = (*Repository)(nil)

// New constructs a named ports.ItemPersonRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.ItemPersonRepository, error) {
	inner, err := store.NewComposite(name, collection, ds, keyOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func keyOf(i *domain.ItemPerson) (string, string, string) {
	return i.ItemID, i.PersonID, i.Role
}

func indexOf(i *domain.ItemPerson) map[string]string {
	return map[string]string{"k1": i.ItemID, "k2": i.PersonID}
}

// Create implements ports.ItemPersonRepository.
func (r *Repository) Create(ctx context.Context, i *domain.ItemPerson) error {
	return r.inner.Create(ctx, i)
}

// Get implements ports.ItemPersonRepository.
func (r *Repository) Get(ctx context.Context, itemID, personID, role string) (*domain.ItemPerson, error) {
	return r.inner.Get(ctx, itemID, personID, role)
}

// Update implements ports.ItemPersonRepository.
func (r *Repository) Update(ctx context.Context, i *domain.ItemPerson) error {
	return r.inner.Update(ctx, i)
}

// Delete implements ports.ItemPersonRepository.
func (r *Repository) Delete(ctx context.Context, itemID, personID, role string) error {
	return r.inner.Delete(ctx, itemID, personID, role)
}

// List implements ports.ItemPersonRepository. itemID and personID are
// independent, optional filters — an empty string means "no filter on
// this field."
func (r *Repository) List(ctx context.Context, itemID, personID string, pageSize int, pageToken string) ([]*domain.ItemPerson, string, error) {
	filter := map[string]string{}
	if itemID != "" {
		filter["k1"] = itemID
	}
	if personID != "" {
		filter["k2"] = personID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
