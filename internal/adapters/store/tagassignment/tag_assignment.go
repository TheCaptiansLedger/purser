// Package tagassignment is the datastore-backed adapter for the
// ports.TagAssignmentRepository port. Its port signature uses
// domain.EntityType (not a plain string) for one key part, so it can't
// satisfy ports.TagAssignmentRepository structurally the way a bare
// CompositeRepository[T] can — this wraps
// internal/adapters/store.CompositeRepository[domain.TagAssignment] and
// converts at each call, same pattern as externalid. Unlike
// entryperson/itemperson/externalid, all 3 composite-key parts (tag_id,
// entity_type, entity_id) are independently indexed, not just 2 — List
// needs both the browse-by-tag and show-an-entity's-tags directions to be
// real indexed lookups. See docs/technical/tag-assignment.md.
package tagassignment

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "tag_assignment"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.TagAssignmentRepository adapter.
type Repository struct {
	inner *store.CompositeRepository[domain.TagAssignment]
}

var _ ports.TagAssignmentRepository = (*Repository)(nil)

// New constructs a named ports.TagAssignmentRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (ports.TagAssignmentRepository, error) {
	inner, err := store.NewComposite(name, collection, ds, keyOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func keyOf(ta *domain.TagAssignment) (string, string, string) {
	return ta.TagID, string(ta.EntityType), ta.EntityID
}

func indexOf(ta *domain.TagAssignment) map[string]string {
	return map[string]string{
		"tag_id":      ta.TagID,
		"entity_type": string(ta.EntityType),
		"entity_id":   ta.EntityID,
	}
}

// Create implements ports.TagAssignmentRepository.
func (r *Repository) Create(ctx context.Context, ta *domain.TagAssignment) error {
	return r.inner.Create(ctx, ta)
}

// CreateBatch implements ports.TagAssignmentRepository.
func (r *Repository) CreateBatch(ctx context.Context, tas []*domain.TagAssignment) error {
	return r.inner.CreateBatch(ctx, tas)
}

// Get implements ports.TagAssignmentRepository.
func (r *Repository) Get(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error) {
	return r.inner.Get(ctx, tagID, string(entityType), entityID)
}

// Delete implements ports.TagAssignmentRepository.
func (r *Repository) Delete(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) error {
	return r.inner.Delete(ctx, tagID, string(entityType), entityID)
}

// List implements ports.TagAssignmentRepository. tagID and
// (entityType, entityID) are independent, optional filter directions.
func (r *Repository) List(ctx context.Context, tagID string, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.TagAssignment, string, error) {
	filter := map[string]string{}
	if tagID != "" {
		filter["tag_id"] = tagID
	}
	if entityType != "" {
		filter["entity_type"] = string(entityType)
	}
	if entityID != "" {
		filter["entity_id"] = entityID
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
