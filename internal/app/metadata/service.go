package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrUnknownJob is returned when jobName does not correspond to any registered refresh job.
var ErrUnknownJob = errors.New("unknown refresh job")

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
	downloader  ports.ImageDownloader
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
	downloader ports.ImageDownloader,
) *Service {
	return &Service{
		sources:     sources,
		agg:         NewAggregator(sources),
		jobs:        jobs,
		entries:     entries,
		groups:      groups,
		items:       items,
		people:      people,
		tags:        tags,
		externalIDs: externalIDs,
		downloader:  downloader,
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchStudios queries all sources that serve contentType and merges the results.
// Errors from individual sources are logged but do not abort the fan-out.
func (s *Service) SearchStudios(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalStudio, error) {
	return s.agg.SearchStudios(ctx, query, contentType, limit)
}

// SearchPeople queries all sources that cover role and merges the results.
// An empty role queries all sources.
func (s *Service) SearchPeople(ctx context.Context, query string, role domain.PersonRole, limit int) ([]*domain.ExternalPerson, error) {
	return s.agg.SearchPeople(ctx, query, role, limit)
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
	ec, ok := src.(ports.EntryContentSource)
	if !ok {
		return nil, 0, fmt.Errorf("fetch artist discography: source %q does not support entry content", src.Name())
	}
	groups, _, total, err := ec.FetchEntryContent(ctx, contentType, externalID, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch artist discography: %w", err)
	}
	return groups, total, nil
}

// SearchTracksRequest carries the parameters for a metadata-backed track lookup.
type SearchTracksRequest struct {
	Source      domain.ExternalIDSource
	ContentType domain.ContentType
	GroupExtID  string // external release-group or album ID
	Query       string // optional title filter applied after fetch
	Limit       int
}

// SearchTracks fetches all tracks for a release from the named source and applies
// an optional case-insensitive title filter. Returns a ValidationError when the
// source is unknown or does not implement GroupContentSource.
func (s *Service) SearchTracks(ctx context.Context, req *SearchTracksRequest) ([]*domain.ExternalItem, error) {
	src := s.sourceByName(string(req.Source))
	if src == nil {
		return nil, errs.Validation(fmt.Sprintf("unknown metadata source %q", req.Source))
	}
	gc, ok := src.(ports.GroupContentSource)
	if !ok {
		return nil, errs.Validation(fmt.Sprintf("source %q does not support group content", req.Source))
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}
	tracks, _, err := gc.FetchGroupContent(ctx, req.ContentType, req.GroupExtID, 1, limit)
	if err != nil {
		return nil, fmt.Errorf("search tracks: %w", err)
	}
	if req.Query == "" {
		return tracks, nil
	}
	q := strings.ToLower(req.Query)
	out := make([]*domain.ExternalItem, 0, len(tracks))
	for _, t := range tracks {
		if strings.Contains(strings.ToLower(t.Title), q) {
			out = append(out, t)
		}
	}
	return out, nil
}

// ImportItemRequest identifies a single item to fetch from a metadata source and
// persist with full enrichment (performers, tags, genres, cover art). AlbumExternalID
// and AlbumTitle are used only for music content — they carry the release MBID and
// title from the file's embedded tags so the track is linked to its album.
type ImportItemRequest struct {
	Source          domain.ExternalIDSource
	ExternalID      string
	ContentType     domain.ContentType
	AlbumExternalID string // release MBID (music); empty for other content types
	AlbumTitle      string // release title from embedded tags; used when creating a new album
	Monitored       bool
}

// ImportItemResult carries all entities created or resolved during an item import.
type ImportItemResult struct {
	Item    *domain.Item
	Entry   *domain.LibraryEntry
	Network *domain.LibraryEntry
	Album   *domain.Group
}

// ImportItem fetches the full ExternalItem from its metadata source, then creates
// the studio (and optional parent network), the item, and any album group in a
// single call. Performers, tags, genres, and cover art all come from the source —
// no metadata travels through the caller. The operation is idempotent: an existing
// item with the same external ID is returned without re-import.
func (s *Service) ImportItem(ctx context.Context, req *ImportItemRequest) (*ImportItemResult, error) {
	src := s.sourceByName(string(req.Source))
	if src == nil {
		return nil, errs.Validation(fmt.Sprintf("unknown metadata source %q", req.Source))
	}
	is, ok := src.(ports.ItemSource)
	if !ok {
		return nil, errs.Validation(fmt.Sprintf("source %q does not support item lookup by ID", req.Source))
	}
	ext, err := is.FindItemByExternalID(ctx, req.ContentType, req.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("import item: fetch from source: %w", err)
	}

	result, err := s.importItemContainers(ctx, req, ext)
	if err != nil {
		return nil, err
	}

	// Idempotency: item may already exist (e.g. created by a prior catalog refresh or album import).
	if id, findErr := s.externalIDs.FindEntity(ctx, "item", string(req.Source), req.ExternalID); findErr == nil {
		if existing, getErr := s.items.Get(ctx, id); getErr == nil {
			result.Item = existing
			return result, nil
		}
		// External ID points to a deleted item — fall through to re-create.
	}

	var groupID, libraryEntryID string
	if result.Album != nil {
		groupID = result.Album.ID
	}
	if result.Entry != nil {
		libraryEntryID = result.Entry.ID
	}

	itemID := uuid.New().String()
	var coverPath string
	if ext.ImageURL != "" && s.downloader != nil {
		coverPath = s.downloader.Download(ctx, ext.ImageURL, "items", itemID)
	}

	tagCache := s.loadTagCache(ctx)
	personCache := map[string]string{}

	itemStatus := domain.StatusWanted
	if !req.Monitored {
		itemStatus = domain.StatusMissing
	}

	item := &domain.Item{
		ID:             itemID,
		ContentType:    req.ContentType,
		LibraryEntryID: libraryEntryID,
		GroupID:        groupID,
		Title:          ext.Title,
		Overview:       ext.Overview,
		Date:           ext.Date,
		RuntimeSeconds: ext.RuntimeSecs,
		Monitored:      req.Monitored,
		Status:         itemStatus,
		CoverPath:      coverPath,
		People:         s.resolveItemPeople(ctx, ext.People, personCache),
		Tags:           append(s.resolveItemTags(ctx, domain.TagKeyAdult, ext.Tags, tagCache), s.resolveItemGenreTags(ctx, ext.Genres, tagCache)...),
		ExternalIDs:    []domain.ExternalID{{Source: ext.Source, Value: ext.ExternalID}},
		AddedAt:        time.Now().UTC(),
	}
	item.ApplyDefaults()
	if err := s.items.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("import item: save: %w", err)
	}
	result.Item = item
	slog.Info("item.imported", "entry_id", libraryEntryID, "item_id", item.ID, "title", item.Title)

	// Targeted entry image enrichment. For adult studios, repairEntryImageFromSource
	// fetches the image directly from the source. For music artists, fetchArtistHeroImage
	// fans out to the aggregator (TheAudioDB) which carries the hero images MBZ lacks.
	// Both are no-ops when an image is already present, so order does not matter.
	if result.Entry != nil {
		s.repairEntryImageFromSource(ctx, result.Entry, src, ext.Studio.ExternalID)
		s.fetchArtistHeroImage(ctx, result.Entry, ext.Studio.ExternalID)
	}
	s.importItemAlbumCover(ctx, req, ext, result)

	return result, nil
}

// importItemContainers finds or creates the parent entry/network and album (if any) for
// the item being imported. It never triggers a background catalog refresh.
func (s *Service) importItemContainers(ctx context.Context, req *ImportItemRequest, ext *domain.ExternalItem) (*ImportItemResult, error) {
	result := &ImportItemResult{}
	if ext.Studio != nil {
		er, err := s.ImportEntry(ctx, &ImportEntryRequest{
			Source:           ext.Studio.Source,
			ExternalID:       ext.Studio.ExternalID,
			Name:             ext.Studio.Name,
			Overview:         ext.Studio.Overview,
			ContentType:      req.ContentType,
			Monitored:        false,
			MonitorMode:      domain.MonitorLatest,
			ImageURL:         ext.Studio.ImageURL,
			WebsiteURL:       ext.Studio.WebsiteURL,
			ParentExternalID: ext.Studio.ParentID,
			ParentName:       ext.Studio.ParentName,
			ParentImageURL:   ext.Studio.ParentImageURL,
			ParentWebsiteURL: ext.Studio.ParentWebsiteURL,
			AutoImport:       false,
		})
		if err != nil {
			return nil, fmt.Errorf("import item: ensure entry: %w", err)
		}
		result.Entry = er.Entry
		result.Network = er.Network
	}
	// Prefer the explicitly supplied album ID; fall back to the one the source
	// embedded in the item (e.g. MBZ recording → release MBID).
	albumExtID := req.AlbumExternalID
	if albumExtID == "" {
		albumExtID = ext.GroupExternalID
	}
	if albumExtID != "" && result.Entry != nil {
		album, err := s.ImportAlbum(ctx, &ImportAlbumRequest{
			Source:         req.Source,
			ExternalID:     albumExtID,
			LibraryEntryID: result.Entry.ID,
			Title:          req.AlbumTitle,
			Monitored:      req.Monitored,
			MonitorMode:    domain.MonitorAll,
		})
		if err != nil {
			return nil, fmt.Errorf("import item: ensure album: %w", err)
		}
		result.Album = album
	}
	return result, nil
}

// importItemAlbumCover fetches the cover for the album created during an item import.
// It is a no-op when no album was created or the ExternalItem has no studio external ID.
func (s *Service) importItemAlbumCover(ctx context.Context, req *ImportItemRequest, ext *domain.ExternalItem, result *ImportItemResult) {
	if result.Album == nil || ext.Studio == nil {
		return
	}
	var albumExtID string
	for _, eid := range result.Album.ExternalIDs {
		if eid.Source == req.Source {
			albumExtID = eid.Value
			break
		}
	}
	if albumExtID == "" {
		return
	}
	s.fetchAlbumCovers(ctx, req.ContentType, ext.Studio.ExternalID, []artistAlbum{
		{internalID: result.Album.ID, extGroup: &domain.ExternalGroup{ExternalID: albumExtID}},
	})
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
	LabelName      string   // if non-empty, attached as a label tag on the created group
	PrimaryType    string   // e.g. "Album", "EP", "Single" — stored in group metadata
	SecondaryTypes []string // e.g. ["Live"], ["Compilation"] — stored in group metadata
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
		Metadata:       albumTypeMetadata(req.PrimaryType, req.SecondaryTypes),
	}
	if err := s.groups.Save(ctx, g); err != nil {
		return nil, fmt.Errorf("import album: save group: %w", err)
	}

	pending, err := s.collectNewGroupTracks(ctx, src, req.ExternalID, entry.ContentType)
	if err != nil {
		return nil, fmt.Errorf("import album: %w", err)
	}

	if req.LabelName != "" {
		tag, tagErr := s.findOrCreateTagByKey(ctx, domain.TagKeyLabel, req.LabelName, domain.TagScopeMetadata)
		if tagErr != nil {
			return nil, fmt.Errorf("import album: attach label tag: %w", tagErr)
		}
		if tagErr := s.tags.AddGroupTag(ctx, g.ID, tag.ID); tagErr != nil {
			return nil, fmt.Errorf("import album: link label tag: %w", tagErr)
		}
		g.Tags = append(g.Tags, *tag)
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
	gc, ok := src.(ports.GroupContentSource)
	if !ok {
		return nil, fmt.Errorf("fetch tracks: source %q does not support group content", src.Name())
	}
	const perPage = 100
	var tracks []*domain.ExternalItem
	for page := 1; ; page++ {
		extItems, trackTotal, err := gc.FetchGroupContent(ctx, contentType, groupExtID, page, perPage)
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
// logic consistent with RefreshArtist. Genre tags from ExternalItem.Genres are
// attached to each item.
func (s *Service) saveGroupTracks(ctx context.Context, contentType domain.ContentType, libraryEntryID, groupID string, monitorMode domain.MonitorMode, tracks []*domain.ExternalItem) error {
	latestIdx := 0
	for i, ei := range tracks {
		if isLaterExtItem(ei, tracks[latestIdx]) {
			latestIdx = i
		}
	}

	tagCache := s.loadTagCache(ctx)
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
			Sequence:       ei.Sequence,
			Date:           ei.Date,
			RuntimeSeconds: ei.RuntimeSecs,
			Monitored:      monitored,
			Status:         itemStatus,
			Tags:           s.resolveItemGenreTags(ctx, ei.Genres, tagCache),
			ExternalIDs:    []domain.ExternalID{{Source: ei.Source, Value: ei.ExternalID}},
			AddedAt:        now,
		}
		item.ApplyDefaults()
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

// ImportEntryRequest carries the data needed to create a LibraryEntry from an
// external source. The entry kind is always derived from ContentType — the caller
// does not set it. If ContentType has no known parent kind, ImportEntry returns an error.
type ImportEntryRequest struct {
	Source           domain.ExternalIDSource
	ExternalID       string
	Name             string
	Overview         string
	ContentType      domain.ContentType
	Monitored        bool
	MonitorMode      domain.MonitorMode
	AutoImport       bool // when true, enqueue a refresh job immediately after saving
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

// ImportEntryResult holds the persisted entry and, if applicable, its parent network.
type ImportEntryResult struct {
	Entry   *domain.LibraryEntry
	Network *domain.LibraryEntry // nil if no parent was specified or it already existed
}

// importOrFindNetwork resolves or creates the parent network for an ImportEntryRequest.
// Returns the parent ID, the newly created network (nil if it already existed or not needed), and any error.
func (s *Service) importOrFindNetwork(ctx context.Context, req *ImportEntryRequest) (string, *domain.LibraryEntry, error) {
	src := string(req.Source)

	if req.ParentExternalID != "" {
		if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ParentExternalID); err == nil {
			slog.Debug("network.resolved", "parent_id", id, "created", false)
			return id, nil, nil
		} else if !errors.Is(err, errs.ErrNotFound) {
			return "", nil, err
		}
	} else if req.ParentName == "" {
		return "", nil, nil
	}

	networkID := uuid.New().String()
	var imagePath string
	if req.ParentImageURL != "" && s.downloader != nil {
		imagePath = s.downloader.Download(ctx, req.ParentImageURL, "entries", networkID)
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
		slog.Error("network.create.failed", "name", req.ParentName, "error", err)
		return "", nil, err
	}
	slog.Debug("network.resolved", "parent_id", network.ID, "created", true)
	return network.ID, network, nil
}

// ImportEntry persists an external entity as a LibraryEntry of the correct kind
// for its content type. The kind is derived from ContentType.ParentEntryKind() —
// the caller does not set it. If the content type has no known parent kind, an
// error is returned immediately rather than silently creating the wrong thing.
// If a parent network is specified it is looked up or created first.
// The operation is idempotent: if an entry with the same external ID already
// exists, it is returned without modification.
func (s *Service) ImportEntry(ctx context.Context, req *ImportEntryRequest) (*ImportEntryResult, error) {
	kindStr := req.ContentType.ParentEntryKind()
	if kindStr == "" {
		return nil, fmt.Errorf("import entry: content type %q has no known entry kind", req.ContentType)
	}
	kind := domain.Kind(kindStr)

	src := string(req.Source)

	// Idempotency: return existing entry if already imported.
	if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ExternalID); err == nil {
		entry, err := s.entries.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		s.repairEntryImage(ctx, entry, req.ImageURL)
		return &ImportEntryResult{Entry: entry}, nil
	}

	res := &ImportEntryResult{}

	parentID, network, err := s.importOrFindNetwork(ctx, req)
	if err != nil {
		return nil, err
	}
	res.Network = network

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorLatest
	}

	entryID := uuid.New().String()

	var imagePath string
	if req.ImageURL != "" && s.downloader != nil {
		imagePath = s.downloader.Download(ctx, req.ImageURL, "entries", entryID)
	}

	meta := buildEntryMeta(req, kind)

	entry := &domain.LibraryEntry{
		ID:          entryID,
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
	if err := s.entries.Save(ctx, entry); err != nil {
		return nil, err
	}
	res.Entry = entry

	if req.AutoImport && s.jobs != nil {
		if _, err := s.SubmitRefreshJob(ctx, kind.RefreshJobName(), entry.ID); err != nil {
			slog.Warn("auto-import: failed to enqueue refresh", "entry_id", entry.ID, "error", err)
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
	if req.ImageURL != "" && s.downloader != nil {
		imagePath = s.downloader.Download(ctx, req.ImageURL, "people", personID)
	}

	var roles []domain.PersonRole
	if req.Role != "" {
		roles = []domain.PersonRole{req.Role}
	}

	p := &domain.Person{
		ID:          personID,
		Name:        req.Name,
		SortName:    req.Name,
		Overview:    req.Overview,
		Aliases:     req.Aliases,
		Roles:       roles,
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
	ec, ok := src.(ports.EntryContentSource)
	if !ok {
		return nil, fmt.Errorf("refresh studio %q: source %q does not support entry content", entryName, src.Name())
	}
	const perPage = 100
	var newExtItems []*domain.ExternalItem

	for page := 1; ; page++ {
		_, extItems, total, err := ec.FetchEntryContent(ctx, contentType, srcExtID, page, perPage)
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

// SubmitRefreshJob enqueues a metadata refresh for the given entity.
func (s *Service) SubmitRefreshJob(ctx context.Context, jobName, entityID string) (*domain.Job, error) {
	if entityID == "" {
		return nil, errs.Validation("entryId is required")
	}
	for _, k := range domain.Kinds() {
		if k.RefreshJobName() != jobName {
			continue
		}
		return s.jobs.Submit(ctx, jobName, map[string]any{"entry_id": entityID},
			func(ctx context.Context, p ports.ProgressReporter) error {
				return s.refreshForKind(ctx, k, entityID, p)
			})
	}
	return nil, ErrUnknownJob
}

// refreshForKind dispatches to the concrete refresh implementation for k.
// Adding a new Kind requires adding exactly one case here — no string maps.
func (s *Service) refreshForKind(ctx context.Context, k domain.Kind, entityID string, p ports.ProgressReporter) error {
	switch k {
	case domain.KindArtist:
		return s.RefreshArtist(ctx, entityID, p)
	case domain.KindStudio, domain.KindNetwork, domain.KindSeries, domain.KindAuthor, domain.KindMovie, domain.KindPublisher, domain.KindBook:
		return s.RefreshStudio(ctx, entityID, p)
	default:
		return fmt.Errorf("no refresh implementation for kind %s: %w", k, ErrUnknownJob)
	}
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

	slog.Info("studio.refresh.start", "entry_id", entryID, "name", entry.Name, "source", src.Name())

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
		if isLaterExtItem(ei, newExtItems[latestIdx]) {
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
		if ei.ImageURL != "" && s.downloader != nil {
			coverPath = s.downloader.Download(ctx, ei.ImageURL, "items", itemID)
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
			Tags:           append(s.resolveItemTags(ctx, domain.TagKeyAdult, ei.Tags, tagCache), s.resolveItemGenreTags(ctx, ei.Genres, tagCache)...),
			ExternalIDs:    []domain.ExternalID{{Source: ei.Source, Value: ei.ExternalID}},
			AddedAt:        now,
		}
		item.ApplyDefaults()
		if err := s.items.Save(ctx, item); err != nil {
			return fmt.Errorf("refresh studio %q: save item %q: %w", entry.Name, item.Title, err)
		}
		if p != nil {
			p.Report(i+1, total, item.Title)
		}
	}

	s.repairEntryImageFromSource(ctx, entry, src, srcExtID)

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

	slog.Info("artist.refresh.start", "entry_id", entryID, "name", entry.Name, "source", src.Name())

	albums, err := s.resolveArtistAlbums(ctx, src, entry, srcExtID)
	if err != nil {
		return err
	}

	s.importArtistPeople(ctx, src, entryID, srcExtID)
	s.fetchArtistHeroImage(ctx, entry, srcExtID)
	s.fetchAlbumCovers(ctx, entry.ContentType, srcExtID, albums)

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
		if isLaterExtItem(tr.extItem, newTracks[latestIdx].extItem) {
			latestIdx = i
		}
	}

	now := time.Now().UTC()
	total := len(newTracks)

	for i, tr := range newTracks {
		itemID := uuid.New().String()
		var coverPath string
		if tr.extItem.ImageURL != "" && s.downloader != nil {
			coverPath = s.downloader.Download(ctx, tr.extItem.ImageURL, "items", itemID)
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
			Sequence:       tr.extItem.Sequence,
			Date:           tr.extItem.Date,
			RuntimeSeconds: tr.extItem.RuntimeSecs,
			Monitored:      monitored,
			Status:         itemStatus,
			CoverPath:      coverPath,
			ExternalIDs:    []domain.ExternalID{{Source: tr.extItem.Source, Value: tr.extItem.ExternalID}},
			AddedAt:        now,
		}
		item.ApplyDefaults()
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
	ep, ok := src.(ports.EntryPeopleSource)
	if !ok {
		return
	}
	members, err := ep.FetchEntryPeople(ctx, srcExtID)
	if err != nil {
		slog.Warn("refresh artist: fetch members", "entry_id", entryID, "error", err)
		return
	}
	personCache := map[string]string{}
	for _, ep := range members {
		personID := s.resolveOrCreatePerson(ctx, ep, personCache)
		if personID == "" {
			continue
		}
		if ep.ExternalID != "" {
			s.fetchPersonHeroImage(ctx, personID, ep.ExternalID)
		}
		if saveErr := s.entries.SavePerson(ctx, entryID, domain.EntryPerson{
			PersonID: personID,
			Role:     string(ep.Role),
		}); saveErr != nil {
			slog.Warn("refresh artist: link member", "entry_id", entryID, "person_id", personID, "error", saveErr)
		}
	}
}

// fetchArtistHeroImage calls the aggregator for artist-level images and
// downloads the preferred hero image to disk when entry.ImagePath is not yet set.
// Image source priority: TheAudioDB → Fanart → any other source.
func (s *Service) fetchArtistHeroImage(ctx context.Context, entry *domain.LibraryEntry, srcExtID string) {
	merged, err := s.agg.FindByExternalID(ctx, entry.ContentType, srcExtID, entry.ID)
	if err != nil {
		slog.Warn("refresh artist: aggregate images", "entry_id", entry.ID, "error", err)
		return
	}
	if entry.ImagePath != "" || s.downloader == nil || slices.Contains(entry.LockedFields, "imageUrl") {
		return
	}
	url := s.preferredHeroURL(merged.Images)
	if url == "" {
		return
	}
	if ext := s.downloader.Download(ctx, url, "entries", entry.ID); ext != "" {
		entry.ImagePath = ext
		if saveErr := s.entries.Save(ctx, entry); saveErr != nil {
			slog.Warn("refresh artist: save entry image path", "entry_id", entry.ID, "error", saveErr)
		}
	}
}

// preferredHeroURL returns the URL of the hero image with the highest adapter-declared
// ImagePriority. Falls back to any hero image when no priority-mapped source has one.
// Returns "" when no hero image exists.
func (s *Service) preferredHeroURL(images []domain.ExternalImage) string {
	priority := make(map[string]int, len(s.sources))
	for _, src := range s.sources {
		priority[src.Name()] = src.ImagePriority()
	}
	var best string
	bestPri := -1
	for _, img := range images {
		if img.Type != domain.ImageTypeHero {
			continue
		}
		if p := priority[img.Source]; p > bestPri {
			best = img.URL
			bestPri = p
		}
	}
	return best
}

// fetchAlbumCovers downloads the first album cover for each album that does not
// already have one. All sources with ImagePriority > 0 are queried in descending
// priority order; each source first attempts a batch fetch via FetchEntryContent
// and falls back to per-album FetchGroupContent when that is not supported.
func (s *Service) fetchAlbumCovers(ctx context.Context, contentType domain.ContentType, artistMBID string, albums []artistAlbum) {
	if s.downloader == nil {
		return
	}

	coverByMBID := make(map[string]string)
	s.collectGroupCovers(ctx, contentType, artistMBID, albums, coverByMBID)
	slog.Info("refresh artist: album covers", "artist_mbid", artistMBID, "covers", len(coverByMBID), "library_albums", len(albums))

	for _, album := range albums {
		g, err := s.groups.Get(ctx, album.internalID)
		if err != nil {
			slog.Warn("refresh artist: load group for cover", "group_id", album.internalID, "error", err)
			continue
		}
		if g.CoverPath != "" {
			continue
		}
		coverURL, ok := coverByMBID[album.extGroup.ExternalID]
		if !ok {
			continue
		}
		ext := s.downloader.Download(ctx, coverURL, "groups", g.ID)
		if ext == "" {
			continue
		}
		g.CoverPath = ext
		if err := s.groups.Save(ctx, g); err != nil {
			slog.Warn("refresh artist: save album cover", "group_id", g.ID, "error", err)
		}
	}
}

// collectGroupCovers fans out to all sources with ImagePriority > 0 in descending
// priority order. Sources implementing EntryContentSource are called first for a
// batch fetch (e.g. fanart returns all album covers in one call); sources that
// only implement GroupContentSource fall back to per-album fetches. First writer
// wins, so higher-priority sources fill the map before lower-priority ones run.
func (s *Service) collectGroupCovers(ctx context.Context, contentType domain.ContentType, artistMBID string, albums []artistAlbum, out map[string]string) {
	type srcPri struct {
		src ports.MetadataSource
		pri int
	}
	var candidates []srcPri
	for _, src := range s.sources {
		if p := src.ImagePriority(); p > 0 {
			candidates = append(candidates, srcPri{src, p})
		}
	}
	slices.SortFunc(candidates, func(a, b srcPri) int { return b.pri - a.pri })

	for _, c := range candidates {
		if ec, ok := c.src.(ports.EntryContentSource); ok {
			_, items, _, err := ec.FetchEntryContent(ctx, contentType, artistMBID, 1, 1000)
			if err == nil {
				for _, item := range items {
					rgMBID, hasID := item.ExternalIDs["mbid"]
					if !hasID || len(item.Images) == 0 {
						continue
					}
					if _, exists := out[rgMBID]; !exists {
						out[rgMBID] = item.Images[0].URL
					}
				}
				continue
			}
			slog.Warn("refresh artist: fetch group covers", "source", c.src.Name(), "artist_mbid", artistMBID, "error", err)
			continue
		}
		if gc, ok := c.src.(ports.GroupContentSource); ok {
			s.collectGroupCoversByAlbum(ctx, gc, contentType, albums, out)
		}
	}
}

// collectGroupCoversByAlbum fills out with per-album cover URLs from src using
// FetchGroupContent. Albums that already have an entry in out are skipped.
func (s *Service) collectGroupCoversByAlbum(ctx context.Context, src ports.GroupContentSource, contentType domain.ContentType, albums []artistAlbum, out map[string]string) {
	for _, album := range albums {
		rgMBID := album.extGroup.ExternalID
		if _, exists := out[rgMBID]; exists {
			continue
		}
		items, _, err := src.FetchGroupContent(ctx, contentType, rgMBID, 1, 1)
		if err != nil {
			slog.Warn("refresh artist: fetch group cover", "mbid", rgMBID, "error", err)
			continue
		}
		if len(items) > 0 && len(items[0].Images) > 0 {
			out[rgMBID] = items[0].Images[0].URL
		}
	}
}

// fetchPersonHeroImage downloads the best available hero image for a person
// by fanning out FetchPersonImage to all registered sources. Image source
// priority mirrors fetchArtistHeroImage: TheAudioDB → Fanart → any. It is a
// no-op when the person already has an image or no downloader is configured.
func (s *Service) fetchPersonHeroImage(ctx context.Context, personID, extID string) {
	if s.downloader == nil {
		return
	}
	person, err := s.people.Get(ctx, personID)
	if err != nil || person.ImagePath != "" {
		return
	}
	var images []domain.ExternalImage
	for _, src := range s.sources {
		ps, ok := src.(ports.PersonImageSource)
		if !ok {
			continue
		}
		img, imgErr := ps.FetchPersonImage(ctx, extID)
		if imgErr != nil || img == nil {
			continue
		}
		images = append(images, *img)
	}
	url := s.preferredHeroURL(images)
	if url == "" {
		return
	}
	if ext := s.downloader.Download(ctx, url, "people", personID); ext != "" {
		person.ImagePath = ext
		if saveErr := s.people.Save(ctx, person); saveErr != nil {
			slog.Warn("refresh artist: save person image", "person_id", personID, "error", saveErr)
		}
	}
}

// resolveArtistAlbums pages through FetchEntryContent for the artist and
// creates Group records for any album not already in the library. Returns a
// slice of all albums (new and existing) with their internal IDs resolved.
// The album types imported are controlled by entry.Metadata["album_filter"].
func (s *Service) resolveArtistAlbums(ctx context.Context, src ports.MetadataSource, entry *domain.LibraryEntry, srcExtID string) ([]artistAlbum, error) {
	ec, ok := src.(ports.EntryContentSource)
	if !ok {
		return nil, fmt.Errorf("refresh artist %q: source %q does not support entry content", entry.Name, src.Name())
	}
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
		extGroups, _, albumTotal, err := ec.FetchEntryContent(ctx, entry.ContentType, srcExtID, page, perPage)
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
	token := eg.AlbumFilterToken()
	return slices.Contains(filter, token)
}

// buildEntryMeta constructs the metadata map for a new library entry from an
// import request. Kept separate to avoid inflating ImportEntry's cyclomatic
// complexity beyond the project lint threshold.
func buildEntryMeta(req *ImportEntryRequest, kind domain.Kind) map[string]any {
	meta := map[string]any{}
	if req.WebsiteURL != "" {
		meta["website_url"] = req.WebsiteURL
	}
	if kind.SupportsAlbumFilter() && len(req.AlbumFilter) > 0 {
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
	gc, ok := src.(ports.GroupContentSource)
	if !ok {
		return nil, fmt.Errorf("refresh artist %q: source %q does not support group content", entryName, src.Name())
	}
	const perPage = 100
	var tracks []artistTrack

	for _, album := range albums {
		for page := 1; ; page++ {
			extItems, trackTotal, err := gc.FetchGroupContent(ctx, contentType, album.extGroup.ExternalID, page, perPage)
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
		cache[strings.ToLower(t.Value)] = t
	}
	return cache
}

// findOrCreateTagByKey looks up an existing tag by key+value (case-insensitive)+scope,
// or creates one. Used by ImportAlbum for label tags.
func (s *Service) findOrCreateTagByKey(ctx context.Context, key domain.TagKey, value string, scope domain.TagScope) (*domain.Tag, error) {
	existing, err := s.tags.List(ctx, ports.TagFilter{Key: key, Scope: scope})
	if err != nil {
		return nil, fmt.Errorf("find or create tag: %w", err)
	}
	lower := strings.ToLower(value)
	for _, t := range existing {
		if strings.ToLower(t.Value) == lower {
			return t, nil
		}
	}
	t := &domain.Tag{ID: uuid.New().String(), Key: key, Value: value, Scope: scope}
	if err := s.tags.Save(ctx, t); err != nil {
		return nil, fmt.Errorf("find or create tag: save: %w", err)
	}
	return t, nil
}

// resolveItemGenreTags looks up or creates a Tag record with key=genre for each
// genre name and returns a deduplicated slice. The cache key is prefixed with
// "genre:" to avoid collisions with general tags in the shared tagCache.
func (s *Service) resolveItemGenreTags(ctx context.Context, genres []string, tagCache map[string]*domain.Tag) []domain.Tag {
	if s.tags == nil || len(genres) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []domain.Tag
	for _, name := range genres {
		cacheKey := "genre:" + strings.ToLower(name)
		if seen[cacheKey] {
			continue
		}
		seen[cacheKey] = true
		t, ok := tagCache[cacheKey]
		if !ok {
			t = &domain.Tag{Key: domain.TagKeyGenre, Value: name, Scope: domain.TagScopeMetadata}
			if err := s.tags.Save(ctx, t); err != nil {
				slog.Warn("resolve genre tag: save failed", "name", name, "error", err)
				continue
			}
			tagCache[cacheKey] = t
		}
		out = append(out, *t)
	}
	return out
}

// resolveItemTags looks up or creates a Tag record for each name and returns
// a deduplicated slice. tagCache is updated in-place for reuse across scenes.
// Cached tags with a different key are replaced so that re-imports correct
// previously misfiled tags (e.g. key='tag' → key='adult' after migration 011).
func (s *Service) resolveItemTags(ctx context.Context, tagKey domain.TagKey, names []string, tagCache map[string]*domain.Tag) []domain.Tag {
	if s.tags == nil || len(names) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []domain.Tag
	for _, name := range names {
		cacheKey := strings.ToLower(name)
		if seen[cacheKey] {
			continue
		}
		seen[cacheKey] = true
		t, ok := tagCache[cacheKey]
		if !ok || t.Key != tagKey {
			t = &domain.Tag{Key: tagKey, Value: name, Scope: domain.TagScopeMetadata}
			if err := s.tags.Save(ctx, t); err != nil {
				slog.Warn("refresh studio: save tag failed", "name", name, "error", err)
				continue
			}
			tagCache[cacheKey] = t
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
	if ep.ImageURL != "" && s.downloader != nil {
		imagePath = s.downloader.Download(ctx, ep.ImageURL, "people", personID)
	}

	var personRoles []domain.PersonRole
	if ep.Role != "" {
		personRoles = []domain.PersonRole{ep.Role}
	}
	person := &domain.Person{
		ID:          personID,
		Name:        ep.Name,
		SortName:    ep.Name,
		Overview:    ep.Overview,
		Aliases:     ep.Aliases,
		Roles:       personRoles,
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

// isLaterExtItem reports whether a sorts after b under the canonical item sort order,
// using the same key as the stored sort_key field (domain.ItemSortKey).
func isLaterExtItem(a, b *domain.ExternalItem) bool {
	return domain.ItemSortKey(a.Date, a.Sequence, a.Title) > domain.ItemSortKey(b.Date, b.Sequence, b.Title)
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

// ── Provider images ───────────────────────────────────────────────────────────

// FetchImagesForEntry fans out to all enabled sources using the entry's stored
// external IDs and returns all images, deduped by URL.
func (s *Service) FetchImagesForEntry(ctx context.Context, entryID string) ([]domain.ExternalImage, error) {
	entry, err := s.entries.Get(ctx, entryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("fetch images for entry: %w", err)
	}
	return s.collectImages(ctx, entry.ContentType, entry.ExternalIDs), nil
}

// FetchImagesForGroup fans out to all enabled sources using both the entry's and
// group's stored external IDs and returns all images, deduped by URL.
func (s *Service) FetchImagesForGroup(ctx context.Context, groupID string) ([]domain.ExternalImage, error) {
	group, err := s.groups.Get(ctx, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("fetch images for group: %w", err)
	}
	entry, err := s.entries.Get(ctx, group.LibraryEntryID)
	if err != nil {
		return nil, fmt.Errorf("fetch images for group: load entry: %w", err)
	}
	return s.agg.FetchGroupImages(ctx, entry.ContentType, entry.ExternalIDs, group.ExternalIDs), nil
}

// FetchImagesForItem fans out to all enabled sources using the item's stored
// external IDs and returns all images, deduped by URL.
func (s *Service) FetchImagesForItem(ctx context.Context, itemID string) ([]domain.ExternalImage, error) {
	item, err := s.items.Get(ctx, itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("fetch images for item: %w", err)
	}
	return s.collectImages(ctx, item.ContentType, item.ExternalIDs), nil
}

// FetchImagesForPerson fans out to all enabled sources using the person's stored
// external IDs and returns all images, deduped by URL.
func (s *Service) FetchImagesForPerson(ctx context.Context, personID string) ([]domain.ExternalImage, error) {
	person, err := s.people.Get(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("fetch images for person: %w", err)
	}
	// Person has no content type; each external ID's source is used to infer it.
	return s.collectImages(ctx, "", person.ExternalIDs), nil
}

// collectImages fans out each external ID through the aggregator (which calls all
// enabled sources for the relevant content type) and returns the combined image
// set, deduped by URL. When contentType is empty, it is inferred from the
// source's own ContentTypes list (used for person lookups).
func (s *Service) collectImages(ctx context.Context, contentType domain.ContentType, extIDs []domain.ExternalID) []domain.ExternalImage {
	seen := map[string]bool{}
	var all []domain.ExternalImage

	for _, extID := range extIDs {
		ct := contentType
		if ct == "" {
			src := s.sourceByName(string(extID.Source))
			if src == nil {
				continue
			}
			types := src.ContentTypes()
			if len(types) == 0 {
				continue
			}
			ct = types[0]
		}

		merged, err := s.agg.FindByExternalID(ctx, ct, extID.Value, "")
		if err != nil {
			slog.Warn("fetch images: aggregator error", "source", extID.Source, "external_id", extID.Value, "error", err)
			continue
		}

		for _, img := range merged.Images {
			if seen[img.URL] {
				continue
			}
			seen[img.URL] = true
			all = append(all, img)
		}
	}

	return all
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func servesContentType(src ports.MetadataSource, contentType domain.ContentType) bool {
	return slices.Contains(src.ContentTypes(), contentType)
}

func albumTypeMetadata(primaryType string, secondaryTypes []string) map[string]any {
	if primaryType == "" {
		return nil
	}
	st := secondaryTypes
	if st == nil {
		st = []string{}
	}
	return map[string]any{
		"primary_type":    primaryType,
		"secondary_types": st,
	}
}

// repairEntryImage downloads and saves an image for entry if its ImagePath is
// empty and imageURL is non-empty. The save error is intentionally ignored —
// a missing image is a cosmetic issue, not a fatal one.
func (s *Service) repairEntryImage(ctx context.Context, entry *domain.LibraryEntry, imageURL string) {
	if entry.ImagePath != "" || imageURL == "" || s.downloader == nil {
		return
	}
	if path := s.downloader.Download(ctx, imageURL, "entries", entry.ID); path != "" {
		entry.ImagePath = path
		_ = s.entries.Save(ctx, entry)
	}
}

// repairEntryImageFromSource fetches the entry's own metadata from src and
// downloads its image when the entry currently has none.
func (s *Service) repairEntryImageFromSource(ctx context.Context, entry *domain.LibraryEntry, src ports.MetadataSource, srcExtID string) {
	if entry.ImagePath != "" || s.downloader == nil {
		return
	}
	es, ok := src.(ports.ExternalIDSource)
	if !ok {
		return
	}
	extEntry, err := es.FindByExternalID(ctx, entry.ContentType, srcExtID)
	if err != nil || extEntry.ImageURL == "" {
		return
	}
	if path := s.downloader.Download(ctx, extEntry.ImageURL, "entries", entry.ID); path != "" {
		entry.ImagePath = path
		if saveErr := s.entries.Save(ctx, entry); saveErr != nil {
			slog.Warn("studio.refresh: update entry image", "error", saveErr)
		}
	}
}
