package scan_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"purser/internal/adapters/fs"
	"purser/internal/app/errs"
	"purser/internal/app/scan"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockScanner struct{ files []domain.ScannedFile }

func (m *mockScanner) Scan(_ context.Context, _ []string, _ ports.ScanFilter) (<-chan domain.ScannedFile, error) {
	ch := make(chan domain.ScannedFile, len(m.files))
	for _, f := range m.files {
		ch <- f
	}
	close(ch)
	return ch, nil
}

type mockWatcher struct{ events []ports.WatchEvent }

func (m *mockWatcher) Watch(_ context.Context, _ []string) (<-chan ports.WatchEvent, error) {
	ch := make(chan ports.WatchEvent, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

type mockFingerprinter struct {
	contentTypes []domain.ContentType
	result       *domain.Fingerprint
}

func (m *mockFingerprinter) ContentTypes() []domain.ContentType { return m.contentTypes }
func (m *mockFingerprinter) Fingerprint(_ context.Context, _ domain.ScannedFile) (*domain.Fingerprint, error) {
	return m.result, nil
}

type mockIdentifier struct {
	contentTypes []domain.ContentType
	candidates   []domain.MatchCandidate
}

func (m *mockIdentifier) ContentTypes() []domain.ContentType { return m.contentTypes }
func (m *mockIdentifier) Identify(_ context.Context, _ domain.ScannedFile) ([]domain.MatchCandidate, error) {
	return m.candidates, nil
}

type mockItemRepo struct {
	mu    sync.Mutex
	items map[string]*domain.Item
}

func newItemRepo(items ...*domain.Item) *mockItemRepo {
	r := &mockItemRepo{items: make(map[string]*domain.Item)}
	for _, i := range items {
		cp := *i
		r.items[i.ID] = &cp
	}
	return r
}

func (r *mockItemRepo) Get(_ context.Context, id string) (*domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i, ok := r.items[id]; ok {
		cp := *i
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return nil, 0, nil
}

func (r *mockItemRepo) Save(_ context.Context, item *domain.Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *item
	r.items[item.ID] = &cp
	return nil
}

func (r *mockItemRepo) Delete(_ context.Context, _ string) error               { return nil }
func (r *mockItemRepo) DeleteByGroup(_ context.Context, _ string) error        { return nil }
func (r *mockItemRepo) DeleteByLibraryEntry(_ context.Context, _ string) error { return nil }
func (r *mockItemRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return nil, nil //nolint:nilnil // test stub: port contract allows nil, nil for unused methods
}

func (r *mockItemRepo) status(id string) domain.ItemStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i, ok := r.items[id]; ok {
		return i.Status
	}
	return ""
}

type mockMediaFileRepo struct {
	mu             sync.Mutex
	byHash         map[string]*domain.MediaFile
	byPath         map[string]*domain.MediaFile
	byItemID       map[string]*domain.MediaFile
	deleted        map[string]bool
	saved          []*domain.MediaFile
	getByPathCalls int
}

func newMediaFileRepo(files ...*domain.MediaFile) *mockMediaFileRepo {
	r := &mockMediaFileRepo{
		byHash:   make(map[string]*domain.MediaFile),
		byPath:   make(map[string]*domain.MediaFile),
		byItemID: make(map[string]*domain.MediaFile),
		deleted:  make(map[string]bool),
	}
	for _, f := range files {
		cp := *f
		r.byHash[f.OSHash] = &cp
		r.byPath[f.Path] = &cp
		if f.ItemID != "" {
			r.byItemID[f.ItemID] = &cp
		}
	}
	return r
}

func (r *mockMediaFileRepo) GetByOSHash(_ context.Context, hash string) (*domain.MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.byHash[hash]; ok {
		cp := *f
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) GetByPath(_ context.Context, path string) (*domain.MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getByPathCalls++
	if f, ok := r.byPath[path]; ok {
		cp := *f
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) GetByItemID(_ context.Context, itemID string) (*domain.MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.byItemID[itemID]; ok {
		cp := *f
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) Save(_ context.Context, f *domain.MediaFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	if f.OSHash != "" {
		r.byHash[f.OSHash] = &cp
	}
	r.byPath[f.Path] = &cp
	if f.ItemID != "" {
		r.byItemID[f.ItemID] = &cp
	}
	r.saved = append(r.saved, &cp)
	return nil
}

func (r *mockMediaFileRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted[id] = true
	for hash, f := range r.byHash {
		if f.ID == id {
			delete(r.byHash, hash)
		}
	}
	for path, f := range r.byPath {
		if f.ID == id {
			delete(r.byPath, path)
		}
	}
	for itemID, f := range r.byItemID {
		if f.ID == id {
			delete(r.byItemID, itemID)
		}
	}
	return nil
}

type mockUnmatchedRepo struct {
	mu    sync.Mutex
	files map[string]*domain.UnmatchedFile
	saved []*domain.UnmatchedFile
}

func newUnmatchedRepo(files ...*domain.UnmatchedFile) *mockUnmatchedRepo {
	r := &mockUnmatchedRepo{files: make(map[string]*domain.UnmatchedFile)}
	for _, f := range files {
		cp := *f
		r.files[f.ID] = &cp
	}
	return r
}

func (r *mockUnmatchedRepo) List(_ context.Context, f ports.UnmatchedFilter) ([]*domain.UnmatchedFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.UnmatchedFile
	for _, uf := range r.files {
		if f.ContentType != "" && uf.ContentType != f.ContentType {
			continue
		}
		if f.Status != "" && uf.Status != f.Status {
			continue
		}
		if f.Path != "" && uf.Path != f.Path {
			continue
		}
		cp := *uf
		out = append(out, &cp)
	}
	return out, nil
}

func (r *mockUnmatchedRepo) Get(_ context.Context, id string) (*domain.UnmatchedFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.files[id]; ok {
		cp := *f
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockUnmatchedRepo) Save(_ context.Context, f *domain.UnmatchedFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.files[f.ID] = &cp
	r.saved = append(r.saved, &cp)
	return nil
}

func (r *mockUnmatchedRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.files, id)
	return nil
}

type mockNotifier struct {
	mu     sync.Mutex
	events []domain.NotificationEvent
}

func (n *mockNotifier) Dispatch(_ context.Context, e domain.NotificationEvent) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, e)
	return nil
}

func (n *mockNotifier) has(t domain.NotificationEventType) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, e := range n.events {
		if e.Type == t {
			return true
		}
	}
	return false
}

type mockThumbnailCache struct {
	mu     sync.Mutex
	stored []string
}

func (m *mockThumbnailCache) Store(_ context.Context, url, key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stored = append(m.stored, url)
	return "/test/thumbnails/" + key + ".jpg"
}

type mockGroupRepo struct {
	mu     sync.Mutex
	groups map[string]*domain.Group
}

func newGroupRepo(groups ...*domain.Group) *mockGroupRepo {
	r := &mockGroupRepo{groups: make(map[string]*domain.Group)}
	for _, g := range groups {
		cp := *g
		r.groups[g.ID] = &cp
	}
	return r
}

func (r *mockGroupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.groups[id]; ok {
		cp := *g
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockGroupRepo) List(_ context.Context, _ ports.GroupFilter) ([]*domain.Group, error) {
	return nil, nil //nolint:nilnil
}

func (r *mockGroupRepo) Save(_ context.Context, g *domain.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *g
	r.groups[g.ID] = &cp
	return nil
}

func (r *mockGroupRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.groups, id)
	return nil
}
func (r *mockGroupRepo) DeleteByLibraryEntry(_ context.Context, _ string) error { return nil }
func (r *mockGroupRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return nil, nil //nolint:nilnil
}

// ── helpers ────────────────────────────────────────────────────────────────────

func newScannedFile(ct domain.ContentType) domain.ScannedFile {
	return domain.ScannedFile{
		ID:           uuid.New().String(),
		Path:         "/media/test-" + string(ct),
		Size:         1024,
		ContentType:  ct,
		DiscoveredAt: time.Now().UTC(),
	}
}

func newItem() *domain.Item {
	return &domain.Item{ID: uuid.New().String(), Status: domain.StatusWanted, Title: "Test Item"}
}

// newSvc is the standard test constructor; passes nil for optional last two params.
func newSvc(
	scanner ports.FileScanner,
	watcher ports.FileWatcher,
	fps []ports.FileFingerprinter,
	ids []ports.FileIdentifier,
	items *mockItemRepo,
	mfRepo *mockMediaFileRepo,
	unmatchedRepo *mockUnmatchedRepo,
	notifier *mockNotifier,
	threshold float64,
	jobs ports.JobQueue,
	entries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
) *scan.Service {
	return scan.New(scanner, watcher, fps, ids, nil, nil, items, mfRepo, unmatchedRepo, notifier, threshold, jobs, entries, groups, nil, nil)
}

// ── core pipeline tests ───────────────────────────────────────────────────────

func TestService_AboveThreshold_AutoImports(t *testing.T) {
	item := newItem()
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "abc123"},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeMusic)}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save called %d times, want 1", len(mfRepo.saved))
	}
	if got := itemRepo.status(item.ID); got != domain.StatusImported {
		t.Errorf("item status = %q, want %q", got, domain.StatusImported)
	}
	if !notifier.has(domain.NotifyAutoMatched) {
		t.Error("NotifyAutoMatched not dispatched")
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save called %d times, want 0", len(unmatchedRepo.saved))
	}
}

