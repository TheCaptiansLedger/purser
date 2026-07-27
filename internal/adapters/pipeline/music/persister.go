package music

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Persister implements ports.Persister for domain.ContentTypeMusic — the
// artist -> release group -> release -> item -> media-file cascade given a
// winning domain.MatchCandidate whose ExternalRef is a MusicBrainz release
// MBID. See docs/technical/pipeline-music-persist.md.
type Persister struct {
	mb             ports.MusicBrainzClient
	externalIDs    ports.ExternalIDRepository
	libraryEntries ports.LibraryEntryRepository
	groups         ports.GroupRepository
	releases       ports.MusicReleaseRepository
	items          ports.ItemRepository
	mediaFiles     ports.MediaFileRepository

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.Persister = (*Persister)(nil)

// NewPersister constructs a Persister backed by mb and the given
// repositories. Reuses the same Option/WithLogger/WithTracerProvider used
// by New/NewIdentifier/NewConfidenceScorer — all four share the same
// {logger, tracerProvider} shape.
func NewPersister(
	mb ports.MusicBrainzClient,
	externalIDs ports.ExternalIDRepository,
	libraryEntries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	releases ports.MusicReleaseRepository,
	items ports.ItemRepository,
	mediaFiles ports.MediaFileRepository,
	opts ...Option,
) *Persister {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Persister{
		mb:             mb,
		externalIDs:    externalIDs,
		libraryEntries: libraryEntries,
		groups:         groups,
		releases:       releases,
		items:          items,
		mediaFiles:     mediaFiles,
		logger:         o.logger.With("component", "adapters.pipeline.music.persister"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.Persister.
func (p *Persister) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// Persist implements ports.Persister. fingerprint is unused: everything
// this cascade needs comes from the resolved MusicBrainz release itself
// (candidate.ExternalRef), not the group's consensus Fingerprint — the
// parameter exists only to satisfy the shared, content-type-agnostic
// Persister signature. No cross-repository transaction backs this: each
// step below is look-up-or-create, so a retry after a partial failure
// picks up wherever the previous attempt left off, per
// docs/technical/pipeline-music-persist.md's "No true cross-repository
// transaction" section.
func (p *Persister) Persist(ctx context.Context, _ *domain.Fingerprint, candidate domain.MatchCandidate, files []*domain.UnmatchedFile) error {
	ctx, span := p.tracer.Start(ctx, "music.persister.persist", trace.WithAttributes(
		attribute.String("music.external_ref", candidate.ExternalRef),
	))
	defer span.End()

	mbRelease, err := p.mb.LookupRelease(ctx, candidate.ExternalRef)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/music: looking up release %s: %w", candidate.ExternalRef, err)
	}
	if mbRelease.ReleaseGroup == nil {
		return fmt.Errorf("adapters/pipeline/music: release %s has no release group", candidate.ExternalRef)
	}
	if len(mbRelease.ArtistCredit) == 0 {
		return fmt.Errorf("adapters/pipeline/music: release %s has no artist credit", candidate.ExternalRef)
	}

	artistMBID := mbRelease.ArtistCredit[0].Artist.ID
	artistID, err := p.getOrCreateArtist(ctx, artistMBID)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/music: resolving artist %s: %w", artistMBID, err)
	}

	groupID, err := p.getOrCreateReleaseGroup(ctx, mbRelease.ReleaseGroup, artistID)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/music: resolving release group %s: %w", mbRelease.ReleaseGroup.ID, err)
	}

	release, err := p.getOrCreateRelease(ctx, mbRelease, groupID, artistID)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/music: resolving release %s: %w", candidate.ExternalRef, err)
	}

	allMatched, err := p.persistTracks(ctx, release, artistID, mbRelease, files)
	if err != nil {
		return fmt.Errorf("adapters/pipeline/music: persisting tracks for release %s: %w", release.ID, err)
	}

	wantStatus := music.ReleaseStatusPartial
	if allMatched {
		wantStatus = music.ReleaseStatusImported
	}
	if release.Status != wantStatus {
		release.Status = wantStatus
		if err := p.releases.Update(ctx, release); err != nil {
			return fmt.Errorf("adapters/pipeline/music: updating release %s status: %w", release.ID, err)
		}
	}

	p.logger.InfoContext(ctx, "music release persisted",
		"music_release.id", release.ID, "music_release.mbid", release.MBID, "music_release.status", release.Status)
	return nil
}

