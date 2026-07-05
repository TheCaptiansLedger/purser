package scan_test

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/app/scan"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// ── Fictional entity identifiers ─────────────────────────────────────────────
//
// All external IDs below are invented. No real network calls are made.
// "The Boozy Fig Digglers" is the test band; "Naughty Salamander Productions"
// is used for the no-parent-entry validation test.

const (
	boozyFigArtistMBID  = "artist-mbid-bfd-001"
	boozyFigReleaseMBID = "release-mbid-bfd-album1"
	boozyFigTrackMBID   = "rec-mbid-bfd-track01"
	boozyFigTrack2MBID  = "rec-mbid-bfd-track02"
	boozyFigAlbumTitle  = "Fermented Frequencies"
	boozyFigArtistName  = "The Boozy Fig Digglers"
	boozyFigTrack1Title = "Dirty Martini"
	boozyFigTrack2Title = "Overripe"
)

// ── E2E-specific stubs ────────────────────────────────────────────────────────

// e2eItemRepo is an in-memory ItemRepository that properly stores, retrieves,
// and lists items. Unlike mockItemRepo (scan unit tests), List returns real data.
type e2eItemRepo struct {
	mu    sync.Mutex
	items map[string]*domain.Item
	saved []*domain.Item
}

func newE2EItemRepo(initial ...*domain.Item) *e2eItemRepo {
	r := &e2eItemRepo{items: make(map[string]*domain.Item)}
	for _, it := range initial {
		cp := *it
		r.items[it.ID] = &cp
	}
	return r
}

func (r *e2eItemRepo) Get(_ context.Context, id string) (*domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if it, ok := r.items[id]; ok {
		cp := *it
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *e2eItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Item, 0, len(r.items))
	for _, it := range r.items {
		cp := *it
		out = append(out, &cp)
	}
	return out, len(out), nil
}

func (r *e2eItemRepo) Save(_ context.Context, item *domain.Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *item
	r.items[item.ID] = &cp
	r.saved = append(r.saved, &cp)
	return nil
}

func (r *e2eItemRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

func (r *e2eItemRepo) DeleteByGroup(_ context.Context, groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, it := range r.items {
		if it.GroupID == groupID {
			delete(r.items, id)
		}
	}
	return nil
}

func (r *e2eItemRepo) DeleteByLibraryEntry(_ context.Context, entryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, it := range r.items {
		if it.LibraryEntryID == entryID {
			delete(r.items, id)
		}
	}
	return nil
}

func (r *e2eItemRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeDestroy}, nil
}

// e2eEntryRepo is an in-memory LibraryEntryRepository that properly stores and
// retrieves entries. Unlike mockEntryRepo (scan unit tests), Save actually persists.
type e2eEntryRepo struct {
	mu   sync.Mutex
	data map[string]*domain.LibraryEntry
}

func newE2EEntryRepo() *e2eEntryRepo {
	return &e2eEntryRepo{data: make(map[string]*domain.LibraryEntry)}
}

func (r *e2eEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.data[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *e2eEntryRepo) List(_ context.Context, _ ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return nil, 0, nil //nolint:nilnil
}

func (r *e2eEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *e
	r.data[e.ID] = &cp
	return nil
}

func (r *e2eEntryRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}

func (r *e2eEntryRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeDestroy}, nil
}

func (r *e2eEntryRepo) GetPeople(_ context.Context, _ string) ([]domain.EntryPerson, error) {
	return nil, nil //nolint:nilnil
}

func (r *e2eEntryRepo) SavePerson(_ context.Context, _ string, _ domain.EntryPerson) error {
	return nil
}

func (r *e2eEntryRepo) RemovePerson(_ context.Context, _, _, _ string) error { return nil }

// e2eExtIDRepo is an in-memory ExternalIDRepository. It starts empty (simulating
// a fresh library) but supports register() to seed known external→internal ID mappings
// for idempotency tests.
type e2eExtIDRepo struct {
	mu      sync.Mutex
	entries map[string]string // "entityType:source:value" → internalID
}

func newE2EExtIDRepo() *e2eExtIDRepo {
	return &e2eExtIDRepo{entries: make(map[string]string)}
}

func (r *e2eExtIDRepo) FindEntity(_ context.Context, entityType, source, value string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.entries[entityType+":"+source+":"+value]; ok {
		return id, nil
	}
	return "", errs.ErrNotFound
}

