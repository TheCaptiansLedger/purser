// Package item is the datastore-backed adapter for the
// ports.ItemRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, exposing List's
// libraryEntryID/contentType/groupID filter as named parameters instead of
// a generic map. See docs/adr/0012-datastore-persistence.md.
package item

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/ports"
)

const collection = "item"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.ItemRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[domain.Item]
}

var _ ports.ItemRepository = (*Repository)(nil)

// New constructs a named ports.ItemRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(i *domain.Item) string { return i.ID }

func indexOf(i *domain.Item) map[string]string {
	return map[string]string{
		"library_entry_id": i.LibraryEntryID,
		"content_type":     string(i.ContentType),
		"group_id":         i.GroupID,
		"status":           string(i.Status),
	}
}

// Create implements ports.ItemRepository.
func (r *Repository) Create(ctx context.Context, i *domain.Item) error {
	return r.inner.Create(ctx, i)
}

// Get implements ports.ItemRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Item, error) {
	return r.inner.Get(ctx, id)
}

// Update implements ports.ItemRepository.
func (r *Repository) Update(ctx context.Context, i *domain.Item) error {
	return r.inner.Update(ctx, i)
}

// Delete implements ports.ItemRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// DeleteBatch implements ports.ItemRepository.
func (r *Repository) DeleteBatch(ctx context.Context, ids []string) error {
	return r.inner.DeleteBatch(ctx, ids)
}

// List implements ports.ItemRepository. libraryEntryID, contentType,
// groupID, and status are independent, optional filters — an empty string
// (or domain.ItemStatus("") for status) means "no filter on this field."
func (r *Repository) List(ctx context.Context, libraryEntryID, contentType, groupID string, status domain.ItemStatus, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	filter := map[string]string{}
	if libraryEntryID != "" {
		filter["library_entry_id"] = libraryEntryID
	}
	if contentType != "" {
		filter["content_type"] = contentType
	}
	if groupID != "" {
		filter["group_id"] = groupID
	}
	if status != "" {
		filter["status"] = string(status)
	}
	if len(filter) == 0 {
		filter = nil
	}
	return r.inner.List(ctx, filter, pageSize, pageToken)
}
