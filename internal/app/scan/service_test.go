package scan_test

import (
	"context"
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
	saved          []*domain.MediaFile
	getByPathCalls int
}

func newMediaFileRepo(files ...*domain.MediaFile) *mockMediaFileRepo {
	r := &mockMediaFileRepo{
		byHash: make(map[string]*domain.MediaFile),
		byPath: make(map[string]*domain.MediaFile),
	}
	for _, f := range files {
		cp := *f
		r.byHash[f.OSHash] = &cp
		r.byPath[f.Path] = &cp
	}
	return r
}

func (r *mockMediaFileRepo) GetByOSHash(_ context.Context, hash string) (*domain.MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.byHash[hash]; ok {
		return f, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) GetByPath(_ context.Context, path string) (*domain.MediaFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getByPathCalls++
	if f, ok := r.byPath[path]; ok {
		return f, nil
	}
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) GetByItemID(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (r *mockMediaFileRepo) Save(_ context.Context, f *domain.MediaFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.byHash[f.OSHash] = &cp
	r.byPath[f.Path] = &cp
	r.saved = append(r.saved, &cp)
	return nil
}

func (r *mockMediaFileRepo) Delete(_ context.Context, _ string) error { return nil }

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

func (r *mockUnmatchedRepo) List(_ context.Context, _ ports.UnmatchedFilter) ([]*domain.UnmatchedFile, error) {
	return nil, nil
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

func (r *mockUnmatchedRepo) Delete(_ context.Context, _ string) error { return nil }

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

// ── tests ─────────────────────────────────────────────────────────────────────

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

	svc := scan.New(scanner, nil,
		[]ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85)

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

	svc := scan.New(scanner, nil,
		[]ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85)

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

func TestService_DuplicateOSHash_Skips(t *testing.T) {
	item := newItem()
	item.Status = domain.StatusImported
	itemRepo := newItemRepo(item)

	existing := &domain.MediaFile{
		ID:     uuid.New().String(),
		ItemID: item.ID,
		Path:   "/media/test-music",
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

	svc := scan.New(scanner, nil,
		[]ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85)

	if err := svc.ScanRoots(context.Background(), []string{"/"}, ports.ScanFilter{}); err != nil {
		t.Fatal(err)
	}

	if len(mfRepo.saved) != 0 {
		t.Errorf("mediaFiles.Save called %d times, want 0 (duplicate)", len(mfRepo.saved))
	}
	if len(unmatchedRepo.saved) != 0 {
		t.Errorf("unmatched.Save called %d times, want 0 (duplicate)", len(unmatchedRepo.saved))
	}
	if notifier.has(domain.NotifyAutoMatched) || notifier.has(domain.NotifyUnmatched) {
		t.Error("AutoMatched or Unmatched dispatched for duplicate file")
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

	svc := scan.New(nil, nil, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85)

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

func TestService_RemovedFile_NoStatusChange(t *testing.T) {
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
		{Path: "/media/gone.mkv", ContentType: domain.ContentTypeMovie, Op: ports.WatchRemoved},
	}}

	svc := scan.New(nil, watcher, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85)

	if err := svc.StartWatching(context.Background(), []string{"/media"}); err != nil {
		t.Fatal(err)
	}

	if mfRepo.getByPathCalls == 0 {
		t.Error("GetByPath was not called for removed file")
	}
	if got := itemRepo.status(item.ID); got != domain.StatusImported {
		t.Errorf("item status changed to %q after removal, want it unchanged at %q", got, domain.StatusImported)
	}
}