func TestService_BelowThreshold_EnqueuesUnmatched(t *testing.T) {
	item := newItem()
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "def456"},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.40, Source: "filename"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeMusic)}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save called %d times, want 1", len(unmatchedRepo.saved))
	}
	if !notifier.has(domain.NotifyUnmatched) {
		t.Error("NotifyUnmatched not dispatched")
	}
	if len(mfRepo.saved) != 0 {
		t.Errorf("mediaFiles.Save called %d times, want 0", len(mfRepo.saved))
	}
}

// Regression: scanning the same unchanged file twice must be a no-op.
func TestService_IdempotentScan_SamePath_SameSize_Skips(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   "/media/test-music",
		Size:   1024, // same size as newScannedFile
		OSHash: "existinghash",
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "existinghash"},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeMusic)}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 0 {
		t.Errorf("mediaFiles.Save called %d times, want 0 (idempotent)", len(mfRepo.saved))
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save called %d times, want 0 (idempotent)", len(unmatchedRepo.saved))
	}
}

func TestService_ManualMatch(t *testing.T) {
	item := newItem()
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	notifier := &mockNotifier{}

	ufID := uuid.New().String()
	uf := &domain.UnmatchedFile{
		ID:          ufID,
		Path:        "/media/unknown.flac",
		Size:        2048,
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{OSHash: "abc"},
		Status:      domain.UnmatchedPending,
	}
	unmatchedRepo := newUnmatchedRepo(uf)

	svc := newSvc(nil, nil, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ManualMatch(context.Background(), ufID, item.ID); err != nil {
		t.Fatal(err)
	}

	if got := itemRepo.status(item.ID); got != domain.StatusImported {
		t.Errorf("item status = %q, want %q", got, domain.StatusImported)
	}
	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save called %d times, want 1", len(mfRepo.saved))
	}
	if mfRepo.saved[0].MatchConfidence != domain.MatchManual {
		t.Errorf("MatchConfidence = %q, want %q", mfRepo.saved[0].MatchConfidence, domain.MatchManual)
	}
	if len(unmatchedRepo.saved) == 0 {
		t.Fatal("unmatched record not updated")
	}
	last := unmatchedRepo.saved[len(unmatchedRepo.saved)-1]
	if last.Status != domain.UnmatchedMatched {
		t.Errorf("unmatched status = %q, want %q", last.Status, domain.UnmatchedMatched)
	}
}