// resolveOrCreate implements ADR-0026's speculative-create-then-reconcile
// pattern generically: GetByValue hit returns immediately. On a miss,
// createSpeculative builds and persists a new entity via its own
// repository, then externalIDs.Create links it to mbid. If that Create's
// get-or-create mutates the link to a different EntityID (this caller lost
// a concurrent race), the just-created speculative entity is deleted via
// deleteByID and the winner's ID is returned instead — see
// docs/adr/0026-external-id-get-or-create.md.
func (p *Persister) resolveOrCreate(
	ctx context.Context,
	entityType domain.EntityType,
	mbid string,
	createSpeculative func(ctx context.Context) (id string, err error),
	deleteByID func(ctx context.Context, id string) error,
) (string, error) {
	existing, err := p.externalIDs.GetByValue(ctx, entityType, domain.ExternalIDSourceMBZ, mbid)
	if err == nil {
		return existing.EntityID, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return "", err
	}

	id, err := createSpeculative(ctx)
	if err != nil {
		return "", err
	}

	link := &domain.ExternalID{EntityType: entityType, EntityID: id, Source: domain.ExternalIDSourceMBZ, Value: mbid}
	if err := p.externalIDs.Create(ctx, link); err != nil {
		return "", err
	}
	if link.EntityID != id {
		if delErr := deleteByID(ctx, id); delErr != nil {
			return "", fmt.Errorf("adapters/pipeline/music: cleaning up speculative entity %s after losing get-or-create race: %w", id, delErr)
		}
		return link.EntityID, nil
	}
	return id, nil
}

// getOrCreateArtist resolves mbid to a LibraryEntry.ID, per
// docs/adr/0021-music-domain-model.md's Artist mapping — get-or-create via
// ExternalID(library_entry, mbz, mbid), same mechanism for Various Artists
// as any other artist (no separate seed step, per ADR-0021).
func (p *Persister) getOrCreateArtist(ctx context.Context, mbid string) (string, error) {
	return p.resolveOrCreate(ctx, domain.EntityTypeLibraryEntry, mbid, func(ctx context.Context) (string, error) {
		artist, err := p.mb.LookupArtist(ctx, mbid)
		if err != nil {
			return "", err
		}
		entry := &domain.LibraryEntry{
			ID:          domain.NewID(),
			ContentType: domain.ContentTypeMusic,
			Kind:        domain.KindArtist,
			Name:        artist.Name,
			SortName:    artist.SortName,
			MonitorMode: domain.MonitorModeAll,
			Metadata:    artistMetadata(artist),
		}
		if err := entry.Validate(); err != nil {
			return "", err
		}
		if err := p.libraryEntries.Create(ctx, entry); err != nil {
			return "", err
		}
		return entry.ID, nil
	}, p.libraryEntries.Delete)
}

// getOrCreateReleaseGroup resolves rg to a Group.ID, per ADR-0021's Release
// Group mapping — get-or-create via ExternalID(group, mbz, rg.ID).
func (p *Persister) getOrCreateReleaseGroup(ctx context.Context, rg *ports.ReleaseGroup, artistID string) (string, error) {
	return p.resolveOrCreate(ctx, domain.EntityTypeGroup, rg.ID, func(ctx context.Context) (string, error) {
		group := &domain.Group{
			ID:             domain.NewID(),
			LibraryEntryID: artistID,
			Title:          rg.Title,
			MonitorMode:    domain.MonitorModeAll,
			Metadata:       map[string]any{"album_type": albumType(rg)},
		}
		if err := group.Validate(); err != nil {
			return "", err
		}
		if err := p.groups.Create(ctx, group); err != nil {
			return "", err
		}
		return group.ID, nil
	}, p.groups.Delete)
}

