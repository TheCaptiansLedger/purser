package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/media"
	"purser/internal/ports"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service fans out metadata searches to all registered sources and handles
// importing search results into the library as domain entities.
type Service struct {
	sources     []ports.MetadataSource
	agg         *Aggregator
	jobs        ports.JobQueue
	entries     ports.LibraryEntryRepository
	groups      ports.GroupRepository
	items       ports.ItemRepository
	people      ports.PersonRepository
	tags        ports.TagRepository
	externalIDs ports.ExternalIDRepository
	mediaPath   string
}

// New constructs a metadata Service wired to the given sources and repositories.
func New(
	sources []ports.MetadataSource,
	jobs ports.JobQueue,
	entries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	items ports.ItemRepository,
	people ports.PersonRepository,
	tags ports.TagRepository,
	externalIDs ports.ExternalIDRepository,
	imageRepo ports.ImageRepository,
	mediaPath string,
) *Service {
	return &Service{
		sources:     sources,
		agg:         NewAggregator(sources, imageRepo),
		jobs:        jobs,
		entries:     entries,
		groups:      groups,
		items:       items,
		people:      people,
		tags:        tags,
		externalIDs: externalIDs,
		mediaPath:   mediaPath,
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchStudios queries all sources that serve contentType and merges the results.
// Errors from individual sources are logged but do not abort the fan-out.
func (s *Service) SearchStudios(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalStudio, error) {
	type result struct {
		studios []*domain.ExternalStudio
	}
	ch := make(chan result, len(s.sources))

	var wg sync.WaitGroup
	for _, src := range s.sources {
		if !servesContentType(src, contentType) {
			continue
		}
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			studios, err := src.SearchStudios(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata search studios failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{studios}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalStudio
	for r := range ch {
		all = append(all, r.studios...)
	}
	return all, nil
}

// SearchPeople queries all sources that serve contentType and merges the results.
func (s *Service) SearchPeople(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalPerson, error) {
	type result struct {
		people []*domain.ExternalPerson
	}
	ch := make(chan result, len(s.sources))

	var wg sync.WaitGroup
	for _, src := range s.sources {
		if contentType != "" && !servesContentType(src, contentType) {
			continue
		}
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			people, err := src.SearchPeople(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata search people failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{people}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalPerson
	for r := range ch {
		all = append(all, r.people...)
	}
	return all, nil
}

// FetchArtistDiscography returns one page of release groups for the artist
// identified by externalID in the named source.
// Returns a ValidationError if source is not registered.
func (s *Service) FetchArtistDiscography(
	ctx context.Context,
	source domain.ExternalIDSource,
	contentType domain.ContentType,
	externalID string,
	page, perPage int,
) ([]*domain.ExternalGroup, int, error) {
	var src ports.MetadataSource
	for _, candidate := range s.sources {
		if candidate.Name() == string(source) {
			src = candidate
			break
		}
	}
	if src == nil {
		return nil, 0, errs.Validation(fmt.Sprintf("unknown metadata source %q", source))
	}
	groups, _, total, err := src.FetchEntryContent(ctx, contentType, externalID, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch artist discography: %w", err)
	}
	return groups, total, nil
}

// ImportAlbumRequest carries the user-selected album to persist.
type ImportAlbumRequest struct {
	Source         domain.ExternalIDSource
	ExternalID     string // release-group MBID
	LibraryEntryID string // the artist's internal entry ID
	Title          string
	Year           int
	Monitored      bool
	MonitorMode    domain.MonitorMode
}

// ImportAlbum adds a single album (and its tracks) to an existing artist entry.
// The operation is idempotent: if a group with this external ID already exists,
// it is returned without modification.
func (s *Service) ImportAlbum(ctx context.Context, req *ImportAlbumRequest) (*domain.Group, error) {
	if id, err := s.externalIDs.FindEntity(ctx, "group", string(req.Source), req.ExternalID); err == nil {
		return s.groups.Get(ctx, id)
	}

	src := s.sourceByName(string(req.Source))
	if src == nil {
		return nil, errs.Validation(fmt.Sprintf("unknown metadata source %q", req.Source))
	}

	entry, err := s.entries.Get(ctx, req.LibraryEntryID)
	if err != nil {
		return nil, fmt.Errorf("import album: load entry: %w", err)
	}

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorAll
	}

	g := &domain.Group{
		ID:             uuid.New().String(),
		LibraryEntryID: req.LibraryEntryID,
		Title:          req.Title,
		SortName:       req.Title,
		Year:           req.Year,
		Monitored:      req.Monitored,
		MonitorMode:    monitorMode,
		ExternalIDs:    []domain.ExternalID{{Source: req.Source, Value: req.ExternalID}},
	}
	if err := s.groups.Save(ctx, g); err != nil {
		return nil, fmt.Errorf("import album: save group: %w", err)
	}

	pending, err := s.collectNewGroupTracks(ctx, src, req.ExternalID, entry.ContentType)
	if err != nil {
		return nil, fmt.Errorf("import album: %w", err)
	}

	if len(pending) == 0 {
		return g, nil
	}

	if err := s.saveGroupTracks(ctx, entry.ContentType, req.LibraryEntryID, g.ID, monitorMode, pending); err != nil {
		return nil, fmt.Errorf("import album: %w", err)
	}

	slog.Info("album.imported", "entry_id", req.LibraryEntryID, "group_id", g.ID, "title", g.Title, "new_tracks", len(pending))
	return g, nil
}

// collectNewGroupTracks pages through FetchGroupContent for a single group and
// returns tracks not already present in the library.
func (s *Service) collectNewGroupTracks(ctx context.Context, src ports.MetadataSource, groupExtID string, contentType domain.ContentType) ([]*domain.ExternalItem, error) {
	const perPage = 100
	var tracks []*domain.ExternalItem
	for page := 1; ; page++ {
		extItems, trackTotal, err := src.FetchGroupContent(ctx, contentType, groupExtID, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("fetch tracks page %d: %w", page, err)
		}
		for _, ei := range extItems {
			if _, findErr := s.externalIDs.FindEntity(ctx, "item", string(ei.Source), ei.ExternalID); findErr == nil {
				continue
			}
			tracks = append(tracks, ei)
		}
		if len(extItems) == 0 || page*perPage >= trackTotal {
			break
		}
	}
	return tracks, nil
}

// saveGroupTracks creates Item records for each track, applying monitor-mode
// logic consistent with RefreshArtist.
func (s *Service) saveGroupTracks(ctx context.Context, contentType domain.ContentType, libraryEntryID, groupID string, monitorMode domain.MonitorMode, tracks []*domain.ExternalItem) error {
	latestIdx := 0
	for i, ei := range tracks {
		if ei.Date.After(tracks[latestIdx].Date) {
			latestIdx = i
		}
	}

	now := time.Now().UTC()
	for i, ei := range tracks {
		monitored := monitoredForMode(monitorMode, now, ei.Date)
		if monitorMode == domain.MonitorLatest {
			monitored = (i == latestIdx)
		}
		itemStatus := domain.StatusWanted
		if !monitored {
			itemStatus = domain.StatusMissing
		}
		item := &domain.Item{
			ID:             uuid.New().String(),
			ContentType:    contentType,
			LibraryEntryID: libraryEntryID,
			GroupID:        groupID,
			Title:          ei.Title,
			Overview:       ei.Overview,
			Date:           ei.Date,
			RuntimeSeconds: ei.RuntimeSecs,
			Monitored:      monitored,
			Status:         itemStatus,
			ExternalIDs:    []domain.ExternalID{{Source: ei.Source, Value: ei.ExternalID}},
			AddedAt:        now,
		}
		if err := s.items.Save(ctx, item); err != nil {
			return fmt.Errorf("save track %q: %w", item.Title, err)
		}
	}
	return nil
}

// sourceByName returns the first registered MetadataSource with the given name,
// or nil if none is found.
func (s *Service) sourceByName(name string) ports.MetadataSource {
	for _, src := range s.sources {
		if src.Name() == name {
			return src
		}
	}
	return nil
}

// ── Import ────────────────────────────────────────────────────────────────────

// ImportStudioRequest carries the (user-reviewed) studio data to persist.
type ImportStudioRequest struct {
	Source           domain.ExternalIDSource
	ExternalID       string
	Name             string
	Overview         string
	ContentType      domain.ContentType
	Kind             domain.Kind // defaults to KindStudio if empty
	Monitored        bool
	MonitorMode      domain.MonitorMode
	AutoImport       bool // when true, enqueue a RefreshStudio job immediately after saving
	ImageURL         string
	WebsiteURL       string
	ParentExternalID string // parent's ID within the same source
	ParentName       string
	ParentImageURL   string
	ParentWebsiteURL string
	// AlbumFilter controls which MusicBrainz release types are imported for artists.
	// Tokens: "studio", "live", "compilation", "ep", "single", "all".
	// Empty defaults to ["studio", "live"].
	AlbumFilter []string
}

// ImportStudioResult holds the persisted studio and, if applicable, its network.
type ImportStudioResult struct {
	Studio  *domain.LibraryEntry
	Network *domain.LibraryEntry // nil if no parent was specified or it already existed
}

// importOrFindNetwork resolves or creates the parent network for an ImportStudioRequest.
// Returns the parent ID, the newly created network (nil if it already existed or not needed), and any error.
func (s *Service) importOrFindNetwork(ctx context.Context, req *ImportStudioRequest) (string, *domain.LibraryEntry, error) {
	src := string(req.Source)

	if req.ParentExternalID != "" {
		if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ParentExternalID); err == nil {
			return id, nil, nil
		} else if !errors.Is(err, errs.ErrNotFound) {
			return "", nil, err
		}
	} else if req.ParentName == "" {
		return "", nil, nil
	}

	networkID := uuid.New().String()
	var imagePath string
	if req.ParentImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, req.ParentImageURL, "entries", networkID)
	}
	meta := map[string]any{}
	if req.ParentWebsiteURL != "" {
		meta["website_url"] = req.ParentWebsiteURL
	}
	extIDs := []domain.ExternalID{}
	if req.ParentExternalID != "" {
		extIDs = []domain.ExternalID{{Source: req.Source, Value: req.ParentExternalID}}
	}
	network := &domain.LibraryEntry{
		ID:          networkID,
		ContentType: req.ContentType,
		Kind:        domain.KindNetwork,
		Name:        req.ParentName,
		SortName:    req.ParentName,
		Status:      domain.EntryStatusActive,
		Monitored:   false,
		MonitorMode: domain.MonitorNone,
		ImagePath:   imagePath,
		Metadata:    meta,
		ExternalIDs: extIDs,
	}
	if err := s.entries.Save(ctx, network); err != nil {
		return "", nil, err
	}
	return network.ID, network, nil
}

// ImportStudio persists an ExternalStudio as a library entry. If the studio
// has a parent network, that network is looked up or created first.
// The operation is idempotent: if an entry with the same external ID already
// exists, it is returned without modification.
func (s *Service) ImportStudio(ctx context.Context, req *ImportStudioRequest) (*ImportStudioResult, error) {
	src := string(req.Source)

	// Idempotency: return existing studio if already imported.
	if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ExternalID); err == nil {
		entry, err := s.entries.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &ImportStudioResult{Studio: entry}, nil
	}

	res := &ImportStudioResult{}

	parentID, network, err := s.importOrFindNetwork(ctx, req)
	if err != nil {
		return nil, err
	}
	res.Network = network

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorLatest
	}

	kind := req.Kind
	if kind == "" {
		kind = domain.KindStudio
	}

	studioID := uuid.New().String()

	var imagePath string
	if req.ImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, req.ImageURL, "entries", studioID)
	}

	meta := buildEntryMeta(req, kind)

	studio := &domain.LibraryEntry{
		ID:          studioID,
		ContentType: req.ContentType,
		Kind:        kind,
		Name:        req.Name,
		SortName:    req.Name,
		Overview:    req.Overview,
		ParentID:    parentID,
		Status:      domain.EntryStatusActive,
		Monitored:   req.Monitored,
		MonitorMode: monitorMode,
		ImagePath:   imagePath,
		Metadata:    meta,
		ExternalIDs: []domain.ExternalID{
			{Source: req.Source, Value: req.ExternalID},
		},
	}
	if err := s.entries.Save(ctx, studio); err != nil {
		return nil, err
	}
	res.Studio = studio

	if req.AutoImport && s.jobs != nil {
		entryID := studio.ID
		if kind == domain.KindArtist {
			if _, err := s.jobs.Submit(ctx, "RefreshArtist", map[string]any{"entry_id": entryID},
				func(ctx context.Context, p ports.ProgressReporter) error {
					return s.RefreshArtist(ctx, entryID, p)
				}); err != nil {
				slog.Warn("auto-import: failed to enqueue RefreshArtist", "entry_id", entryID, "error", err)
			}
		} else {
			if _, err := s.jobs.Submit(ctx, "RefreshStudio", map[string]any{"entry_id": entryID},
				func(ctx context.Context, p ports.ProgressReporter) error {
					return s.RefreshStudio(ctx, entryID, p)
				}); err != nil {
				slog.Warn("auto-import: failed to enqueue RefreshStudio", "entry_id", entryID, "error", err)
			}
		}
	}

	return res, nil
}