// ── handleRemoved tests ───────────────────────────────────────────────────────

// Regression: handleRemoved previously only logged; now it marks the item missing
// and removes the media file record.
func TestService_FileRemoved_MarksItemMissing(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   "/media/gone.mkv",
		OSHash: "hash1",
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	watcher := &mockWatcher{events: []ports.WatchEvent{
		{Path: "/media/gone.mkv", Op: ports.WatchRemoved},
	}}

	svc := newSvc(nil, watcher, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.StartWatching(context.Background(), []scan.Module{{ContentType: domain.ContentTypeMovie, Roots: []string{"/media"}}}); err != nil {
		t.Fatal(err)
	}

	if got := itemRepo.status(item.ID); got != domain.StatusMissing {
		t.Errorf("item status = %q, want %q", got, domain.StatusMissing)
	}
	if !mfRepo.deleted[existing.ID] {
		t.Error("media file record not deleted after file removal")
	}
	if !notifier.has(domain.NotifyFileMissing) {
		t.Error("NotifyFileMissing not dispatched")
	}
}

// cleanUnmatchedAtPath: pending unmatched entries at the removed path are cleaned up.
func TestService_FileRemoved_CleansUnmatchedAtPath(t *testing.T) {
	itemRepo := newItemRepo()
	mfRepo := newMediaFileRepo() // no media file at this path
	pending := &domain.UnmatchedFile{
		ID:     uuid.New().String(),
		Path:   "/media/pending.mp4",
		Status: domain.UnmatchedPending,
	}
	unmatchedRepo := newUnmatchedRepo(pending)
	notifier := &mockNotifier{}

	watcher := &mockWatcher{events: []ports.WatchEvent{
		{Path: "/media/pending.mp4", Op: ports.WatchRemoved},
	}}

	svc := newSvc(nil, watcher, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.StartWatching(context.Background(), []scan.Module{{ContentType: domain.ContentTypeMovie, Roots: []string{"/media"}}}); err != nil {
		t.Fatal(err)
	}

	// Pending entry must have been removed.
	_, err := unmatchedRepo.Get(context.Background(), pending.ID)
	if !errs.IsNotFound(err) {
		t.Errorf("pending unmatched entry not deleted after file removal, err=%v", err)
	}
}

// ── moved-file / hash-state tests ────────────────────────────────────────────

// Regression: same OSHash at a different path → update the path record (moved file).
func TestService_MovedFile_UpdatesPath(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	const oldPath = "/media/old/scene.mp4"
	const newPath = "/media/new/scene.mp4"

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   oldPath,
		Size:   2048,
		OSHash: "movehash",
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result:       &domain.Fingerprint{OSHash: "movehash"},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path:        newPath,
		Size:        2048,
		ContentType: domain.ContentTypeAdult,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, nil,
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/media/new"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save called %d times, want 0 (moved file, not duplicate)", len(unmatchedRepo.saved))
	}
	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save called %d times, want 1 (path update)", len(mfRepo.saved))
	}
	if mfRepo.saved[0].Path != newPath {
		t.Errorf("updated path = %q, want %q", mfRepo.saved[0].Path, newPath)
	}
	if mfRepo.saved[0].ID != existing.ID {
		t.Error("Save created a new record instead of updating the existing one")
	}
}

// ── quality-upgrade tests ─────────────────────────────────────────────────────

