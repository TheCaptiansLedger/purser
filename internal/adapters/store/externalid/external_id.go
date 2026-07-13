// Package externalid is the datastore-backed adapter for the
// ports.ExternalIDRepository port. Unlike entryperson/itemperson, its
// port signature uses domain.EntityType (not a plain string) for the
// first key part, so it can't satisfy ports.ExternalIDRepository
// structurally the way those two do — this wraps
// internal/adapters/store.CompositeRepository[domain.ExternalID] and
// converts at each call. See docs/adr/0012-datastore-persistence.md.
package externalid

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "external_id"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.ExternalIDRepository adapter.
type Repository struct {
	inner *store.CompositeRepository[domain.ExternalID]
}

var _ ports.ExternalIDRepository = (*Repository)(nil)

// New constructs a named ports.ExternalIDRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.ExternalIDRepository, error) {
	inner, err := store.NewComposite(name, collection, ds, keyOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func keyOf(e *domain.ExternalID) (string, string, string) {
	return string(e.EntityType), e.EntityID, string(e.Source)
}

func indexOf(e *domain.ExternalID) map[string]string {
	return map[string]string{"k1": string(e.EntityType), "k2": e.EntityID}
}

// Create implements ports.ExternalIDRepository.
func (r *Repository) Create(ctx context.Context, e *domain.ExternalID) error {
	return r.inner.Create(ctx, e)
}

// Get implements ports.ExternalIDRepository.
func (r *Repository) Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error) {
	return r.inner.Get(ctx, string(entityType), entityID, source)
}

// Update implements ports.ExternalIDRepository.
func (r *Repository) Update(ctx context.Context, e *domain.ExternalID) error {
	return r.inner.Update(ctx, e)
}

// Delete implements ports.ExternalIDRepository.
func (r *Repository) Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error {
	return r.inner.Delete(ctx, string(entityType), entityID, source)
}

// List implements ports.ExternalIDRepository.
func (r *Repository) List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.ExternalID, string, error) {
	filter := map[string]string{}
	if entityType != "" {
		filter["k1"] = string(entityType)
	}
	if entityID != "" {
		filter["k2"] = entityID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