func (r *e2eExtIDRepo) register(entityType, source, value, internalID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[entityType+":"+source+":"+value] = internalID
}

// e2ePersonRepo is a no-op PersonRepository for tests that don't exercise people.
type e2ePersonRepo struct{}

func (r *e2ePersonRepo) Get(_ context.Context, _ string) (*domain.Person, error) {
	return nil, errs.ErrNotFound
}

func (r *e2ePersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	return nil, 0, nil //nolint:nilnil
}

func (r *e2ePersonRepo) Save(_ context.Context, _ *domain.Person) error { return nil }
func (r *e2ePersonRepo) Delete(_ context.Context, _ string) error       { return nil }
func (r *e2ePersonRepo) ListRoles(_ context.Context) ([]domain.PersonRoleCount, error) {
	return nil, nil //nolint:nilnil
}

func (r *e2ePersonRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeUnlink}, nil
}

// e2eTagRepo is a no-op TagRepository for tests that don't exercise tagging.
type e2eTagRepo struct{}

func (r *e2eTagRepo) Get(_ context.Context, _ string) (*domain.Tag, error) {
	return nil, ports.ErrNotFound
}

func (r *e2eTagRepo) List(_ context.Context, _ ports.TagFilter) ([]*domain.Tag, error) {
	return nil, nil //nolint:nilnil
}

func (r *e2eTagRepo) Save(_ context.Context, t *domain.Tag) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

func (r *e2eTagRepo) Delete(_ context.Context, _ string) error { return nil }

func (r *e2eTagRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeUnlink}, nil
}

func (r *e2eTagRepo) AddGroupTag(_ context.Context, _, _ string) error    { return nil }
func (r *e2eTagRepo) RemoveGroupTag(_ context.Context, _, _ string) error { return nil }

// noopImageDownloader returns "" without making any network calls.
type noopImageDownloader struct{}

func (d *noopImageDownloader) Download(_ context.Context, _, _, _ string) string { return "" }

// ── Music source fixtures ─────────────────────────────────────────────────────

// boozyFigSource is a stubbed MetadataSource for "The Boozy Fig Digglers".
// FindItemByExternalID returns the track for any ID (tests care about the
// structure, not the specific ID). FetchGroupContent returns both album tracks.
type boozyFigSource struct{}

func (s *boozyFigSource) Name() string       { return "mbz" }
func (s *boozyFigSource) ImagePriority() int { return 0 }
func (s *boozyFigSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (s *boozyFigSource) FindItemByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return &domain.ExternalItem{
		Source:          domain.SourceMusicBrainz,
		ExternalID:      boozyFigTrackMBID,
		ContentType:     domain.ContentTypeMusic,
		Title:           boozyFigTrack1Title,
		Sequence:        "1",
		RuntimeSecs:     213,
		GroupExternalID: boozyFigReleaseMBID,
		GroupTitle:      boozyFigAlbumTitle,
		Studio: &domain.ExternalStudio{
			Source:     domain.SourceMusicBrainz,
			ExternalID: boozyFigArtistMBID,
			Name:       boozyFigArtistName,
		},
	}, nil
}

func (s *boozyFigSource) FetchGroupContent(_ context.Context, _ domain.ContentType, groupExtID string, page, _ int) ([]*domain.ExternalItem, int, error) {
	if groupExtID != boozyFigReleaseMBID || page != 1 {
		return nil, 0, nil
	}
	tracks := []*domain.ExternalItem{
		{
			Source:          domain.SourceMusicBrainz,
			ExternalID:      boozyFigTrackMBID,
			ContentType:     domain.ContentTypeMusic,
			Title:           boozyFigTrack1Title,
			Sequence:        "1",
			RuntimeSecs:     213,
			GroupExternalID: boozyFigReleaseMBID,
		},
		{
			Source:          domain.SourceMusicBrainz,
			ExternalID:      boozyFigTrack2MBID,
			ContentType:     domain.ContentTypeMusic,
			Title:           boozyFigTrack2Title,
			Sequence:        "2",
			RuntimeSecs:     187,
			GroupExternalID: boozyFigReleaseMBID,
		},
	}
	return tracks, len(tracks), nil
}

// nilStudioSource returns an ExternalItem with Studio=nil, simulating a
// source that lacks artist information. Used for Phase 4 (no parent entry) test.
type nilStudioSource struct{}