// Regression: quality upgrade with auto mode replaces old file.
func TestService_QualityUpgrade_AutoMode_AppliesUpgrade(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing1080 := &domain.MediaFile{
		ID:      "mf-1080",
		ItemID:  item.ID,
		Path:    "/media/scene-1080p.mp4",
		Size:    2048,
		OSHash:  "hash1080",
		Quality: domain.Quality1080,
	}
	mfRepo := newMediaFileRepo(existing1080)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result: &domain.Fingerprint{
			OSHash:       "hash4k",
			EmbeddedTags: map[string]string{"resolution": "3840x2160"},
		},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path:        "/media/scene-4k.mp4",
		Size:        8192,
		ContentType: domain.ContentTypeAdult,
	}}}

	upgradeMode := map[domain.ContentType]string{domain.ContentTypeAdult: "auto"}
	svc := scan.New(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		nil, nil,
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85,
		nil, nil, nil, nil, upgradeMode)

	if err := svc.ScanRoots(context.Background(), []string{"/media"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if !mfRepo.deleted["mf-1080"] {
		t.Error("old 1080p media file not deleted on auto upgrade")
	}
	if len(mfRepo.saved) != 1 {
		t.Fatalf("mediaFiles.Save called %d times, want 1 (new 4K import)", len(mfRepo.saved))
	}
	if mfRepo.saved[0].Quality != domain.Quality4K {
		t.Errorf("saved quality = %q, want %q", mfRepo.saved[0].Quality, domain.Quality4K)
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save called %d times, want 0 (auto upgrade)", len(unmatchedRepo.saved))
	}
	if !notifier.has(domain.NotifyUpgradeApplied) {
		t.Error("NotifyUpgradeApplied not dispatched")
	}
}

// Regression: quality upgrade with queue mode (default) enqueues with DuplicateOf set.
func TestService_QualityUpgrade_QueueMode_EnqueuesWithDuplicateOf(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing720 := &domain.MediaFile{
		ID:      "mf-720",
		ItemID:  item.ID,
		Path:    "/media/scene-720p.mp4",
		Size:    1024,
		OSHash:  "hash720",
		Quality: domain.Quality720,
	}
	mfRepo := newMediaFileRepo(existing720)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result: &domain.Fingerprint{
			OSHash:       "hash1080",
			EmbeddedTags: map[string]string{"resolution": "1920x1080"},
		},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path:        "/media/scene-1080p.mp4",
		Size:        4096,
		ContentType: domain.ContentTypeAdult,
	}}}

	// default upgradeMode = "queue"
	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/media"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if mfRepo.deleted["mf-720"] {
		t.Error("old 720p media file deleted in queue mode — should be kept")
	}
	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save called %d times, want 1 (queued upgrade)", len(unmatchedRepo.saved))
	}
	if unmatchedRepo.saved[0].DuplicateOf != "mf-720" {
		t.Errorf("DuplicateOf = %q, want %q", unmatchedRepo.saved[0].DuplicateOf, "mf-720")
	}
	if !notifier.has(domain.NotifyUpgradeQueued) {
		t.Error("NotifyUpgradeQueued not dispatched")
	}
}

// Same-quality file for an already-imported item is enqueued as a duplicate.
func TestService_SameQualityDuplicate_EnqueuedWithDuplicateOf(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing := &domain.MediaFile{
		ID:      "mf-existing",
		ItemID:  item.ID,
		Path:    "/media/scene-a.mp4",
		Size:    1024,
		OSHash:  "hashA",
		Quality: domain.Quality1080,
	}
	mfRepo := newMediaFileRepo(existing)
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result: &domain.Fingerprint{
			OSHash:       "hashB",
			EmbeddedTags: map[string]string{"resolution": "1920x1080"}, // same quality
		},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{{
		Path:        "/media/scene-b.mp4",
		Size:        2048,
		ContentType: domain.ContentTypeAdult,
	}}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/media"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save called %d times, want 1 (duplicate)", len(unmatchedRepo.saved))
	}
	if unmatchedRepo.saved[0].DuplicateOf != "mf-existing" {
		t.Errorf("DuplicateOf = %q, want %q", unmatchedRepo.saved[0].DuplicateOf, "mf-existing")
	}
	if len(mfRepo.saved) != 0 {
		t.Errorf("mediaFiles.Save called %d times, want 0 (not imported)", len(mfRepo.saved))
	}
}

// Regression: item in any library status (not just wanted) gets auto-associated.
func TestService_AssociatesItemRegardlessOfStatus(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusSkipped // not wanted, not missing — but in library
	itemRepo := newItemRepo(item)
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		result:       &domain.Fingerprint{OSHash: "newhash"},
	}
	id := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "oshash"}},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeMusic)}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if got := itemRepo.status(item.ID); got != domain.StatusImported {
		t.Errorf("item status = %q, want %q", got, domain.StatusImported)
	}
	if len(mfRepo.saved) != 1 {
		t.Errorf("mediaFiles.Save called %d times, want 1", len(mfRepo.saved))
	}
}

// Regression: ExternalItem-only candidate (nil Item) must not panic and must enqueue.
func TestService_NilItemCandidate_NoPanic_Enqueues(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result:       &domain.Fingerprint{OSHash: "ext-hash"},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		candidates: []domain.MatchCandidate{
			{
				Item:         nil,
				ExternalItem: &domain.ExternalItem{ExternalID: "ext-1", Source: "stashdb"},
				Confidence:   0.95,
				Source:       "oshash",
			},
		},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeAdult)}}

	svc := newSvc(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}
	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save called %d times, want 1", len(unmatchedRepo.saved))
	}
}