// getOrCreateRelease resolves mbRelease to a music.Release — Create is
// itself get-or-create on MBID (this branch's own reservation-document
// fix, internal/adapters/store/music), so a hit mutates the built release
// in place to the pre-existing row.
func (p *Persister) getOrCreateRelease(ctx context.Context, mbRelease *ports.Release, groupID, artistID string) (*music.Release, error) {
	mediumCount := len(mbRelease.Media)
	trackCount := 0
	for _, m := range mbRelease.Media {
		trackCount += len(m.Tracks)
	}

	var label, catalogNumber string
	if len(mbRelease.LabelInfo) > 0 {
		label = mbRelease.LabelInfo[0].Label.Name
		catalogNumber = mbRelease.LabelInfo[0].CatalogNumber
	}
	var format string
	if len(mbRelease.Media) > 0 {
		format = mbRelease.Media[0].Format
	}

	// IsDefault (MBZ's canonical release for the release group) needs
	// comparing against every release in the group
	// (ListReleasesForReleaseGroup), not derivable from this single
	// LookupRelease call — left false, same "not asserted correct on
	// first pass" caveat this codebase already carries for other
	// starting values.
	rel := &music.Release{
		ID:             domain.NewID(),
		GroupID:        groupID,
		LibraryEntryID: artistID,
		Title:          mbRelease.Title,
		Country:        mbRelease.Country,
		Date:           parseMBDate(mbRelease.Date),
		Label:          label,
		CatalogNumber:  catalogNumber,
		Barcode:        mbRelease.Barcode,
		Format:         format,
		MediumCount:    mediumCount,
		TrackCount:     trackCount,
		Monitored:      true,
		Status:         music.ReleaseStatusPartial,
		MBID:           mbRelease.ID,
	}
	if err := rel.Validate(); err != nil {
		return nil, err
	}
	if err := p.releases.Create(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// trackKey identifies one tracklist position — the idempotency key
// persistTracks uses to recognize a track already created on a prior
// attempt, per docs/technical/pipeline-music-persist.md's step 6.
type trackKey struct {
	disc int
	seq  string
}

// tracksPageSize bounds each ListTracksByRelease call persistTracks issues
// while draining a release's already-persisted tracks.
const tracksPageSize = 100

// persistTracks implements step 6 of the cascade: for each (medium, track)
// in mbRelease's official tracklist, create the matching Item/MediaFile
// pair, or a stub Item{Status: missing} if no file matches yet.
//
// Every Item is created (or found) with Status=missing first, and only
// flipped to imported after its MediaFile is successfully created — never
// the other way around. A "stub with no file" and "Item whose MediaFile
// create failed last attempt" are therefore the exact same, indistinguishable
// state, which is what makes resuming correct: existingTracksByPosition
// recognizes either case identically, and a retry just re-attempts finding
// a file and creating the MediaFile, same as a genuine first attempt at
// that position. Setting Status=imported before the MediaFile existed
// would have made a retry skip the position forever (item.Status ==
// imported already) despite it having no backing file — the gap design
// review flagged, guarded here structurally, not just by the GetByHash
// check below (which additionally covers the file itself already having a
// MediaFile from a prior attempt, independent of the Item's own status).
// Returns whether every tracklist entry ended up Status=imported.
func (p *Persister) persistTracks(ctx context.Context, release *music.Release, artistID string, mbRelease *ports.Release, files []*domain.UnmatchedFile) (allMatched bool, err error) {
	existing, err := p.existingTracksByPosition(ctx, release.ID)
	if err != nil {
		return false, err
	}

	allMatched = true
	for _, medium := range mbRelease.Media {
		for _, track := range medium.Tracks {
			key := trackKey{disc: medium.Position, seq: track.Number}
			matched, err := p.persistOneTrack(ctx, release, artistID, medium, track, existing[key], files)
			if err != nil {
				return false, err
			}
			if !matched {
				allMatched = false
			}
		}
	}
	return allMatched, nil
}

// persistOneTrack handles one (medium, track) tracklist position against
// existingItem (nil if no Item is linked to this position yet, per
// existingTracksByPosition). Returns whether this position ended up
// Status=imported.
func (p *Persister) persistOneTrack(ctx context.Context, release *music.Release, artistID string, medium ports.Medium, track ports.Track, existingItem *domain.Item, files []*domain.UnmatchedFile) (matched bool, err error) {
	if existingItem != nil && existingItem.Status == domain.ItemStatusImported {
		return true, nil
	}

	uf := findUnmatchedFile(files, medium.Position, track.Number)
	if uf == nil {
		if existingItem == nil {
			if err := p.items.Create(ctx, buildTrackItem(release, artistID, medium, track)); err != nil {
				return false, err
			}
		}
		return false, nil
	}

	// A MediaFile for this exact file may already exist from a prior
	// attempt that got this far before a later track in the loop failed —
	// reconcile the Item it belongs to (its own status-flip Update may be
	// what failed) instead of assuming "MediaFile exists" already implies
	// "Item.Status is imported."
	if mf, err := p.mediaFiles.GetByHash(ctx, uf.OSHash, uf.SHA1, uf.MD5, uf.SHA512); err == nil {
		return p.ensureItemImported(ctx, mf.ItemID)
	} else if !errors.Is(err, ports.ErrNotFound) {
		return false, err
	}

	return true, p.createOrAttachTrack(ctx, release, artistID, medium, track, existingItem, uf)
}

// createOrAttachTrack reuses existingItem (a stub from a prior attempt) or
// creates a new Item, links its Recording MBID, creates the MediaFile
// linking it to uf, and flips its Status to imported.
func (p *Persister) createOrAttachTrack(ctx context.Context, release *music.Release, artistID string, medium ports.Medium, track ports.Track, existingItem *domain.Item, uf *domain.UnmatchedFile) error {
	item := existingItem
	if item == nil {
		item = buildTrackItem(release, artistID, medium, track)
		if err := item.Validate(); err != nil {
			return err
		}
		if err := p.items.Create(ctx, item); err != nil {
			return err
		}
	}

	if track.Recording != nil && track.Recording.ID != "" {
		if err := p.linkRecording(ctx, item.ID, track.Recording.ID); err != nil {
			return err
		}
	}

	mf := &domain.MediaFile{
		ID: domain.NewID(), ItemID: item.ID, Path: uf.Path, Size: uf.Size,
		OSHash: uf.OSHash, MD5: uf.MD5, SHA1: uf.SHA1, SHA512: uf.SHA512,
	}
	if err := mf.Validate(); err != nil {
		return err
	}
	if err := p.mediaFiles.Create(ctx, mf); err != nil {
		return err
	}

	item.Status = domain.ItemStatusImported
	return p.items.Update(ctx, item)
}

// ensureItemImported flips itemID's Status to imported if it isn't
// already — used when a MediaFile is found to already exist for an Item
// whose own status-flip Update apparently failed on a prior attempt.
func (p *Persister) ensureItemImported(ctx context.Context, itemID string) (bool, error) {
	item, err := p.items.Get(ctx, itemID)
	if err != nil {
		return false, err
	}
	if item.Status == domain.ItemStatusImported {
		return true, nil
	}
	item.Status = domain.ItemStatusImported
	if err := p.items.Update(ctx, item); err != nil {
		return false, err
	}
	return true, nil
}

// linkRecording creates the ExternalID(item, mbz_recording, recordingMBID)
// link for a track Item — idempotent (ADR-0026 get-or-create), safe to
// call again for an Item reused from a prior partial attempt.
func (p *Persister) linkRecording(ctx context.Context, itemID, recordingMBID string) error {
	link := &domain.ExternalID{EntityType: domain.EntityTypeItem, EntityID: itemID, Source: domain.ExternalIDSourceMBZRecording, Value: recordingMBID}
	if err := link.Validate(); err != nil {
		return err
	}
	return p.externalIDs.Create(ctx, link)
}

// existingTracksByPosition drains every Item already linked to release
// (via ListTracksByRelease) into a map keyed by (disc_number, Sequence) —
// disc_number round-trips through real storage as float64 (Item.Metadata
// is map[string]any, JSON-backed), so discNumberOf handles both the
// freshly-set int (same-process retry within one Persist call never
// happens, but defensive either way) and the decoded float64.
func (p *Persister) existingTracksByPosition(ctx context.Context, releaseID string) (map[trackKey]*domain.Item, error) {
	out := make(map[trackKey]*domain.Item)
	pageToken := ""
	for {
		items, next, err := p.releases.ListTracksByRelease(ctx, releaseID, tracksPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			out[trackKey{disc: discNumberOf(item), seq: item.Sequence}] = item
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

func discNumberOf(i *domain.Item) int {
	switch v := i.Metadata["disc_number"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// findUnmatchedFile returns the UnmatchedFile in files whose (DiscNumber,
// TrackNumber) directly matches (discNumber, trackNumber) — MusicBrainz's
// own (medium.position int, track.number string) shape needs no
// translation on either side, per
// docs/technical/pipeline-music-persist.md's step 6.
func findUnmatchedFile(files []*domain.UnmatchedFile, discNumber int, trackNumber string) *domain.UnmatchedFile {
	for _, f := range files {
		if f.DiscNumber == discNumber && f.TrackNumber == trackNumber {
			return f
		}
	}
	return nil
}

// buildTrackItem constructs the Item a tracklist entry maps to, per
// ADR-0021's Track mapping — LibraryEntryID is the resolved Artist's
// LibraryEntry.ID (not derivable from the MusicBrainz tracklist data
// itself, denormalized the same way Group.LibraryEntryID already is).
// Always built Status=missing — see persistTracks' doc comment for why the
// caller, never this constructor, is responsible for flipping it to
// imported, and only after the MediaFile it backs actually exists.
func buildTrackItem(release *music.Release, artistID string, medium ports.Medium, track ports.Track) *domain.Item {
	metadata := map[string]any{
		"release_id":  release.ID,
		"disc_number": medium.Position,
	}
	if track.Recording != nil && len(track.Recording.ISRCs) > 0 {
		metadata["isrc"] = track.Recording.ISRCs[0]
	}

	return &domain.Item{
		ID:             domain.NewID(),
		ContentType:    domain.ContentTypeMusic,
		LibraryEntryID: artistID,
		GroupID:        release.GroupID,
		Title:          track.Title,
		Sequence:       track.Number,
		RuntimeSeconds: track.Length / 1000,
		Status:         domain.ItemStatusMissing,
		Metadata:       metadata,
	}
}

// albumType maps a MusicBrainz ReleaseGroup to ADR-0021's closed
// album_type vocabulary (studio|live|compilation|ep|single|other) for
// Group.Metadata. Live/Compilation are usually MusicBrainz secondary
// types layered on a primary type of Album, not primary types themselves,
// so secondary types are checked first.
func albumType(rg *ports.ReleaseGroup) string {
	for _, st := range rg.SecondaryTypes {
		switch strings.ToLower(st) {
		case "live":
			return "live"
		case "compilation":
			return "compilation"
		}
	}
	switch strings.ToLower(rg.PrimaryType) {
	case "album":
		return "studio"
	case "ep":
		return "ep"
	case "single":
		return "single"
	case "":
		return ""
	default:
		return "other"
	}
}

// artistMetadata builds LibraryEntry.Metadata from a MusicBrainz Artist,
// per ADR-0021's Artist mapping table — best-effort: MusicBrainz doesn't
// guarantee any of these fields are present, and Metadata is an
// unvalidated bag, so a missing field is simply omitted rather than
// erroring.
func artistMetadata(artist *ports.Artist) map[string]any {
	md := map[string]any{}
	if artist.Type != "" {
		md["artist_type"] = strings.ToLower(artist.Type)
	}
	if len(artist.Aliases) > 0 {
		aliases := make([]string, len(artist.Aliases))
		for i, a := range artist.Aliases {
			aliases[i] = a.Name
		}
		md["aliases"] = aliases
	}
	if len(artist.ISNIs) > 0 {
		md["isni"] = artist.ISNIs[0]
	}

	isPerson := strings.EqualFold(artist.Type, "person")
	if artist.LifeSpan.Begin != "" {
		if isPerson {
			md["born_date"] = artist.LifeSpan.Begin
		} else {
			md["founded_date"] = artist.LifeSpan.Begin
		}
	}
	if artist.LifeSpan.End != "" {
		if isPerson {
			md["died_date"] = artist.LifeSpan.End
		} else {
			md["dissolved_date"] = artist.LifeSpan.End
		}
	}

	for _, rel := range artist.Relations {
		if rel.URL == nil {
			continue
		}
		t := strings.ToLower(rel.Type)
		switch {
		case strings.Contains(t, "official homepage"):
			md["official_url"] = rel.URL.Resource
		case strings.Contains(t, "last.fm"), strings.Contains(t, "lastfm"):
			md["lastfm_url"] = rel.URL.Resource
		case strings.Contains(t, "wikipedia"):
			md["wikipedia_url"] = rel.URL.Resource
		}
	}

	return md
}

// parseMBDate parses a MusicBrainz date string, which may be a full date,
// year-month, or year-only. Returns nil for an empty or unparseable
// string rather than erroring — a release's Date is optional.
func parseMBDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