func (s *nilStudioSource) Name() string       { return "mbz" }
func (s *nilStudioSource) ImagePriority() int { return 0 }
func (s *nilStudioSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (s *nilStudioSource) FindItemByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  "orphan-track-001",
		ContentType: domain.ContentTypeMusic,
		Title:       "Phantom Track",
		Studio:      nil, // no artist — triggers Phase 4 validation error
	}, nil
}

func (s *nilStudioSource) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _ int, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// newMetaSvc builds a metadata.Service wired to shared in-memory repos.
func newMetaSvc(
	src ports.MetadataSource,
	items ports.ItemRepository,
	entries *e2eEntryRepo,
	groups *mockGroupRepo,
	extIDs *e2eExtIDRepo,
) *metadata.Service {
	var sources []ports.MetadataSource
	if src != nil {
		sources = []ports.MetadataSource{src}
	}
	return metadata.New(
		sources,
		nil, // no job queue
		entries,
		groups,
		items,
		&e2ePersonRepo{},
		&e2eTagRepo{},
		extIDs,
		&noopImageDownloader{},
	)
}

// boozyFigCandidate returns a below-threshold MatchCandidate for a Boozy Fig
// track, backed by ExternalItem only (no local library item yet).
func boozyFigCandidate() domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Source:          domain.SourceMusicBrainz,
			ExternalID:      boozyFigTrackMBID,
			ContentType:     domain.ContentTypeMusic,
			Title:           boozyFigTrack1Title,
			GroupExternalID: boozyFigReleaseMBID,
			GroupTitle:      boozyFigAlbumTitle,
		},
		Confidence: 0.80, // below default 0.85 threshold
		Source:     string(domain.MatchSourceAcoustID),
	}
}

// ── Test cases ────────────────────────────────────────────────────────────────

// TestE2E_AboveThreshold_AutoImports verifies the happy path: a file identified
// with high confidence against an existing library item is auto-imported without
// entering the unmatched queue.
func TestE2E_AboveThreshold_AutoImports(t *testing.T) {
	item := &domain.Item{
		ID:          uuid.New().String(),
		ContentType: domain.ContentTypeMusic,
		Title:       boozyFigTrack1Title,
		Status:      domain.StatusWanted,
	}
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "bfd-hash-001"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates: []domain.MatchCandidate{
			{Item: item, Confidence: 0.95, Source: string(domain.MatchSourceAcoustID)},
		},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		ID:          uuid.New().String(),
		Path:        "/music/boozy-fig/dirty-martini.flac",
		Size:        4096,
		ContentType: domain.ContentTypeMusic,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save = %d, want 1", len(mfRepo.saved))
	}
	if got := itemRepo.status(item.ID); got != domain.StatusImported {
		t.Errorf("item.Status = %q, want imported", got)
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save = %d, want 0", len(unmatchedRepo.saved))
	}
	if !notifier.has(domain.NotifyAutoMatched) {
		t.Error("NotifyAutoMatched not dispatched")
	}
}

// TestE2E_BelowThreshold_TwoFiles_GroupedByAlbum verifies that two files whose
// top candidates share the same GroupExternalID are grouped into a single queue
// group rather than separate catch-all groups.
func TestE2E_BelowThreshold_TwoFiles_GroupedByAlbum(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	// Each file needs a unique OSHash to avoid false move-detection.
	rfp := &routingFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		routes: map[string]*domain.Fingerprint{
			"/music/bfd/track01.flac": {OSHash: "bfd-hash-t1"},
			"/music/bfd/track02.flac": {OSHash: "bfd-hash-t2"},
		},
	}
	// Both files resolve to the same album's external group ID.
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{boozyFigCandidate()},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{
		{ID: uuid.New().String(), Path: "/music/bfd/track01.flac", Size: 4096, ContentType: domain.ContentTypeMusic},
		{ID: uuid.New().String(), Path: "/music/bfd/track02.flac", Size: 3800, ContentType: domain.ContentTypeMusic},
	}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{rfp}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(unmatchedRepo.saved) != 2 {
		t.Fatalf("unmatched.Save = %d, want 2", len(unmatchedRepo.saved))
	}
	if len(mfRepo.saved) != 0 {
		t.Errorf("mediaFiles.Save = %d, want 0 (below threshold)", len(mfRepo.saved))
	}

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListUnmatchedGrouped = %d groups, want 1", len(groups))
	}
	if groups[0].GroupID != boozyFigReleaseMBID {
		t.Errorf("group.GroupID = %q, want %q", groups[0].GroupID, boozyFigReleaseMBID)
	}
	if groups[0].GroupTitle != boozyFigAlbumTitle {
		t.Errorf("group.GroupTitle = %q, want %q", groups[0].GroupTitle, boozyFigAlbumTitle)
	}
	if len(groups[0].Files) != 2 {
		t.Errorf("group has %d files, want 2", len(groups[0].Files))
	}
}

