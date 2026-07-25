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

// indexOf writes item_id plus every hash field into Document.Index — the
// hash fields back GetByHash below (ADR-0024's "already known"
// short-circuit), one indexed key per hash algorithm since Datastore's
// List filter is an AND-match over the whole map, not an OR across keys.
func indexOf(m *domain.MediaFile) map[string]string {
	return map[string]string{
		"item_id": m.ItemID,
		"oshash":  m.OSHash,
		"sha1":    m.SHA1,
		"md5":     m.MD5,
		"sha512":  m.SHA512,
	}
}

// hashLookups pairs each hash algorithm's index key with the caller-supplied
// value, in the order GetByHash checks them.
func hashLookups(oshash, sha1, md5, sha512 string) []struct{ key, value string } {
	return []struct{ key, value string }{
		{"oshash", oshash},
		{"sha1", sha1},
		{"md5", md5},
		{"sha512", sha512},
	}
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

// GetByHash implements ports.MediaFileRepository.
func (r *Repository) GetByHash(ctx context.Context, oshash, sha1, md5, sha512 string) (*domain.MediaFile, error) {
	for _, lookup := range hashLookups(oshash, sha1, md5, sha512) {
		if lookup.value == "" {
			continue
		}
		matches, _, err := r.inner.List(ctx, map[string]string{lookup.key: lookup.value}, 1, "")
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return nil, ports.ErrNotFound
}