// ThumbnailCache.Store is called during enqueue when ExternalItem has an image URL.
func TestService_Enqueue_CachesThumbnail(t *testing.T) {
	mfRepo := newMediaFileRepo()
	unmatchedRepo := newUnmatchedRepo()
	notifier := &mockNotifier{}
	tc := &mockThumbnailCache{}

	fp := &mockFingerprinter{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		result:       &domain.Fingerprint{},
	}
	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		candidates: []domain.MatchCandidate{
			{
				Item:         nil,
				ExternalItem: &domain.ExternalItem{ExternalID: "s1", ImageURL: "https://cdn.example.com/img.jpg"},
				Confidence:   0.50,
				Source:       "title",
			},
		},
	}
	scanner := &mockScanner{files: []domain.ScannedFile{newScannedFile(domain.ContentTypeAdult)}}

	svc := scan.New(scanner, nil, []ports.FileFingerprinter{fp}, []ports.FileIdentifier{ider},
		nil, nil,
		newItemRepo(), mfRepo, unmatchedRepo, notifier, 0.85,
		nil, nil, nil, tc, nil)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	tc.mu.Lock()
	stored := len(tc.stored)
	tc.mu.Unlock()
	if stored != 1 {
		t.Errorf("ThumbnailCache.Store called %d times, want 1", stored)
	}
	if len(unmatchedRepo.saved) != 1 {
		t.Fatalf("unmatched.Save called %d times, want 1", len(unmatchedRepo.saved))
	}
	if unmatchedRepo.saved[0].ThumbnailPath == "" {
		t.Error("ThumbnailPath not set on unmatched entry")
	}
}

// ── additional mocks ──────────────────────────────────────────────────────────

type mockJobQueue struct {
	mu   sync.Mutex
	jobs []*domain.Job
}

func (q *mockJobQueue) Submit(_ context.Context, name string, payload map[string]any, _ ports.JobFunc) (*domain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := &domain.Job{ID: uuid.New().String(), Name: name, Payload: payload, Status: domain.JobStatusQueued}
	q.jobs = append(q.jobs, j)
	return j, nil
}

func (q *mockJobQueue) Get(_ context.Context, id string) (*domain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return nil, errs.ErrNotFound
}

func (q *mockJobQueue) List(_ context.Context) ([]*domain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]*domain.Job{}, q.jobs...), nil
}

func (q *mockJobQueue) Cancel(_ context.Context, _ string) error { return nil }

type mockEntryRepo struct {
	entries map[string]*domain.LibraryEntry
}

func newEntryRepo(entries ...*domain.LibraryEntry) *mockEntryRepo {
	r := &mockEntryRepo{entries: make(map[string]*domain.LibraryEntry)}
	for _, e := range entries {
		cp := *e
		r.entries[e.ID] = &cp
	}
	return r
}