// TestE2E_ImportAndCreate_FullFlow is the primary cross-service e2e test.
// It exercises: scan → unmatched queue → ImportItem (metadata) → ManualMatch (scan)
// and asserts that the item, entry, and album IDs are wired correctly end-to-end.
func TestE2E_ImportAndCreate_FullFlow(t *testing.T) {
	ctx := context.Background()

	// Shared in-memory repos used by both scan.Service and metadata.Service.
	items := newItemRepo()
	entries := newE2EEntryRepo()
	groups := newGroupRepo()
	unmatched := newUnmatchedRepo()
	mfRepo := newMediaFileRepo()
	extIDs := newE2EExtIDRepo()
	notifier := &mockNotifier{}

	// ── Phase 1: Scan creates an unmatched entry ────────────────────────────────
	trackPath := "/music/bfd/dirty-martini.flac"
	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "bfd-hash-001"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{boozyFigCandidate()},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		ID:          uuid.New().String(),
		Path:        trackPath,
		Size:        4096,
		ContentType: domain.ContentTypeMusic,
	}}}

	scanSvc := scan.New(
		scanner, nil,
		[]ports.FileFingerprinter{fp},
		[]ports.FileIdentifier{ider},
		items, mfRepo, unmatched, notifier,
		0.85, nil, entries, groups, nil, nil,
	)

	if err := scanSvc.ScanRoots(ctx, []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}
	if len(unmatched.saved) != 1 {
		t.Fatalf("after scan: unmatched.Save = %d, want 1", len(unmatched.saved))
	}
	ufID := unmatched.saved[0].ID

	// ── Phase 2: ImportItem creates artist, album, and track ───────────────────
	metaSvc := newMetaSvc(&boozyFigSource{}, items, entries, groups, extIDs)

	result, err := metaSvc.ImportItem(ctx, &metadata.ImportItemRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  boozyFigTrackMBID,
		ContentType: domain.ContentTypeMusic,
		Monitored:   true,
	})
	if err != nil {
		t.Fatalf("ImportItem: %v", err)
	}
	if result.Item == nil {
		t.Fatal("ImportItem returned nil item")
	}
	if result.Entry == nil {
		t.Fatal("ImportItem returned nil entry (artist)")
	}
	if result.Album == nil {
		t.Fatal("ImportItem returned nil album")
	}

	// ── Phase 3: ManualMatch wires the file to the created item ────────────────
	if err := scanSvc.ManualMatch(ctx, ufID, result.Item.ID); err != nil {
		t.Fatalf("ManualMatch: %v", err)
	}

	// ── Assertions ─────────────────────────────────────────────────────────────
	savedItem, err := items.Get(ctx, result.Item.ID)
	if err != nil {
		t.Fatalf("get item after match: %v", err)
	}
	if savedItem.LibraryEntryID != result.Entry.ID {
		t.Errorf("item.LibraryEntryID = %q, want %q", savedItem.LibraryEntryID, result.Entry.ID)
	}
	if savedItem.GroupID != result.Album.ID {
		t.Errorf("item.GroupID = %q, want %q", savedItem.GroupID, result.Album.ID)
	}
	if savedItem.Status != domain.StatusImported {
		t.Errorf("item.Status = %q, want imported", savedItem.Status)
	}

	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save = %d, want 1", len(mfRepo.saved))
	}
	if mfRepo.saved[0].Path != trackPath {
		t.Errorf("mediaFile.Path = %q, want %q", mfRepo.saved[0].Path, trackPath)
	}

	uf, err := unmatched.Get(ctx, ufID)
	if err != nil {
		t.Fatalf("get unmatched after match: %v", err)
	}
	if uf.Status != domain.UnmatchedMatched {
		t.Errorf("unmatched.Status = %q, want matched", uf.Status)
	}
}