// ImportPersonRequest carries the (user-reviewed) person data to persist.
type ImportPersonRequest struct {
	Source      domain.ExternalIDSource
	ExternalID  string
	Name        string
	Aliases     []string
	Overview    string
	Role        domain.PersonRole
	Monitored   bool
	MonitorMode domain.MonitorMode
	ImageURL    string
	Metadata    map[string]any
}

// ImportPerson persists an ExternalPerson as a people record.
// Idempotent: returns the existing record if already imported.
func (s *Service) ImportPerson(ctx context.Context, req *ImportPersonRequest) (*domain.Person, error) {
	src := string(req.Source)

	if id, err := s.externalIDs.FindEntity(ctx, "person", src, req.ExternalID); err == nil {
		return s.people.Get(ctx, id)
	}

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorNone
	}

	personID := uuid.New().String()

	var imagePath string
	if req.ImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, req.ImageURL, "people", personID)
	}

	p := &domain.Person{
		ID:          personID,
		Name:        req.Name,
		SortName:    req.Name,
		Overview:    req.Overview,
		Aliases:     req.Aliases,
		Monitored:   req.Monitored,
		MonitorMode: monitorMode,
		ImagePath:   imagePath,
		Metadata:    req.Metadata,
		ExternalIDs: []domain.ExternalID{
			{Source: req.Source, Value: req.ExternalID},
		},
	}
	if err := s.people.Save(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ── Refresh ───────────────────────────────────────────────────────────────────

// collectNewItems pages through src for the given entry external ID and returns
// external items that do not yet have a corresponding item record.
func (s *Service) collectNewItems(ctx context.Context, src ports.MetadataSource, entryName, srcExtID string, contentType domain.ContentType) ([]*domain.ExternalItem, error) {
	const perPage = 100
	var newExtItems []*domain.ExternalItem

	for page := 1; ; page++ {
		_, extItems, total, err := src.FetchEntryContent(ctx, contentType, srcExtID, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("refresh studio %q: page %d: %w", entryName, page, err)
		}
		for _, ei := range extItems {
			if _, err := s.externalIDs.FindEntity(ctx, "item", string(ei.Source), ei.ExternalID); err == nil {
				continue // already imported
			}
			newExtItems = append(newExtItems, ei)
		}
		if len(extItems) == 0 || page*perPage >= total {
			break
		}
	}
	return newExtItems, nil
}

// RefreshStudio fetches all content for a studio from its external metadata
// source and creates Item records for any scenes not already in the library.
// Cover art, performers, and tags are imported alongside each item.
// Progress is reported through p if non-nil; p is updated once per saved item.
func (s *Service) RefreshStudio(ctx context.Context, entryID string, p ports.ProgressReporter) error {
	entry, err := s.entries.Get(ctx, entryID)
	if err != nil {
		return fmt.Errorf("refresh studio: load entry: %w", err)
	}

	src, srcExtID := s.sourceForEntry(entry)
	if src == nil {
		return fmt.Errorf("refresh studio %q: no configured metadata source has an external ID for this entry", entry.Name)
	}

	newExtItems, err := s.collectNewItems(ctx, src, entry.Name, srcExtID, entry.ContentType)
	if err != nil {
		return err
	}

	if len(newExtItems) == 0 {
		slog.Info("studio.refresh.noop", "entry_id", entryID, "name", entry.Name)
		return nil
	}

	// Seed caches used across all scenes in this refresh.
	personCache := map[string]string{} // "source:extID" → internal person ID
	tagCache := s.loadTagCache(ctx)    // lowercase name → *domain.Tag

	// Pre-scan to find the most recent item index (needed for MonitorLatest).
	// This is a cheap metadata-only pass — no images or DB ops.
	latestIdx := 0
	for i, ei := range newExtItems {
		if ei.Date.After(newExtItems[latestIdx].Date) {
			latestIdx = i
		}
	}

	// Phase 2+3: build, save, and report each item one at a time so that
	// scenes appear in the UI as they are imported rather than all at once.
	now := time.Now().UTC()
	total := len(newExtItems)

	for i, ei := range newExtItems {
		itemID := uuid.New().String()

		var coverPath string
		if ei.ImageURL != "" && s.mediaPath != "" {
			coverPath = s.fetchImage(ctx, ei.ImageURL, "items", itemID)
		}

		monitored := monitoredForMode(entry.MonitorMode, entry.AddedAt, ei.Date)
		if entry.MonitorMode == domain.MonitorLatest {
			monitored = (i == latestIdx)
		}

		itemStatus := domain.StatusWanted
		if !monitored {
			itemStatus = domain.StatusMissing
		}

		item := &domain.Item{
			ID:             itemID,
			ContentType:    entry.ContentType,
			LibraryEntryID: entry.ID,
			Title:          ei.Title,
			Overview:       ei.Overview,
			Date:           ei.Date,
			RuntimeSeconds: ei.RuntimeSecs,
			Monitored:      monitored,
			Status:         itemStatus,
			CoverPath:      coverPath,
			People:         s.resolveItemPeople(ctx, ei.People, personCache),
			Tags:           s.resolveItemTags(ctx, ei.Tags, tagCache),
			ExternalIDs:    []domain.ExternalID{{Source: ei.Source, Value: ei.ExternalID}},
			AddedAt:        now,
		}

		if err := s.items.Save(ctx, item); err != nil {
			return fmt.Errorf("refresh studio %q: save item %q: %w", entry.Name, item.Title, err)
		}
		if p != nil {
			p.Report(i+1, total, item.Title)
		}
	}

	slog.Info("studio.refreshed", "entry_id", entryID, "name", entry.Name, "new_items", total)
	return nil
}

// artistAlbum is the intermediate representation used during RefreshArtist to
// carry both the resolved internal group ID and the external album metadata.
type artistAlbum struct {
	internalID string
	extGroup   *domain.ExternalGroup
}

// artistTrack is a track collected during RefreshArtist, bundled with the
// internal group ID of the album it belongs to.
type artistTrack struct {
	groupID string
	extItem *domain.ExternalItem
}

// RefreshArtist fetches the discography for a music artist and creates Group
// records for albums and Item records for tracks not already in the library.
// Progress is reported through p if non-nil; p is updated once per saved track.
func (s *Service) RefreshArtist(ctx context.Context, entryID string, p ports.ProgressReporter) error {
	entry, err := s.entries.Get(ctx, entryID)
	if err != nil {
		return fmt.Errorf("refresh artist: load entry: %w", err)
	}

	src, srcExtID := s.sourceForEntry(entry)
	if src == nil {
		return fmt.Errorf("refresh artist %q: no configured metadata source has an external ID for this entry", entry.Name)
	}

	albums, err := s.resolveArtistAlbums(ctx, src, entry, srcExtID)
	if err != nil {
		return err
	}

	s.importArtistPeople(ctx, src, entryID, srcExtID)

	if _, aggErr := s.agg.FindByExternalID(ctx, entry.ContentType, srcExtID, entry.ID); aggErr != nil {
		slog.Warn("refresh artist: aggregate images", "entry_id", entryID, "error", aggErr)
	}

	newTracks, err := s.collectArtistTracks(ctx, src, entry.Name, albums, entry.ContentType)
	if err != nil {
		return err
	}

	if len(newTracks) == 0 {
		slog.Info("artist.refresh.noop", "entry_id", entryID, "name", entry.Name)
		return nil
	}

	latestIdx := 0
	for i, tr := range newTracks {
		if tr.extItem.Date.After(newTracks[latestIdx].extItem.Date) {
			latestIdx = i
		}
	}

	now := time.Now().UTC()
	total := len(newTracks)

	for i, tr := range newTracks {
		itemID := uuid.New().String()
		var coverPath string
		if tr.extItem.ImageURL != "" && s.mediaPath != "" {
			coverPath = s.fetchImage(ctx, tr.extItem.ImageURL, "items", itemID)
		}
		monitored := monitoredForMode(entry.MonitorMode, entry.AddedAt, tr.extItem.Date)
		if entry.MonitorMode == domain.MonitorLatest {
			monitored = (i == latestIdx)
		}
		itemStatus := domain.StatusWanted
		if !monitored {
			itemStatus = domain.StatusMissing
		}
		item := &domain.Item{
			ID:             itemID,
			ContentType:    entry.ContentType,
			LibraryEntryID: entry.ID,
			GroupID:        tr.groupID,
			Title:          tr.extItem.Title,
			Overview:       tr.extItem.Overview,
			Date:           tr.extItem.Date,
			RuntimeSeconds: tr.extItem.RuntimeSecs,
			Monitored:      monitored,
			Status:         itemStatus,
			CoverPath:      coverPath,
			ExternalIDs:    []domain.ExternalID{{Source: tr.extItem.Source, Value: tr.extItem.ExternalID}},
			AddedAt:        now,
		}
		if err := s.items.Save(ctx, item); err != nil {
			return fmt.Errorf("refresh artist %q: save track %q: %w", entry.Name, item.Title, err)
		}
		if p != nil {
			p.Report(i+1, total, item.Title)
		}
	}

	slog.Info("artist.refreshed", "entry_id", entryID, "name", entry.Name, "new_tracks", total)
	return nil
}

// importArtistPeople fetches band members from the metadata source and saves
// them as Person records linked to the artist entry via entry_people. Errors
// are logged and suppressed — people import is best-effort and must not block
// the rest of the refresh.
func (s *Service) importArtistPeople(ctx context.Context, src ports.MetadataSource, entryID, srcExtID string) {
	members, err := src.FetchEntryPeople(ctx, srcExtID)
	if err != nil {
		if !errors.Is(err, ports.ErrNotSupported) {
			slog.Warn("refresh artist: fetch members", "entry_id", entryID, "error", err)
		}
		return
	}
	personCache := map[string]string{}
	for _, ep := range members {
		personID := s.resolveOrCreatePerson(ctx, ep, personCache)
		if personID == "" {
			continue
		}
		if saveErr := s.entries.SavePerson(ctx, entryID, domain.EntryPerson{
			PersonID: personID,
			Role:     string(ep.Role),
		}); saveErr != nil {
			slog.Warn("refresh artist: link member", "entry_id", entryID, "person_id", personID, "error", saveErr)
		}
	}
}

// resolveArtistAlbums pages through FetchEntryContent for the artist and
// creates Group records for any album not already in the library. Returns a
// slice of all albums (new and existing) with their internal IDs resolved.
// The album types imported are controlled by entry.Metadata["album_filter"].
func (s *Service) resolveArtistAlbums(ctx context.Context, src ports.MetadataSource, entry *domain.LibraryEntry, srcExtID string) ([]artistAlbum, error) {
	const perPage = 100
	var albums []artistAlbum

	var albumFilter []string
	if raw, ok := entry.Metadata["album_filter"]; ok {
		if slice, ok := raw.([]any); ok {
			for _, v := range slice {
				if sv, ok := v.(string); ok {
					albumFilter = append(albumFilter, sv)
				}
			}
		}
	}

	for page := 1; ; page++ {
		extGroups, _, albumTotal, err := src.FetchEntryContent(ctx, entry.ContentType, srcExtID, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("refresh artist %q: fetch albums page %d: %w", entry.Name, page, err)
		}
		for _, eg := range extGroups {
			if !isImportableAlbum(eg, albumFilter) {
				continue
			}
			if id, findErr := s.externalIDs.FindEntity(ctx, "group", string(eg.Source), eg.ExternalID); findErr == nil {
				albums = append(albums, artistAlbum{internalID: id, extGroup: eg})
				continue
			}
			g := &domain.Group{
				ID:             uuid.New().String(),
				LibraryEntryID: entry.ID,
				Title:          eg.Title,
				SortName:       eg.Title,
				Number:         eg.Number,
				Year:           eg.Year,
				Overview:       eg.Overview,
				Monitored:      true,
				MonitorMode:    domain.MonitorAll,
				ExternalIDs:    []domain.ExternalID{{Source: eg.Source, Value: eg.ExternalID}},
				Metadata:       albumMetadata(eg),
			}
			if err := s.groups.Save(ctx, g); err != nil {
				return nil, fmt.Errorf("refresh artist %q: save album %q: %w", entry.Name, g.Title, err)
			}
			albums = append(albums, artistAlbum{internalID: g.ID, extGroup: eg})
		}
		if len(extGroups) == 0 || page*perPage >= albumTotal {
			break
		}
	}
	return albums, nil
}

// isImportableAlbum reports whether a release group matches the user's filter.
// filter tokens: "studio", "live", "compilation", "ep", "single", "all".
// An empty filter falls back to the legacy default (studio + live).
func isImportableAlbum(eg *domain.ExternalGroup, filter []string) bool {
	if len(filter) == 1 && filter[0] == "all" {
		return true
	}
	if len(filter) == 0 {
		filter = []string{"studio", "live"}
	}
	token := albumFilterToken(eg)
	return slices.Contains(filter, token)
}

// albumFilterToken maps an ExternalGroup's MusicBrainz type pair to one of the
// user-facing filter tokens understood by isImportableAlbum.
func albumFilterToken(eg *domain.ExternalGroup) string {
	switch eg.PrimaryType {
	case "EP":
		return "ep"
	case "Single":
		return "single"
	case "Album":
		for _, s := range eg.SecondaryTypes {
			switch s {
			case "Live":
				return "live"
			case "Compilation":
				return "compilation"
			}
		}
		return "studio"
	default:
		return "other"
	}
}

// buildEntryMeta constructs the metadata map for a new library entry from an
// import request. Kept separate to avoid inflating ImportStudio's cyclomatic
// complexity beyond the project lint threshold.
func buildEntryMeta(req *ImportStudioRequest, kind domain.Kind) map[string]any {
	meta := map[string]any{}
	if req.WebsiteURL != "" {
		meta["website_url"] = req.WebsiteURL
	}
	if kind == domain.KindArtist && len(req.AlbumFilter) > 0 {
		meta["album_filter"] = req.AlbumFilter
	}
	return meta
}

// albumMetadata builds the metadata map persisted with a Group record so that
// the UI can section the discography by release type without re-querying MusicBrainz.
func albumMetadata(eg *domain.ExternalGroup) map[string]any {
	m := map[string]any{"primary_type": eg.PrimaryType}
	if len(eg.SecondaryTypes) > 0 {
		m["secondary_types"] = eg.SecondaryTypes
	}
	return m
}

// collectArtistTracks pages through FetchGroupContent for each album and
// returns tracks not already present in the library. Albums that fail track
// lookup are logged and skipped rather than aborting the entire refresh.
func (s *Service) collectArtistTracks(ctx context.Context, src ports.MetadataSource, entryName string, albums []artistAlbum, contentType domain.ContentType) ([]artistTrack, error) {
	const perPage = 100
	var tracks []artistTrack

	for _, album := range albums {
		for page := 1; ; page++ {
			extItems, trackTotal, err := src.FetchGroupContent(ctx, contentType, album.extGroup.ExternalID, page, perPage)
			if err != nil {
				slog.Warn("refresh artist: fetch tracks", "entry", entryName, "album", album.extGroup.Title, "error", err)
				break
			}
			for _, ei := range extItems {
				if _, err := s.externalIDs.FindEntity(ctx, "item", string(ei.Source), ei.ExternalID); err == nil {
					continue
				}
				tracks = append(tracks, artistTrack{groupID: album.internalID, extItem: ei})
			}
			if len(extItems) == 0 || page*perPage >= trackTotal {
				break
			}
		}
	}
	return tracks, nil
}

// loadTagCache returns a map of lowercase tag name → tag for all existing
// metadata-scoped tags. Used to deduplicate tag creation within a refresh.
func (s *Service) loadTagCache(ctx context.Context) map[string]*domain.Tag {
	cache := map[string]*domain.Tag{}
	if s.tags == nil {
		return cache
	}
	existing, err := s.tags.List(ctx, ports.TagFilter{Scope: domain.TagScopeMetadata})
	if err != nil {
		slog.Warn("refresh studio: load tag cache failed", "error", err)
		return cache
	}
	for _, t := range existing {
		cache[strings.ToLower(t.Name)] = t
	}
	return cache
}

// resolveItemTags looks up or creates a Tag record for each name and returns
// a deduplicated slice. tagCache is updated in-place for reuse across scenes.
func (s *Service) resolveItemTags(ctx context.Context, names []string, tagCache map[string]*domain.Tag) []domain.Tag {
	if s.tags == nil || len(names) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []domain.Tag
	for _, name := range names {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		t, ok := tagCache[key]
		if !ok {
			t = &domain.Tag{Name: name, Scope: domain.TagScopeMetadata}
			if err := s.tags.Save(ctx, t); err != nil {
				slog.Warn("refresh studio: save tag failed", "name", name, "error", err)
				continue
			}
			tagCache[key] = t
		}
		out = append(out, *t)
	}
	return out
}

// resolveItemPeople looks up or creates a Person record for each ExternalPerson
// and returns the ItemPerson slice. personCache (keyed by "source:extID") is
// updated in-place to avoid redundant DB lookups across scenes.
func (s *Service) resolveItemPeople(ctx context.Context, external []*domain.ExternalPerson, personCache map[string]string) []domain.ItemPerson {
	if len(external) == 0 {
		return nil
	}
	var out []domain.ItemPerson
	for _, ep := range external {
		personID := s.resolveOrCreatePerson(ctx, ep, personCache)
		if personID == "" {
			continue
		}
		role := ep.Role
		if role == "" {
			role = domain.RolePerformer
		}
		out = append(out, domain.ItemPerson{PersonID: personID, Role: role})
	}
	return out
}

// resolveOrCreatePerson returns the internal person ID for ep, creating a new
// Person record if none exists. Returns "" on unrecoverable error (logged).
func (s *Service) resolveOrCreatePerson(ctx context.Context, ep *domain.ExternalPerson, cache map[string]string) string {
	cacheKey := string(ep.Source) + ":" + ep.ExternalID
	if id, ok := cache[cacheKey]; ok {
		return id
	}

	if id, err := s.externalIDs.FindEntity(ctx, "person", string(ep.Source), ep.ExternalID); err == nil {
		cache[cacheKey] = id
		return id
	}

	personID := uuid.New().String()
	var imagePath string
	if ep.ImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, ep.ImageURL, "people", personID)
	}

	person := &domain.Person{
		ID:          personID,
		Name:        ep.Name,
		SortName:    ep.Name,
		Overview:    ep.Overview,
		Aliases:     ep.Aliases,
		Monitored:   false,
		MonitorMode: domain.MonitorNone,
		ImagePath:   imagePath,
		Metadata:    ep.Metadata,
		ExternalIDs: []domain.ExternalID{{Source: ep.Source, Value: ep.ExternalID}},
	}
	if err := s.people.Save(ctx, person); err != nil {
		slog.Warn("refresh studio: save performer failed", "name", ep.Name, "error", err)
		return ""
	}

	cache[cacheKey] = personID
	return personID
}

