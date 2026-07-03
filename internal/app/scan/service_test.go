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

	svc := scan.New(scanner, nil,
		[]ports.FileFingerprinter{fp}, []ports.FileIdentifier{id},
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
		itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

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

	svc := scan.New(nil, nil, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

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

	svc := scan.New(nil, watcher, nil, nil, itemRepo, mfRepo, unmatchedRepo, notifier, 0.85, nil, nil, nil)

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

// ── new tests ─────────────────────────────────────────────────────────────────

func TestService_Dismiss(t *testing.T) {
	uf := &domain.UnmatchedFile{
		ID:     uuid.New().String(),
		Path:   "/media/unknown.flac",
		Status: domain.UnmatchedPending,
	}
	unmatchedRepo := newUnmatchedRepo(uf)

	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, nil)

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
	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, nil, nil, nil)
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

	svc := scan.New(nil, nil, nil, []ports.FileIdentifier{ider}, itemRepo, newMediaFileRepo(), unmatchedRepo, &mockNotifier{}, 0.85, nil, nil, nil)

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

	svc := scan.New(nil, nil, nil, []ports.FileIdentifier{ider}, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(uf), &mockNotifier{}, 0.85, nil, nil, nil)

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

	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), repo, &mockNotifier{}, 0.85, nil, nil, nil)

	got, err := svc.ListUnmatched(context.Background(), ports.UnmatchedFilter{Status: domain.UnmatchedPending})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != pending.ID {
		t.Errorf("ListUnmatched(pending) returned %d items, want 1 with ID %q", len(got), pending.ID)
	}
}

func TestService_SubmitScanLibraryJob_EnqueuesJob(t *testing.T) {
	entryID := uuid.New().String()
	entry := &domain.LibraryEntry{ID: entryID, Path: "/mnt/music", Name: "Test Entry"}
	entryRepo := newEntryRepo(entry)
	jobs := &mockJobQueue{}

	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, entryRepo, nil)

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

	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, entryRepo, nil)

	_, err := svc.SubmitScanLibraryJob(context.Background(), entryID)
	if !errs.IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestService_SubmitScanAllRootsJob_EnqueuesJob(t *testing.T) {
	jobs := &mockJobQueue{}
	svc := scan.New(nil, nil, nil, nil, newItemRepo(), newMediaFileRepo(), newUnmatchedRepo(), &mockNotifier{}, 0.85, jobs, nil, nil)

	job, err := svc.SubmitScanAllRootsJob(context.Background(), []string{"/mnt/movies", "/mnt/tv"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Name != "ScanAllRoots" {
		t.Errorf("job name = %q, want ScanAllRoots", job.Name)
	}
}