// TestE2E_ImportItem_NoParentEntry_ValidationError verifies Phase 4: when the
// external source returns a track with Studio=nil, ImportItem must return a
// validation error rather than silently saving a dangling item.
func TestE2E_ImportItem_NoParentEntry_ValidationError(t *testing.T) {
	items := newE2EItemRepo()
	entries := newE2EEntryRepo()
	groups := newGroupRepo()
	extIDs := newE2EExtIDRepo()

	metaSvc := newMetaSvc(&nilStudioSource{}, items, entries, groups, extIDs)

	_, err := metaSvc.ImportItem(context.Background(), &metadata.ImportItemRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  "orphan-track-001",
		ContentType: domain.ContentTypeMusic,
	})
	if err == nil {
		t.Fatal("ImportItem returned nil error, want validation error")
	}
	if !errs.IsValidation(err) {
		t.Errorf("error type = %T (%v), want ValidationError", err, err)
	}

	// No item must have been saved.
	all, _, _ := items.List(context.Background(), ports.ItemFilter{})
	if len(all) != 0 {
		t.Errorf("item repo has %d items after failed import, want 0", len(all))
	}
}

// TestE2E_IdempotentScan_SamePath_SameSize verifies that scanning the same
// unchanged file twice is a no-op: the second pass must not create a second
// MediaFile record.
func TestE2E_IdempotentScan_SamePath_SameSize(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	const path = "/music/bfd/dirty-martini.flac"
	const size = int64(4096)

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   path,
		Size:   size,
		OSHash: "bfd-hash-001",
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "bfd-hash-001"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path: path, Size: size, ContentType: domain.ContentTypeMusic,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	// First scan — file is already tracked; path idempotency skips processing.
	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}
	if len(mfRepo.saved) != 0 {
		t.Errorf("first scan: mediaFiles.Save = %d, want 0 (idempotent)", len(mfRepo.saved))
	}

	// Second scan — same outcome.
	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}
	if len(mfRepo.saved) != 0 {
		t.Errorf("second scan: mediaFiles.Save = %d, want 0 (idempotent)", len(mfRepo.saved))
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save = %d, want 0", len(unmatchedRepo.saved))
	}
}

// TestE2E_IdempotentScan_MovedFile_UpdatesPath verifies that scanning a file
// which has been renamed/moved (same hash, new path) updates the existing
// MediaFile record in place rather than creating a new one.
func TestE2E_IdempotentScan_MovedFile_UpdatesPath(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	const oldPath = "/music/old/dirty-martini.flac"
	const newPath = "/music/new/dirty-martini.flac"

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   oldPath,
		Size:   4096,
		OSHash: "bfd-hash-move",
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "bfd-hash-move"},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path: newPath, Size: 4096, ContentType: domain.ContentTypeMusic,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, nil,
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save = %d, want 1 (path update)", len(mfRepo.saved))
	}
	if mfRepo.saved[0].ID != existing.ID {
		t.Error("Save created a new record instead of updating the existing one")
	}
	if mfRepo.saved[0].Path != newPath {
		t.Errorf("updated path = %q, want %q", mfRepo.saved[0].Path, newPath)
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save = %d, want 0 (moved file, not new)", len(unmatchedRepo.saved))
	}
}

// TestE2E_NoCandidates_EnqueuesInCatchAllGroup verifies that a file with no
// identification candidates is enqueued in the catch-all group (GroupID="").
func TestE2E_NoCandidates_EnqueuesInCatchAllGroup(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "unknown-hash"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   nil, // no candidates
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		ID:          uuid.New().String(),
		Path:        "/music/mystery/unknown.flac",
		Size:        1024,
		ContentType: domain.ContentTypeMusic,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save = %d, want 1", len(unmatchedRepo.saved))
	}
	if len(unmatchedRepo.saved[0].Candidates) != 0 {
		t.Errorf("candidates = %d, want 0 (no identification)", len(unmatchedRepo.saved[0].Candidates))
	}

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// No-candidate files land in the catch-all group (GroupID="").
	if len(groups) != 1 {
		t.Fatalf("ListUnmatchedGrouped = %d groups, want 1", len(groups))
	}
	if groups[0].GroupID != "" {
		t.Errorf("catch-all GroupID = %q, want empty string", groups[0].GroupID)
	}
}

