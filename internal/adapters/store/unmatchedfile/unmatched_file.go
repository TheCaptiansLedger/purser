// Package unmatchedfile is the datastore-backed adapter for the
// ports.UnmatchedFileRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, per docs/adr/0012's
// Image/Tag/MusicRelease hand-written-translator category (this entity
// needs filtered List by Status, so it isn't forced into the generic
// single-ID/no-filter store.Repository[T] shape). See
// docs/adr/0024-pipeline-core.md.
package unmatchedfile

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "unmatched_file"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.UnmatchedFileRepository
// adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.UnmatchedFile]
}

var _ ports.UnmatchedFileRepository = (*Repository)(nil)

// New constructs a named ports.UnmatchedFileRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(u *domain.UnmatchedFile) string { return u.ID }

// indexOf writes Status into Document.Index — ADR-0024 names status as
// the review queue's filter dimension, read back by List below.
func indexOf(u *domain.UnmatchedFile) map[string]string {
	return map[string]string{"status": string(u.Status)}
}

// Create implements ports.UnmatchedFileRepository.
func (r *Repository) Create(ctx context.Context, u *domain.UnmatchedFile) error {
	return r.inner.Create(ctx, u)
}

// Get implements ports.UnmatchedFileRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.UnmatchedFile, error) {
	return r.inner.Get(ctx, id)
}

// List implements ports.UnmatchedFileRepository.
func (r *Repository) List(ctx context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error) {
	var filter map[string]string
	if status != "" {
		filter = map[string]string{"status": string(status)}
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