func (r *mockEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	if e, ok := r.entries[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockEntryRepo) List(_ context.Context, _ ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return nil, 0, nil //nolint:nilnil
}
func (r *mockEntryRepo) Save(_ context.Context, _ *domain.LibraryEntry) error { return nil }
func (r *mockEntryRepo) Delete(_ context.Context, _ string) error             { return nil }
func (r *mockEntryRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return nil, nil //nolint:nilnil
}

func (r *mockEntryRepo) GetPeople(_ context.Context, _ string) ([]domain.EntryPerson, error) {
	return nil, nil //nolint:nilnil
}

func (r *mockEntryRepo) SavePerson(_ context.Context, _ string, _ domain.EntryPerson) error {
	return nil
}
func (r *mockEntryRepo) RemovePerson(_ context.Context, _, _, _ string) error { return nil }

// ── dismiss / rescrape / list tests ──────────────────────────────────────────

func TestService_Dismiss(t *testing.T) {
	uf := &domain.UnmatchedFile{
		ID:     uuid.New().String(),
		Path:   "/media/unknown.flac",
		Status: domain.UnmatchedPending,
	}
	unmatchedRepo := newUnmatchedRepo(uf)

	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, nil)

	if err := svc.Dismiss(context.Background(), uf.ID); err != nil {
		t.Fatal(err)
	}

	got, err := unmatchedRepo.Get(context.Background(), uf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.UnmatchedDismissed {
		t.Errorf("status = %q, want %q", got.Status, domain.UnmatchedDismissed)
	}
}

func TestService_Dismiss_NotFound(t *testing.T) {
	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, nil, nil, nil)
	err := svc.Dismiss(context.Background(), "no-such-id")
	if !errs.IsNotFound(err) {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestService_Rescrape_ReturnsIdentifierCandidates(t *testing.T) {
	item := newItem()
	itemRepo := newItemRepo(item)

	uf := &domain.UnmatchedFile{
		ID:          uuid.New().String(),
		Path:        "/media/unknown.flac",
		Size:        1024,
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{OSHash: "abc"},
		Status:      domain.UnmatchedPending,
	}
	unmatchedRepo := newUnmatchedRepo(uf)

	ider := &mockIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{Item: item, Confidence: 0.80, Source: "tags"}},
	}

	svc := newSvc(nil, nil, nil, []ports.FileIdentifier{ider}, itemRepo, newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, nil)

	candidates, err := svc.Rescrape(context.Background(), uf.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	if candidates[0].Item.ID != item.ID {
		t.Errorf("candidate item ID = %q, want %q", candidates[0].Item.ID, item.ID)
	}

	// original record must be unchanged
	got, _ := unmatchedRepo.Get(context.Background(), uf.ID)
	if got.Status != domain.UnmatchedPending {
		t.Errorf("rescrape must not change status: got %q", got.Status)
	}
}

func TestService_Rescrape_QueryOverridesPath(t *testing.T) {
	var capturedPath string
	ider := &capturePathIdentifier{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}

	uf := &domain.UnmatchedFile{
		ID:          uuid.New().String(),
		Path:        "/original/path.flac",
		ContentType: domain.ContentTypeMusic,
		Status:      domain.UnmatchedPending,
	}

	svc := newSvc(nil, nil, nil, []ports.FileIdentifier{ider}, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(uf), &mockNotifier{}, 0.85, nil, nil, nil)

	_, _ = svc.Rescrape(context.Background(), uf.ID, "Bella Donna Stevie Nicks")
	capturedPath = ider.lastPath
	if capturedPath != "Bella Donna Stevie Nicks" {
		t.Errorf("path passed to identifier = %q, want override query", capturedPath)
	}
}

type capturePathIdentifier struct {
	contentTypes []domain.ContentType
	lastPath     string
}

func (c *capturePathIdentifier) ContentTypes() []domain.ContentType { return c.contentTypes }
func (c *capturePathIdentifier) Identify(_ context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	c.lastPath = f.Path
	return nil, nil
}

func TestService_ListUnmatched_FiltersByStatus(t *testing.T) {
	pending := &domain.UnmatchedFile{ID: uuid.New().String(), Status: domain.UnmatchedPending, ContentType: domain.ContentTypeMusic}
	matched := &domain.UnmatchedFile{ID: uuid.New().String(), Status: domain.UnmatchedMatched, ContentType: domain.ContentTypeMusic}
	repo := newUnmatchedRepo(pending, matched)

	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), repo, &mockNotifier{}, 0.85, nil, nil, nil)

	got, err := svc.ListUnmatched(context.Background(), ports.UnmatchedFilter{Status: domain.UnmatchedPending})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != pending.ID {
		t.Errorf("ListUnmatched(pending) returned %d items, want 1 with ID %q", len(got), pending.ID)
	}
}

// ── job submission tests ──────────────────────────────────────────────────────

func TestService_SubmitScanLibraryJob_EnqueuesJob(t *testing.T) {
	entryID := uuid.New().String()
	entry := &domain.LibraryEntry{ID: entryID, Path: "/mnt/music", Name: "Test Entry"}
	entryRepo := newEntryRepo(entry)
	jobs := &mockJobQueue{}

	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, entryRepo, nil)

	job, err := svc.SubmitScanLibraryJob(context.Background(), entryID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Name != "ScanLibrary" {
		t.Errorf("job name = %q, want ScanLibrary", job.Name)
	}
	if job.Payload["entry_id"] != entryID {
		t.Errorf("payload entry_id = %v, want %q", job.Payload["entry_id"], entryID)
	}
}

func TestService_SubmitScanLibraryJob_NoPath_ReturnsValidationError(t *testing.T) {
	entryID := uuid.New().String()
	entry := &domain.LibraryEntry{ID: entryID, Path: ""}
	entryRepo := newEntryRepo(entry)
	jobs := &mockJobQueue{}

	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, entryRepo, nil)

	_, err := svc.SubmitScanLibraryJob(context.Background(), entryID)
	if !errs.IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

// ── watcher integration tests (real filesystem + real watcher) ────────────────

// slowFingerprinter sleeps for a configurable duration to simulate a heavy
// operation (fpcalc, AcoustID network call, etc.).
type slowFingerprinter struct {
	delay        time.Duration
	contentTypes []domain.ContentType
}

func (s *slowFingerprinter) ContentTypes() []domain.ContentType { return s.contentTypes }
func (s *slowFingerprinter) Fingerprint(_ context.Context, _ domain.ScannedFile) (*domain.Fingerprint, error) {
	time.Sleep(s.delay)
	return &domain.Fingerprint{}, nil
}

// pollUnmatched repeatedly calls List until at least want entries appear or the
// deadline fires. It returns the list at the moment the condition is met.
func pollUnmatched(t *testing.T, repo *mockUnmatchedRepo, want int, deadline time.Duration) []*domain.UnmatchedFile {
	t.Helper()
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	for {
		time.Sleep(50 * time.Millisecond)
		files, err := repo.List(context.Background(), ports.UnmatchedFilter{Status: domain.UnmatchedPending})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(files) >= want {
			return files
		}
		select {
		case <-timer.C:
			t.Fatalf("timeout after %s: only %d/%d files enqueued", deadline, len(files), want)
		default:
		}
	}
}