// TestE2E_ThreeFilesFromSameAlbum_SingleGroup verifies that three files whose
// top candidates all carry the same GroupExternalID collapse into exactly one
// group in ListUnmatchedGrouped.
func TestE2E_ThreeFilesFromSameAlbum_SingleGroup(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	// Each file needs a unique OSHash to avoid false move-detection.
	rfp3 := &routingFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		routes: map[string]*domain.Fingerprint{
			"/music/bfd/track01.flac": {OSHash: "bfd-3-hash-t1"},
			"/music/bfd/track02.flac": {OSHash: "bfd-3-hash-t2"},
			"/music/bfd/track03.flac": {OSHash: "bfd-3-hash-t3"},
		},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{boozyFigCandidate()},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{
		{ID: uuid.New().String(), Path: "/music/bfd/track01.flac", Size: 4096, ContentType: domain.ContentTypeMusic},
		{ID: uuid.New().String(), Path: "/music/bfd/track02.flac", Size: 3800, ContentType: domain.ContentTypeMusic},
		{ID: uuid.New().String(), Path: "/music/bfd/track03.flac", Size: 4200, ContentType: domain.ContentTypeMusic},
	}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{rfp3}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListUnmatchedGrouped = %d groups, want 1", len(groups))
	}
	if groups[0].GroupID != boozyFigReleaseMBID {
		t.Errorf("GroupID = %q, want %q", groups[0].GroupID, boozyFigReleaseMBID)
	}
	if len(groups[0].Files) != 3 {
		t.Errorf("group has %d files, want 3", len(groups[0].Files))
	}
}

// TestE2E_MixedThreshold_PartialAutoImport verifies the mixed-confidence case:
// a file with a high-confidence local-item candidate is auto-imported while a
// second file with a below-threshold ExternalItem-only candidate goes to the queue.
func TestE2E_MixedThreshold_PartialAutoImport(t *testing.T) {
	item := newItem()
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	const importPath = "/music/bfd/imported.flac"
	const queuePath = "/music/bfd/queued.flac"

	// Each file must have a unique OSHash so the hash-state resolver does not
	// treat the second file as a moved copy of the first.
	routingFP := &routingFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		routes: map[string]*domain.Fingerprint{
			importPath: {OSHash: "import-hash-001"},
			queuePath:  {OSHash: "queue-hash-001"},
		},
	}
	routingIder := &routingIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		routes: map[string][]domain.MatchCandidate{
			importPath: {{Item: item, Confidence: 0.95, Source: string(domain.MatchSourceAcoustID)}},
			queuePath:  {boozyFigCandidate()},
		},
	}

	scanner := &mockScanner{files: []domain.ScannedFile{
		{ID: uuid.New().String(), Path: importPath, Size: 4096, ContentType: domain.ContentTypeMusic},
		{ID: uuid.New().String(), Path: queuePath, Size: 3800, ContentType: domain.ContentTypeMusic},
	}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{routingFP}, []ports.FileIdentifier{routingIder},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save = %d, want 1 (only imported.flac)", len(mfRepo.saved))
	}
	if mfRepo.saved[0].Path != importPath {
		t.Errorf("auto-imported path = %q, want %q", mfRepo.saved[0].Path, importPath)
	}
	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save = %d, want 1 (only queued.flac)", len(unmatchedRepo.saved))
	}
	if unmatchedRepo.saved[0].Path != queuePath {
		t.Errorf("queued path = %q, want %q", unmatchedRepo.saved[0].Path, queuePath)
	}
}

// routingFingerprinter returns a different Fingerprint per file path, allowing
// tests to give each file a unique OSHash so the hash-state resolver does not
// mistakenly treat two files as moved copies of each other.
type routingFingerprinter struct {
	contentTypes []domain.ContentType
	routes       map[string]*domain.Fingerprint // file path → fingerprint
}

func (r *routingFingerprinter) ContentTypes() []domain.ContentType { return r.contentTypes }
func (r *routingFingerprinter) Fingerprint(_ context.Context, f domain.ScannedFile) (*domain.Fingerprint, error) {
	if fp, ok := r.routes[f.Path]; ok {
		return fp, nil
	}
	return &domain.Fingerprint{}, nil
}

// routingIdentifier returns different candidates per file path. It allows the
// mixed-threshold test to control which file gets which outcome.
type routingIdentifier struct {
	contentTypes []domain.ContentType
	routes       map[string][]domain.MatchCandidate // file path → candidates
}

