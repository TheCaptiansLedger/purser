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
	"encoding/json"
	"fmt"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"strconv"
)

const collection = "music_release"

// itemCollection is the shared "item" collection internal/adapters/store/item
// also writes to — ListTracksByRelease reads it directly via the raw
// datastore.Datastore rather than through ports.ItemRepository, per
// docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage" section.
const itemCollection = "item"

// innerListPageSize bounds each internal (non-caller-facing) List call
// ListTracksByRelease issues while draining a Release Group's tracks — see
// ListTracksByRelease's doc comment.
const innerListPageSize = 100

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
	ds    datastore.Datastore
}

var _ ports.MusicReleaseRepository = (*Repository)(nil)

// New constructs a named ports.MusicReleaseRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}
	return &Repository{inner: inner, ds: ds}, nil
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

// ListTracksByRelease implements ports.MusicReleaseRepository. Track ↔
// Release linkage (Item.Metadata["release_id"]) is owned by this
// repository, not ports.ItemRepository — see
// docs/adr/0021-music-domain-model.md. It reads releaseID's own GroupID,
// then drains every Item already indexed under that group_id (an existing,
// not music-specific, Document.Index entry internal/adapters/store/item
// writes for every content type), and narrows to this one release by
// Metadata["release_id"] in memory — the same "indexed pre-filter, then
// in-memory refine" shape internal/service.AfterDarkBrowseService already
// uses for its own bounded fan-out, here bounded by one Release Group's
// track count across all its editions, not the whole catalog. This
// deliberately avoids adding a release-ID filter to ports.ItemRepository
// (docs/adr/0021-music-domain-model.md self-audit item 3) or changing
// internal/adapters/store/item's indexing (which would leak Music-specific
// knowledge into a shared kernel adapter).
func (r *Repository) ListTracksByRelease(ctx context.Context, releaseID string, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	rel, err := r.Get(ctx, releaseID)
	if err != nil {
		return nil, "", err
	}

	var matches []*domain.Item
	innerToken := ""
	for {
		docs, next, err := r.ds.List(ctx, itemCollection, map[string]string{"group_id": rel.GroupID}, innerListPageSize, innerToken)
		if err != nil {
			return nil, "", err
		}
		for _, doc := range docs {
			var item domain.Item
			if err := json.Unmarshal(doc.Data, &item); err != nil {
				return nil, "", fmt.Errorf("adapters/store/music: unmarshal item %q: %w", doc.ID, err)
			}
			if releaseIDOf(&item) == releaseID {
				matches = append(matches, &item)
			}
		}
		if next == "" {
			break
		}
		innerToken = next
	}

	return paginateItems(matches, pageSize, pageToken)
}

// releaseIDOf reads Item.Metadata["release_id"] as a string, or "" if
// absent — Metadata is map[string]any (a google.protobuf.Struct on the
// wire), so a missing key or a non-string value both read as "".
func releaseIDOf(i *domain.Item) string {
	v, _ := i.Metadata["release_id"].(string)
	return v
}

// paginateItems applies this repository's own offset-encoded page token
// over an already-assembled, in-memory slice — the same documented
// simplification internal/service.AfterDarkBrowseService's paginateSlice
// uses for its own bounded, composed fan-out. The page token is a base-10
// offset, still opaque to callers (they only ever round-trip it verbatim).
func paginateItems(items []*domain.Item, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	offset := 0
	if pageToken != "" {
		o, err := strconv.Atoi(pageToken)
		if err != nil || o < 0 {
			return nil, "", fmt.Errorf("adapters/store/music: invalid page token %q", pageToken)
		}
		offset = o
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if offset >= len(items) {
		return []*domain.Item{}, "", nil
	}
	end := min(offset+pageSize, len(items))
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	return items[offset:end], next, nil
}
