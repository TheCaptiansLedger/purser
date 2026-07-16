// Package music is the datastore-backed adapter for the
// ports.MusicReleaseRepository port — a thin wrapper over the shared
// store.FilteredRepository[T] translator, the same pattern
// internal/adapters/store/group and internal/adapters/store/image use.
// GroupID/LibraryEntryID/MBID/Barcode are written into Document.Index on
// Create/Update; ListByGroup/ListByEntry read that index via
// FilteredRepository.List's filter, and GetByMBID/GetByBarcode do the same
// with pageSize 1 — a real indexed lookup, not a scan-and-filter. See
// docs/adr/0021-music-domain-model.md and
// docs/adr/0012-datastore-persistence.md.
package music

import (
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain/music"
	"purser/internal/ports"
)

const collection = "music_release"

// Option customizes a Repository constructed via New.
type Option = store.Option

// WithLogger overrides the default (slog.Default()) logger.
var WithLogger = store.WithLogger

// WithTracerProvider overrides the default (global) TracerProvider.
var WithTracerProvider = store.WithTracerProvider

// WithMeterProvider overrides the default (global) MeterProvider.
var WithMeterProvider = store.WithMeterProvider

// Repository is the datastore-backed ports.MusicReleaseRepository adapter.
type Repository struct {
	inner *store.FilteredRepository[music.Release]
}

var _ ports.MusicReleaseRepository = (*Repository)(nil)

// New constructs a named ports.MusicReleaseRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner}, nil
}

func idOf(r *music.Release) string { return r.ID }

// indexOf writes GroupID/LibraryEntryID unconditionally — both are required
// fields on music.Release — and MBID/Barcode only when non-empty, since both
// are optional and indexing an empty value would collide every un-barcoded
// (or MBID-less) release under one index entry.
func indexOf(r *music.Release) map[string]string {
	idx := map[string]string{
		"group_id":         r.GroupID,
		"library_entry_id": r.LibraryEntryID,
	}
	if r.MBID != "" {
		idx["mbid"] = r.MBID
	}
	if r.Barcode != "" {
		idx["barcode"] = r.Barcode
	}
	return idx
}

// Create implements ports.MusicReleaseRepository.
func (r *Repository) Create(ctx context.Context, rel *music.Release) error {
	return r.inner.Create(ctx, rel)
}

// Get implements ports.MusicReleaseRepository.
func (r *Repository) Get(ctx context.Context, id string) (*music.Release, error) {
	return r.inner.Get(ctx, id)
}

// GetByMBID implements ports.MusicReleaseRepository as an indexed
// single-key lookup against the mbid Document.Index entry. Returns
// ports.ErrNotFound if no release has that MBID.
func (r *Repository) GetByMBID(ctx context.Context, mbid string) (*music.Release, error) {
	return r.getByIndex(ctx, "mbid", mbid)
}

// GetByBarcode implements ports.MusicReleaseRepository as an indexed
// single-key lookup against the barcode Document.Index entry. Returns
// ports.ErrNotFound if no release has that barcode.
func (r *Repository) GetByBarcode(ctx context.Context, barcode string) (*music.Release, error) {
	return r.getByIndex(ctx, "barcode", barcode)
}

func (r *Repository) getByIndex(ctx context.Context, key, value string) (*music.Release, error) {
	releases, _, err := r.inner.List(ctx, map[string]string{key: value}, 1, "")
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, ports.ErrNotFound
	}
	return releases[0], nil
}

// Update implements ports.MusicReleaseRepository. Returns ports.ErrNotFound
// if no release with rel.ID exists.
func (r *Repository) Update(ctx context.Context, rel *music.Release) error {
	return r.inner.Update(ctx, rel)
}

// Delete implements ports.MusicReleaseRepository. Returns ports.ErrNotFound
// if no release with id exists.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}

// List implements ports.MusicReleaseRepository. Unfiltered.
func (r *Repository) List(ctx context.Context, pageSize int, pageToken string) ([]*music.Release, string, error) {
	return r.inner.List(ctx, nil, pageSize, pageToken)
}

// ListByGroup implements ports.MusicReleaseRepository as an indexed filter
// on the group_id Document.Index entry.
func (r *Repository) ListByGroup(ctx context.Context, groupID string, pageSize int, pageToken string) ([]*music.Release, string, error) {
	return r.inner.List(ctx, map[string]string{"group_id": groupID}, pageSize, pageToken)
}

// ListByEntry implements ports.MusicReleaseRepository as an indexed filter
// on the library_entry_id Document.Index entry.
func (r *Repository) ListByEntry(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) ([]*music.Release, string, error) {
	return r.inner.List(ctx, map[string]string{"library_entry_id": libraryEntryID}, pageSize, pageToken)
}