func (r *routingIdentifier) ContentTypes() []domain.ContentType { return r.contentTypes }
func (r *routingIdentifier) Identify(_ context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	return r.routes[f.Path], nil
}

// TestE2E_ExternalItemNoGroupID_CatchAllGroup verifies that a file whose top
// candidate has an ExternalItem but an empty GroupExternalID falls into the
// catch-all group (GroupID="") rather than panicking or being dropped.
func TestE2E_ExternalItemNoGroupID_CatchAllGroup(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "no-group-hash"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates: []domain.MatchCandidate{
			{
				ExternalItem: &domain.ExternalItem{
					Source:          domain.SourceMusicBrainz,
					ExternalID:      "track-without-album-001",
					ContentType:     domain.ContentTypeMusic,
					Title:           "Orphan Track",
					GroupExternalID: "", // no album information
				},
				Confidence: 0.75,
				Source:     string(domain.MatchSourceEmbeddedTags),
			},
		},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		ID:          uuid.New().String(),
		Path:        "/music/misc/orphan.flac",
		Size:        2048,
		ContentType: domain.ContentTypeMusic,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListUnmatchedGrouped = %d groups, want 1", len(groups))
	}
	if groups[0].GroupID != "" {
		t.Errorf("GroupID = %q, want empty (catch-all)", groups[0].GroupID)
	}
}

// TestE2E_ImportItem_AlbumTracksCreated verifies that ImportItem creates the
// artist entry, the album group, and all tracks returned by FetchGroupContent.
// After a single call, both tracks from the Boozy Fig Digglers album must exist.
func TestE2E_ImportItem_AlbumTracksCreated(t *testing.T) {
	ctx := context.Background()
	items := newE2EItemRepo()
	entries := newE2EEntryRepo()
	groups := newGroupRepo()
	extIDs := newE2EExtIDRepo()

	metaSvc := newMetaSvc(&boozyFigSource{}, items, entries, groups, extIDs)

	result, err := metaSvc.ImportItem(ctx, &metadata.ImportItemRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  boozyFigTrackMBID,
		ContentType: domain.ContentTypeMusic,
		Monitored:   true,
	})
	if err != nil {
		t.Fatalf("ImportItem: %v", err)
	}

	// Verify the entry (artist) was created with the correct kind.
	if result.Entry == nil {
		t.Fatal("result.Entry is nil")
	}
	if result.Entry.Kind != domain.KindArtist {
		t.Errorf("entry.Kind = %q, want artist", result.Entry.Kind)
	}
	if result.Entry.Name != boozyFigArtistName {
		t.Errorf("entry.Name = %q, want %q", result.Entry.Name, boozyFigArtistName)
	}

	// Verify the album group was created.
	if result.Album == nil {
		t.Fatal("result.Album is nil")
	}
	if result.Album.LibraryEntryID != result.Entry.ID {
		t.Errorf("album.LibraryEntryID = %q, want %q", result.Album.LibraryEntryID, result.Entry.ID)
	}

	// Verify all tracks from FetchGroupContent were created (both tracks of the album).
	allItems, _, err := items.List(ctx, ports.ItemFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// ImportItem saves the specific track (1), plus saveGroupTracks adds remaining
	// tracks not already in the library. Since track01 is saved as the "item" and
	// FetchGroupContent returns both tracks, both must exist.
	if len(allItems) < 2 {
		t.Errorf("item repo has %d items, want ≥2 (both album tracks)", len(allItems))
	}
	titles := make(map[string]bool, len(allItems))
	for _, it := range allItems {
		titles[it.Title] = true
	}
	if !titles[boozyFigTrack1Title] {
		t.Errorf("track %q not found in item repo", boozyFigTrack1Title)
	}
	if !titles[boozyFigTrack2Title] {
		t.Errorf("track %q not found in item repo", boozyFigTrack2Title)
	}
	for _, it := range allItems {
		if it.LibraryEntryID != result.Entry.ID {
			t.Errorf("item %q: LibraryEntryID = %q, want %q", it.Title, it.LibraryEntryID, result.Entry.ID)
		}
		if it.GroupID != result.Album.ID {
			t.Errorf("item %q: GroupID = %q, want %q", it.Title, it.GroupID, result.Album.ID)
		}
	}
}