// sourceForEntry returns the first registered MetadataSource whose name matches
// one of the entry's external IDs, along with that external ID value.
func (s *Service) sourceForEntry(entry *domain.LibraryEntry) (ports.MetadataSource, string) {
	for _, extID := range entry.ExternalIDs {
		for _, src := range s.sources {
			if src.Name() == string(extID.Source) {
				return src, extID.Value
			}
		}
	}
	return nil, ""
}

// monitoredForMode returns whether a new item should be monitored on import
// given the studio's monitor mode, when the studio was added, and the item date.
// MonitorLatest is handled by the caller after all items are built.
func monitoredForMode(mode domain.MonitorMode, entryAddedAt, itemDate time.Time) bool {
	switch mode {
	case domain.MonitorAll:
		return true
	case domain.MonitorFuture:
		return itemDate.After(entryAddedAt)
	default: // MonitorNone, MonitorLatest — caller sets the latest item for latest mode
		return false
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func servesContentType(src ports.MetadataSource, contentType domain.ContentType) bool {
	return slices.Contains(src.ContentTypes(), contentType)
}

// fetchImage downloads url to the entity's image location and returns the file
// extension (e.g. ".jpg"). Returns "" on error (non-fatal; logged only).
func (s *Service) fetchImage(ctx context.Context, url, entityType, entityID string) string {
	destBase := media.ImagePath(s.mediaPath, entityType, entityID, "")
	if err := os.MkdirAll(filepath.Dir(destBase), 0o750); err != nil {
		slog.Warn("failed to create image dir", "error", err)
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Warn("failed to build image request", "url", url, "error", err)
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("failed to fetch image", "url", url, "error", err)
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("unexpected status fetching image", "url", url, "status", resp.StatusCode)
		return ""
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	dest := filepath.Clean(destBase + ext)
	if !strings.HasPrefix(dest, filepath.Clean(s.mediaPath)) {
		slog.Warn("image path outside media dir", "path", dest)
		return ""
	}
	f, err := os.Create(dest) //nolint:gosec
	if err != nil {
		slog.Warn("failed to create image file", "path", dest, "error", err)
		return ""
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		slog.Warn("failed to write image", "path", dest, "error", err)
		_ = os.Remove(dest)
		return ""
	}

	return ext
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "svg"):
		return ".svg"
	default:
		return ".jpg"
	}
}
