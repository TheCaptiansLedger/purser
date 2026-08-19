// Package music is the datastore-backed adapter for the
// ports.MusicReleaseRepository port. Get/Update/Delete/List/ListByGroup/
// ListByEntry/GetByBarcode/ListTracksByRelease stay a thin wrapper over the
// shared store.FilteredRepository[T] translator, the same pattern
// internal/adapters/store/group and internal/adapters/store/image use.
// Create is hand-written: it must additionally enforce get-or-create
// uniqueness on MBID (when set), via a music_release_mbid reservation
// collection — the same mechanism internal/adapters/store/tag (ADR-0019)
// and internal/adapters/store/externalid (ADR-0026) already use, scoped to
// MBID only per docs/technical/pipeline-music-persist.md ("MusicRelease's
// own reservation-document fix" — Barcode stays a plain, unenforced index).
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
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	collection = "music_release"

	// reservationCollection holds one document per unique, non-empty MBID,
	// pointing at the Release that currently owns it — see
	// docs/technical/pipeline-music-persist.md's "MusicRelease's own
	// reservation-document fix".
	reservationCollection = "music_release_mbid"

	instrumentationName = "purser/internal/adapters/store/music"
)

// reservation is the payload stored in a reservationCollection document.
type reservation struct {
	ReleaseID string `json:"release_id"`
}

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

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
}

var _ ports.MusicReleaseRepository = (*Repository)(nil)

// New constructs a named ports.MusicReleaseRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	inner, err := store.NewFiltered(name, collection, ds, idOf, indexOf, opts...)
	if err != nil {
		return nil, err
	}

	r := &Repository{
		inner:  inner,
		ds:     ds,
		logger: inner.Logger().With("component", "adapters.store."+reservationCollection),
		tracer: inner.Tracer(),
	}

	meter := inner.MeterProvider().Meter(instrumentationName)
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store/music: creating creates counter: %w", err)
	}

	return r, nil
}

func idOf(r *music.Release) string { return r.ID }

// reservationID is the reservation document's ID for a given MBID.
// MusicRelease's get-or-create identity is a single field (unlike Tag's
// (Scope, Key, Value) or ExternalID's (EntityType, Source, Value)), so no
// \x00-delimited composite join is needed — the MBID itself is the key.
func reservationID(mbid string) string { return mbid }

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