// TestService_StartWatching_DroppedDirectory_EnqueuesFiles is an end-to-end
// integration test: a real fsnotify watcher feeds events to a real Service.
// It simulates the user dropping a folder of FLAC files into a watched root and
// verifies that all media files end up in the unmatched queue.
func TestService_StartWatching_DroppedDirectory_EnqueuesFiles(t *testing.T) {
	root := t.TempDir()

	unmatchedRepo := newUnmatchedRepo()
	watcher := fs.NewWatcherNoStabilityCheck(50 * time.Millisecond)
	svc := newSvc(nil, watcher, nil, nil, newItemRepo(), newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.0, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	go func() {
		if err := svc.StartWatching(ctx, []scan.Module{
			{ContentType: domain.ContentTypeMusic, Roots: []string{root}},
		}); err != nil && ctx.Err() == nil {
			t.Errorf("StartWatching: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond) // let fsnotify register the root watch

	// Build a staging directory (outside the watched root) with media files.
	staging := t.TempDir()
	mediaFiles := []string{"track_a.flac", "track_b.flac", "track_c.mp3"}
	nonMedia := []string{"cover.jpg", "info.txt"}
	for _, f := range append(mediaFiles, nonMedia...) {
		if err := os.WriteFile(filepath.Join(staging, f), []byte("fake-audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Atomic rename into the watched root — mirrors "drop folder" on same-volume Finder drag.
	dest := filepath.Join(root, "new-album")
	if err := os.Rename(staging, dest); err != nil {
		t.Fatal(err)
	}

	files := pollUnmatched(t, unmatchedRepo, len(mediaFiles), 15*time.Second)

	// Verify the exact set of media files is present.
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[filepath.Base(f.Path)] = true
	}
	for _, name := range mediaFiles {
		if !got[name] {
			t.Errorf("expected %q to be enqueued, was not", name)
		}
	}
	for _, name := range nonMedia {
		if got[name] {
			t.Errorf("expected %q NOT to be enqueued (non-media), but it was", name)
		}
	}
}

// TestService_StartWatching_ConcurrentProcessing verifies that a slow
// fingerprinter on one file does not block other files from being processed.
// With the old serial implementation, N files × slowDelay = N×slowDelay total.
// With concurrent processing the wall time should be roughly slowDelay, not N×slowDelay.
func TestService_StartWatching_ConcurrentProcessing(t *testing.T) {
	root := t.TempDir()

	const nFiles = 5
	const slowDelay = 150 * time.Millisecond

	unmatchedRepo := newUnmatchedRepo()
	fp := &slowFingerprinter{
		delay:        slowDelay,
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
	}
	watcher := fs.NewWatcherNoStabilityCheck(50 * time.Millisecond)
	svc := newSvc(nil, watcher, []ports.FileFingerprinter{fp}, nil, newItemRepo(), newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.0, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		if err := svc.StartWatching(ctx, []scan.Module{
			{ContentType: domain.ContentTypeMusic, Roots: []string{root}},
		}); err != nil && ctx.Err() == nil {
			t.Errorf("StartWatching: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Create all files, then record when the last one is written.
	for i := range nFiles {
		name := filepath.Join(root, fmt.Sprintf("track%02d.flac", i+1))
		if err := os.WriteFile(name, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()

	// With concurrent processing, debounce (50ms) + fingerprint (150ms) = ~200ms total.
	// Serial would be nFiles × (debounce + fingerprint) ≈ nFiles × 200ms.
	serialBound := time.Duration(nFiles) * (50*time.Millisecond + slowDelay)
	files := pollUnmatched(t, unmatchedRepo, nFiles, 10*time.Second)
	elapsed := time.Since(start)

	if len(files) != nFiles {
		t.Errorf("got %d files, want %d", len(files), nFiles)
	}
	if elapsed >= serialBound {
		t.Errorf("processing took %v which is ≥ serial bound %v — likely processing serially", elapsed, serialBound)
	}
}

func TestService_SubmitScanAllRootsJob_EnqueuesJob(t *testing.T) {
	jobs := &mockJobQueue{}
	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, nil, nil)

	modules := []scan.Module{
		{ContentType: domain.ContentTypeMovie, Roots: []string{"/mnt/movies"}},
		{ContentType: domain.ContentTypeTV, Roots: []string{"/mnt/tv"}},
	}
	job, err := svc.SubmitScanAllRootsJob(context.Background(), modules)
	if err != nil {
		t.Fatal(err)
	}
	if job.Name != "ScanAllRoots" {
		t.Errorf("job name = %q, want ScanAllRoots", job.Name)
	}
}

// ── ListUnmatchedGrouped tests ────────────────────────────────────────────────

// When the top candidate has ExternalItem.GroupExternalID set and Item is nil
// (pre-import file), files must be grouped by the external album ID, not lumped
// into the catch-all empty group.
func TestService_ListUnmatchedGrouped_ExternalGroupID(t *testing.T) {
	const releaseMBID = "release-mbid-1"
	const albumTitle = "Debut Album"

	uf := &domain.UnmatchedFile{
		ID:          uuid.New().String(),
		Path:        "/music/track01.flac",
		ContentType: domain.ContentTypeMusic,
		Status:      domain.UnmatchedPending,
		Candidates: []domain.MatchCandidate{
			{
				Item: nil,
				ExternalItem: &domain.ExternalItem{
					ExternalID:      "rec-001",
					GroupExternalID: releaseMBID,
					GroupTitle:      albumTitle,
				},
				Confidence: 0.80,
				Source:     "acoustid",
			},
		},
	}
	unmatchedRepo := newUnmatchedRepo(uf)
	svc := newSvc(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, nil)

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if got := groups[0].GroupID; got != releaseMBID {
		t.Errorf("GroupID = %q, want %q", got, releaseMBID)
	}
	if got := groups[0].GroupTitle; got != albumTitle {
		t.Errorf("GroupTitle = %q, want %q", got, albumTitle)
	}
}

// When the top candidate has Item set (library item already exists) but no
// ExternalItem GroupExternalID, grouping must use item.GroupID.
func TestService_ListUnmatchedGrouped_LocalGroupID(t *testing.T) {
	const localGroupID = "local-album-uuid"
	const albumTitle = "Local Album"

	item := &domain.Item{
		ID:      uuid.New().String(),
		GroupID: localGroupID,
		Title:   "Track 01",
		Status:  domain.StatusWanted,
	}
	grp := &domain.Group{ID: localGroupID, Title: albumTitle}

	uf := &domain.UnmatchedFile{
		ID:          uuid.New().String(),
		Path:        "/music/track01.flac",
		ContentType: domain.ContentTypeMusic,
		Status:      domain.UnmatchedPending,
		Candidates: []domain.MatchCandidate{
			{
				Item:         item,
				ExternalItem: nil,
				Confidence:   0.75,
				Source:       "tags",
			},
		},
	}
	unmatchedRepo := newUnmatchedRepo(uf)
	itemRepo := newItemRepo(item)
	groupRepo := newGroupRepo(grp)
	svc := newSvc(nil, nil, nil, nil, itemRepo, newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, groupRepo)

	groups, err := svc.ListUnmatchedGrouped(context.Background(), ports.UnmatchedFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if got := groups[0].GroupID; got != localGroupID {
		t.Errorf("GroupID = %q, want %q", got, localGroupID)
	}
	if got := groups[0].GroupTitle; got != albumTitle {
		t.Errorf("GroupTitle = %q, want %q", got, albumTitle)
	}
}

// ── FileGrouper / GroupIdentifier mocks ──────────────────────────────────────

type mockGrouper struct {
	contentTypes []domain.ContentType
	groupFn      func(context.Context, []domain.ScannedFile) ([]ports.ScannedFileGroup, error)
}

func (m *mockGrouper) ContentTypes() []domain.ContentType { return m.contentTypes }
func (m *mockGrouper) Group(ctx context.Context, files []domain.ScannedFile) ([]ports.ScannedFileGroup, error) {
	return m.groupFn(ctx, files)
}

type mockGroupIdentifier struct {
	contentTypes []domain.ContentType
	identifyFn   func(context.Context, ports.ScannedFileGroup) error
}

func (m *mockGroupIdentifier) ContentTypes() []domain.ContentType { return m.contentTypes }
func (m *mockGroupIdentifier) Identify(ctx context.Context, group ports.ScannedFileGroup) error {
	return m.identifyFn(ctx, group)
}

func TestScanService_RoutesGroupsThroughGrouper(t *testing.T) {
	musicFiles := []domain.ScannedFile{
		{Path: "/music/Hi Infidelity/01.flac", ContentType: domain.ContentTypeMusic, Size: 1000},
		{Path: "/music/Hi Infidelity/02.flac", ContentType: domain.ContentTypeMusic, Size: 1000},
	}
	scanner := &mockScanner{files: musicFiles}

	var grouperReceived []domain.ScannedFile
	grouper := &mockGrouper{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupFn: func(_ context.Context, files []domain.ScannedFile) ([]ports.ScannedFileGroup, error) {
			grouperReceived = files
			return []ports.ScannedFileGroup{{
				Files:    files,
				RootPath: "/music/Hi Infidelity",
			}}, nil
		},
	}

	var identifiedGroups []ports.ScannedFileGroup
	groupID := &mockGroupIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		identifyFn: func(_ context.Context, group ports.ScannedFileGroup) error {
			identifiedGroups = append(identifiedGroups, group)
			return nil
		},
	}

	svc := scan.New(
		scanner, nil,
		nil, nil,
		[]ports.FileGrouper{grouper},
		[]ports.GroupIdentifier{groupID},
		newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(),
		&mockNotifier{}, 0.85,
		nil, nil, nil, nil, nil,
	)

	if err := svc.ScanRoots(context.Background(), []string{"/music"}, ports.ScanFilter{}); err != nil {
		t.Fatalf("ScanRoots: %v", err)
	}

	if len(grouperReceived) != 2 {
		t.Errorf("grouper received %d files, want 2", len(grouperReceived))
	}
	if len(identifiedGroups) != 1 {
		t.Errorf("group identifier received %d groups, want 1", len(identifiedGroups))
	}
	if len(identifiedGroups) > 0 && identifiedGroups[0].RootPath != "/music/Hi Infidelity" {
		t.Errorf("group RootPath = %q, want %q", identifiedGroups[0].RootPath, "/music/Hi Infidelity")
	}
}