// TestE2E_ImportItem_IdempotentViaExtIDRepo verifies that when the external ID
// repository already knows about an item (simulating a prior import), ImportItem
// returns the existing item and does not create a duplicate.
func TestE2E_ImportItem_IdempotentViaExtIDRepo(t *testing.T) {
	ctx := context.Background()
	items := newE2EItemRepo()
	entries := newE2EEntryRepo()
	groups := newGroupRepo()
	extIDs := newE2EExtIDRepo()

	// Pre-seed an item that was "already imported" in a prior run.
	existingItem := &domain.Item{
		ID:          uuid.New().String(),
		ContentType: domain.ContentTypeMusic,
		Title:       boozyFigTrack1Title,
		Status:      domain.StatusImported,
	}
	existingItem.ApplyDefaults()
	if err := items.Save(ctx, existingItem); err != nil {
		t.Fatal(err)
	}
	extIDs.register("item", "mbz", boozyFigTrackMBID, existingItem.ID)

	// Pre-seed the artist entry so importItemContainers is also idempotent.
	artistEntry := &domain.LibraryEntry{
		ID:          uuid.New().String(),
		ContentType: domain.ContentTypeMusic,
		Kind:        domain.KindArtist,
		Name:        boozyFigArtistName,
	}
	artistEntry.ApplyDefaults()
	if err := entries.Save(ctx, artistEntry); err != nil {
		t.Fatal(err)
	}
	extIDs.register("library_entry", "mbz", boozyFigArtistMBID, artistEntry.ID)

	// Pre-seed the album group so ImportAlbum is also idempotent.
	albumGroup := &domain.Group{
		ID:             uuid.New().String(),
		LibraryEntryID: artistEntry.ID,
		Title:          boozyFigAlbumTitle,
		ExternalIDs:    []domain.ExternalID{{Source: domain.SourceMusicBrainz, Value: boozyFigReleaseMBID}},
	}
	if err := groups.Save(ctx, albumGroup); err != nil {
		t.Fatal(err)
	}
	extIDs.register("group", "mbz", boozyFigReleaseMBID, albumGroup.ID)

	metaSvc := newMetaSvc(&boozyFigSource{}, items, entries, groups, extIDs)

	result, err := metaSvc.ImportItem(ctx, &metadata.ImportItemRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  boozyFigTrackMBID,
		ContentType: domain.ContentTypeMusic,
		Monitored:   true,
	})
	if err != nil {
		t.Fatalf("ImportItem: %v", err)
	}
	if result.Item == nil {
		t.Fatal("ImportItem returned nil item")
	}
	if result.Item.ID != existingItem.ID {
		t.Errorf("returned item ID = %q, want existing %q (should not create duplicate)", result.Item.ID, existingItem.ID)
	}

	// Verify no new item was saved — the repo should still contain only the one
	// pre-seeded item (idempotency means no additional Save call for the item).
	all, _, _ := items.List(ctx, ports.ItemFilter{})
	// The item repo may have the original pre-seeded item. ImportItem should not
	// add a second one when the extIDRepo lookup succeeds.
	itemCount := 0
	for _, it := range all {
		if it.ContentType == domain.ContentTypeMusic && it.Title == boozyFigTrack1Title {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("found %d items with title %q, want exactly 1 (idempotent import)", itemCount, boozyFigTrack1Title)
	}
}

// TestE2E_UnmatchedQueue_PendingFileScanAgain_IsIdempotent verifies that
// re-scanning a file already in the pending queue does not create a second
// queue entry for the same path.
func TestE2E_UnmatchedQueue_PendingFileScanAgain_IsIdempotent(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "repeat-hash"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{boozyFigCandidate()},
	}
	const path = "/music/bfd/track01.flac"
	file := domain.ScannedFile{
		ID:          uuid.New().String(),
		Path:        path,
		Size:        4096,
		ContentType: domain.ContentTypeMusic,
	}
	scanner := &mockScanner{files: []domain.ScannedFile{file}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	// First scan: creates the unmatched entry.
	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}
	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("first scan: unmatched.Save = %d, want 1", len(unmatchedRepo.saved))
	}

	// Second scan with same file: path+size idempotency or hash dedup prevents a
	// second queue entry being created for the same path.
	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	// List all pending entries at this path — must still be exactly 1.
	pending, err := unmatchedRepo.List(context.Background(), ports.UnmatchedFilter{
		Path:   path,
		Status: domain.UnmatchedPending,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("after two scans: pending entries for path = %d, want 1", len(pending))
	}
}