// Create implements ports.MusicReleaseRepository as get-or-create on
// rel.MBID when rel.MBID is non-empty — see
// docs/technical/pipeline-music-persist.md's "MusicRelease's own
// reservation-document fix". A stub release with no MBID yet
// (ReleaseStatusStub) is created plainly, with no reservation — the same
// "never reserve an empty identity value" rule Tag/ExternalID follow. If a
// release with that MBID already exists, rel is mutated in place to the
// pre-existing release (its incoming ID/fields are discarded) and Create
// returns nil; a genuinely new MBID is created as given.
func (r *Repository) Create(ctx context.Context, rel *music.Release) error {
	if rel.MBID == "" {
		return r.inner.Create(ctx, rel)
	}

	ctx, span := r.tracer.Start(ctx, "music_release_repository.create", trace.WithAttributes(
		attribute.String("music_release.id", rel.ID), attribute.String("music_release.mbid", rel.MBID),
	))
	defer span.End()

	if err := r.createOnce(ctx, rel); err == nil {
		r.creates.Add(ctx, 1)
		r.logger.DebugContext(ctx, "music release created", "music_release.id", rel.ID)
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	existing, found, err := r.resolveIdentity(ctx, rel.MBID)
	if err != nil {
		return err
	}
	if found {
		span.SetAttributes(attribute.Bool("music_release.get_or_create_hit", true))
		*rel = *existing
		return nil
	}

	// The reservation wasn't the cause of the conflict (or was stale and
	// has now been cleared) — retry once; a second conflict means the
	// caller's own ID collides with an unrelated release.
	if err := r.createOnce(ctx, rel); err != nil {
		return err
	}
	r.creates.Add(ctx, 1)
	r.logger.DebugContext(ctx, "music release created", "music_release.id", rel.ID)
	return nil
}

// createOnce atomically writes rel's reservation and MusicRelease documents
// via CreateBatch — the race-safe, constraint-based conflict check
// docs/adr/0012-datastore-persistence.md establishes, not a
// read-then-insert.
func (r *Repository) createOnce(ctx context.Context, rel *music.Release) error {
	data, err := json.Marshal(rel)
	if err != nil {
		return fmt.Errorf("adapters/store/music: marshal release %s: %w", rel.ID, err)
	}
	resData, err := json.Marshal(reservation{ReleaseID: rel.ID})
	if err != nil {
		return fmt.Errorf("adapters/store/music: marshal reservation for release %s: %w", rel.ID, err)
	}

	return r.ds.CreateBatch(ctx, []datastore.Document{
		{Collection: reservationCollection, ID: reservationID(rel.MBID), Data: resData},
		{Collection: collection, ID: r.inner.DocumentID(rel), Data: data, Index: r.inner.IndexOf(rel)},
	})
}

// resolveIdentity reports whether a live release currently owns mbid.
// GetByMBID's own indexed lookup is the authoritative answer — Document.Index
// isn't itself uniqueness-enforcing (only the reservation collection's
// primary key is), but once the reservation mechanism guards every Create,
// at most one release can ever be indexed under a given MBID, so a List hit
// here is always exactly the reservation's rightful owner, no separate
// payload read needed. A miss means either no release ever owned mbid, or
// one did and was deleted (or its MBID changed) without the reservation
// being cleaned up — either way, any stale reservation for mbid is cleared
// so a retry isn't blocked. See
// docs/technical/pipeline-music-persist.md.
func (r *Repository) resolveIdentity(ctx context.Context, mbid string) (*music.Release, bool, error) {
	existing, err := r.GetByMBID(ctx, mbid)
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return nil, false, err
	}

	r.deleteStaleReservation(ctx, reservationID(mbid))
	return nil, false, nil
}

