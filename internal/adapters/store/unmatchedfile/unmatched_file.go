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

// indexOf writes Status plus every hash field into Document.Index —
// Status is the review queue's filter dimension, read back by List below;
// the hash fields back GetByHash (ADR-0024's "already known"
// short-circuit), one indexed key per hash algorithm since Datastore's
// List filter is an AND-match over the whole map, not an OR across keys.
func indexOf(u *domain.UnmatchedFile) map[string]string {
	return map[string]string{
		"status": string(u.Status),
		"oshash": u.OSHash,
		"sha1":   u.SHA1,
		"md5":    u.MD5,
		"sha512": u.SHA512,
	}
}

// hashLookups pairs each hash algorithm's index key with the
// caller-supplied value, in the order GetByHash checks them.
func hashLookups(oshash, sha1, md5, sha512 string) []struct{ key, value string } {
	return []struct{ key, value string }{
		{"oshash", oshash},
		{"sha1", sha1},
		{"md5", md5},
		{"sha512", sha512},
	}
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

// Update implements ports.UnmatchedFileRepository.
func (r *Repository) Update(ctx context.Context, u *domain.UnmatchedFile) error {
	return r.inner.Update(ctx, u)
}

// GetByHash implements ports.UnmatchedFileRepository.
func (r *Repository) GetByHash(ctx context.Context, oshash, sha1, md5, sha512 string) (*domain.UnmatchedFile, error) {
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