func (r *Repository) deleteStaleReservation(ctx context.Context, id string) {
	if err := r.ds.Delete(ctx, reservationCollection, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		r.logger.WarnContext(ctx, "music: failed to clean up a stale/orphaned MBID reservation", "reservation.id", id, "error", err)
	}
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

// Update implements ports.MusicReleaseRepository. If rel.MBID differs from
// the existing release's MBID, the reservation is moved under the same
// uniqueness constraint Create enforces, returning ports.ErrConflict if the
// new MBID is already owned by a different, live release — the same
// "Update may still change the identity field" fix
// docs/adr/0026-external-id-get-or-create.md applies to ExternalID.Update,
// applied here since UpdateMusicRelease's field mask doesn't exclude mbid.
// Returns ports.ErrNotFound if no release with rel.ID exists.
func (r *Repository) Update(ctx context.Context, rel *music.Release) error {
	ctx, span := r.tracer.Start(ctx, "music_release_repository.update", trace.WithAttributes(attribute.String("music_release.id", rel.ID)))
	defer span.End()

	existing, err := r.inner.Get(ctx, rel.ID)
	if err != nil {
		return err
	}

	if existing.MBID == rel.MBID {
		return r.inner.Update(ctx, rel)
	}

	if rel.MBID != "" {
		if err := r.reserve(ctx, rel); err != nil {
			return err
		}
	}

	if err := r.inner.Update(ctx, rel); err != nil {
		return err
	}

	// Best-effort: a crash here leaves the old MBID's reservation orphaned,
	// blocking reuse of that identity until the next Create/Update against
	// it self-heals via resolveIdentity. See
	// docs/technical/pipeline-music-persist.md.
	if existing.MBID != "" {
		r.deleteStaleReservation(ctx, reservationID(existing.MBID))
	}
	r.logger.DebugContext(ctx, "music release updated", "music_release.id", rel.ID)
	return nil
}

// reserve creates the reservation for rel.MBID, self-healing a stale
// reservation and retrying once if the first attempt conflicts with one.
// Returns ports.ErrConflict if a different, live release genuinely owns
// that MBID.
func (r *Repository) reserve(ctx context.Context, rel *music.Release) error {
	data, err := json.Marshal(reservation{ReleaseID: rel.ID})
	if err != nil {
		return fmt.Errorf("adapters/store/music: marshal reservation for release %s: %w", rel.ID, err)
	}
	doc := datastore.Document{Collection: reservationCollection, ID: reservationID(rel.MBID), Data: data}

	if err := r.ds.Create(ctx, doc); err == nil {
		return nil
	} else if !errors.Is(err, ports.ErrConflict) {
		return err
	}

	if _, found, err := r.resolveIdentity(ctx, rel.MBID); err != nil {
		return err
	} else if found {
		return ports.ErrConflict
	}

	return r.ds.Create(ctx, doc)
}

// Delete implements ports.MusicReleaseRepository. The deleted release's
// MBID reservation, if any, is cleaned up best-effort after the release
// document itself is removed — see
// docs/technical/pipeline-music-persist.md.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "music_release_repository.delete", trace.WithAttributes(attribute.String("music_release.id", id)))
	defer span.End()

	existing, err := r.inner.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := r.inner.Delete(ctx, id); err != nil {
		return err
	}

	if existing.MBID != "" {
		r.deleteStaleReservation(ctx, reservationID(existing.MBID))
	}
	return nil
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

// CreateTrack implements ports.MusicReleaseRepository. It resolves
// releaseID's GroupID/LibraryEntryID, stamps them plus
// Metadata["release_id"] onto track, and writes track directly into the
// shared itemCollection — the same collection ListTracksByRelease already
// reads from — rather than through ports.ItemRepository, per
// docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage" section.
// track.ID must already be assigned by the caller. Returns
// ports.ErrNotFound if releaseID doesn't exist, or ports.ErrConflict if
// track.ID already exists.
func (r *Repository) CreateTrack(ctx context.Context, releaseID string, track *domain.Item) error {
	ctx, span := r.tracer.Start(ctx, "music_release_repository.create_track", trace.WithAttributes(
		attribute.String("music_release.id", releaseID), attribute.String("item.id", track.ID),
	))
	defer span.End()

	rel, err := r.Get(ctx, releaseID)
	if err != nil {
		return err
	}

	track.GroupID = rel.GroupID
	track.LibraryEntryID = rel.LibraryEntryID
	if track.Metadata == nil {
		track.Metadata = map[string]any{}
	}
	track.Metadata["release_id"] = releaseID

	data, err := json.Marshal(track)
	if err != nil {
		return fmt.Errorf("adapters/store/music: marshal track %s: %w", track.ID, err)
	}

	if err := r.ds.Create(ctx, datastore.Document{
		Collection: itemCollection,
		ID:         track.ID,
		Data:       data,
		Index:      itemIndexOf(track),
	}); err != nil {
		return err
	}

	r.logger.DebugContext(ctx, "music release track created", "music_release.id", releaseID, "item.id", track.ID)
	return nil
}

// itemIndexOf mirrors internal/adapters/store/item's own indexOf exactly —
// duplicated here, not imported (that function is unexported), because
// CreateTrack writes directly into the shared "item" collection instead of
// going through ports.ItemRepository. Keep this in sync with
// internal/adapters/store/item/item.go's indexOf if that shape ever
// changes — a drift here would make tracks created through this path
// invisible to ItemRepository.List's library_entry_id/content_type/status
// filters.
func itemIndexOf(i *domain.Item) map[string]string {
	return map[string]string{
		"library_entry_id": i.LibraryEntryID,
		"content_type":     string(i.ContentType),
		"group_id":         i.GroupID,
		"status":           string(i.Status),
	}
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
