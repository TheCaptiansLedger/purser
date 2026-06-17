package metadata_test

import (
	"context"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubEntryRepo struct {
	data map[string]*domain.LibraryEntry
}

func newStubEntryRepo() *stubEntryRepo {
	return &stubEntryRepo{data: make(map[string]*domain.LibraryEntry)}
}

func (r *stubEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
	}
	return e, nil
}

func (r *stubEntryRepo) List(_ context.Context, _ ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return nil, 0, nil
}

func (r *stubEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	r.data[e.ID] = e
	return nil
}

func (r *stubEntryRepo) Delete(_ context.Context, id string) error {
	delete(r.data, id)
	return nil
}

type stubItemRepo struct {
	items []*domain.Item
}

func (r *stubItemRepo) Get(_ context.Context, _ string) (*domain.Item, error) {
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return r.items, len(r.items), nil
}

func (r *stubItemRepo) Save(_ context.Context, item *domain.Item) error {
	r.items = append(r.items, item)
	return nil
}
func (r *stubItemRepo) Delete(_ context.Context, _ string) error { return nil }

// stubSource is a hand-rolled MetadataSource that returns a fixed scene list.
type stubSource struct {
	scenes []*domain.ExternalItem
	total  int
}

func (s *stubSource) Name() string { return "stashdb" }
func (s *stubSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

func (s *stubSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, nil
}

func (s *stubSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, nil
}

func (s *stubSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, nil
}

func (s *stubSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *stubSource) FindByExternalID(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotFound
}

func (s *stubSource) FetchEntryContent(_ context.Context, _ string, page, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	if page == 1 {
		return nil, s.scenes, s.total, nil
	}
	return nil, nil, s.total, nil
}

func (s *stubSource) FetchGroupContent(_ context.Context, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

type stubJobQueue struct {
	submitted []string
}

func (q *stubJobQueue) Submit(_ context.Context, name string, _ map[string]any, _ ports.JobFunc) (*domain.Job, error) {
	q.submitted = append(q.submitted, name)
	return &domain.Job{Name: name, Status: domain.JobStatusQueued}, nil
}

func (q *stubJobQueue) Get(_ context.Context, _ string) (*domain.Job, error) {
	return nil, ports.ErrNotFound
}
func (q *stubJobQueue) List(_ context.Context) ([]*domain.Job, error) { return nil, nil }
func (q *stubJobQueue) Cancel(_ context.Context, _ string) error      { return nil }

type stubPersonRepo struct {
	saved []*domain.Person
}

func (r *stubPersonRepo) Get(_ context.Context, _ string) (*domain.Person, error) {
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubPersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	return nil, 0, nil
}

func (r *stubPersonRepo) Save(_ context.Context, p *domain.Person) error {
	r.saved = append(r.saved, p)
	return nil
}
func (r *stubPersonRepo) Delete(_ context.Context, _ string) error { return nil }

type stubTagRepo struct {
	saved []*domain.Tag
}

func (r *stubTagRepo) Get(_ context.Context, _ string) (*domain.Tag, error) {
	return nil, ports.ErrNotFound
}

func (r *stubTagRepo) List(_ context.Context, _ ports.TagFilter) ([]*domain.Tag, error) {
	return r.saved, nil
}

func (r *stubTagRepo) Save(_ context.Context, t *domain.Tag) error {
	r.saved = append(r.saved, t)
	return nil
}
func (r *stubTagRepo) Delete(_ context.Context, _ string) error { return nil }

// stubExternalIDRepo returns ErrNotFound for every lookup, simulating a fresh
// database where no external IDs have been imported yet.
type stubExternalIDRepo struct{}

func (r *stubExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return "", fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func newService() *metadata.Service {
	return metadata.New(
		nil, // no metadata sources needed for import tests
		nil, // no job queue
		newStubEntryRepo(),
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		"", // no media path — image fetching is skipped when empty
	)
}

// ── ImportStudio ──────────────────────────────────────────────────────────────

func TestImportStudio_DefaultMonitorMode(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-1",
		Name:        "Acme Studios",
		ContentType: domain.ContentTypeAdult,
		Monitored:   false,
		// MonitorMode deliberately omitted — should default to latest
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.MonitorMode != domain.MonitorLatest {
		t.Errorf("MonitorMode = %q, want %q", res.Studio.MonitorMode, domain.MonitorLatest)
	}
}

func TestImportStudio_ExplicitMonitorMode(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-2",
		Name:        "Full Collection Studios",
		ContentType: domain.ContentTypeAdult,
		MonitorMode: domain.MonitorAll,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.MonitorMode != domain.MonitorAll {
		t.Errorf("MonitorMode = %q, want %q", res.Studio.MonitorMode, domain.MonitorAll)
	}
}

func TestImportStudio_Idempotent(t *testing.T) {
	entryRepo := newStubEntryRepo()
	svc := metadata.New(nil, nil, entryRepo, &stubItemRepo{}, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, "")

	req := &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-3",
		Name:        "Once Only",
		ContentType: domain.ContentTypeAdult,
	}

	res1, err := svc.ImportStudio(context.Background(), req)
	if err != nil {
		t.Fatalf("first ImportStudio: %v", err)
	}

	// Seed the external ID repo with the saved entry so the second call finds it.
	seededRepo := &seededExternalIDRepo{id: res1.Studio.ID}
	svc2 := metadata.New(nil, nil, entryRepo, &stubItemRepo{}, &stubPersonRepo{}, &stubTagRepo{}, seededRepo, "")

	res2, err := svc2.ImportStudio(context.Background(), req)
	if err != nil {
		t.Fatalf("second ImportStudio: %v", err)
	}
	if res2.Studio.ID != res1.Studio.ID {
		t.Errorf("idempotent call returned different ID: %q vs %q", res2.Studio.ID, res1.Studio.ID)
	}
	if len(entryRepo.data) != 1 {
		t.Errorf("entry count = %d, want 1 (no duplicate created)", len(entryRepo.data))
	}
}

type seededExternalIDRepo struct{ id string }

func (r *seededExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return r.id, nil
}

// ── RefreshStudio ─────────────────────────────────────────────────────────────

// threeScenes returns 3 ExternalItems with distinct dates:
//
//	scene-1: 2023-01-01 (oldest)
//	scene-2: 2023-06-15 (newest)
//	scene-3: 2023-03-10 (middle)
func threeScenes() []*domain.ExternalItem {
	return []*domain.ExternalItem{
		{Source: domain.SourceStashDB, ExternalID: "scene-1", Title: "Scene A", Date: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Source: domain.SourceStashDB, ExternalID: "scene-2", Title: "Scene B", Date: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)},
		{Source: domain.SourceStashDB, ExternalID: "scene-3", Title: "Scene C", Date: time.Date(2023, 3, 10, 0, 0, 0, 0, time.UTC)},
	}
}

func refreshSvc(scenes []*domain.ExternalItem, entryRepo *stubEntryRepo, itemRepo *stubItemRepo) *metadata.Service {
	src := &stubSource{scenes: scenes, total: len(scenes)}
	return metadata.New(
		[]ports.MetadataSource{src},
		nil, // no job queue needed for refresh tests
		entryRepo,
		itemRepo,
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		"",
	)
}

func studioEntry(mode domain.MonitorMode, addedAt time.Time) *domain.LibraryEntry {
	return &domain.LibraryEntry{
		ID:          "entry-1",
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Test Studio",
		MonitorMode: mode,
		AddedAt:     addedAt,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceStashDB, Value: "studio-ext-1"}},
	}
}

func TestRefreshStudio_MonitorAll(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	if err := refreshSvc(threeScenes(), entryRepo, itemRepo).RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(itemRepo.items) != 3 {
		t.Fatalf("item count = %d, want 3", len(itemRepo.items))
	}
	for _, it := range itemRepo.items {
		if !it.Monitored {
			t.Errorf("item %q: monitored = false, want true (MonitorAll)", it.Title)
		}
	}
}

func TestRefreshStudio_MonitorNone(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorNone, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	if err := refreshSvc(threeScenes(), entryRepo, itemRepo).RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(itemRepo.items) != 3 {
		t.Fatalf("item count = %d, want 3", len(itemRepo.items))
	}
	for _, it := range itemRepo.items {
		if it.Monitored {
			t.Errorf("item %q: monitored = true, want false (MonitorNone)", it.Title)
		}
	}
}

func TestRefreshStudio_MonitorFuture(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	// AddedAt = 2023-02-01: scene-1 (Jan) is before, scene-2 (Jun) and scene-3 (Mar) are after.
	entry := studioEntry(domain.MonitorFuture, time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	if err := refreshSvc(threeScenes(), entryRepo, itemRepo).RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(itemRepo.items) != 3 {
		t.Fatalf("item count = %d, want 3", len(itemRepo.items))
	}
	monitored := map[string]bool{}
	for _, it := range itemRepo.items {
		monitored[it.ExternalIDs[0].Value] = it.Monitored
	}
	if monitored["scene-1"] {
		t.Error("scene-1 (Jan): monitored = true, want false (before AddedAt)")
	}
	if !monitored["scene-2"] {
		t.Error("scene-2 (Jun): monitored = false, want true (after AddedAt)")
	}
	if !monitored["scene-3"] {
		t.Error("scene-3 (Mar): monitored = false, want true (after AddedAt)")
	}
}

// ── AutoImport ────────────────────────────────────────────────────────────────

func TestImportStudio_AutoImport_EnqueuesJob(t *testing.T) {
	jobQueue := &stubJobQueue{}
	svc := metadata.New(
		nil, // no metadata sources
		jobQueue,
		newStubEntryRepo(),
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		"",
	)

	_, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-auto-1",
		Name:        "Auto Import Studio",
		ContentType: domain.ContentTypeAdult,
		AutoImport:  true,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if len(jobQueue.submitted) != 1 {
		t.Fatalf("submitted job count = %d, want 1", len(jobQueue.submitted))
	}
	if jobQueue.submitted[0] != "RefreshStudio" {
		t.Errorf("job name = %q, want %q", jobQueue.submitted[0], "RefreshStudio")
	}
}

func TestImportStudio_AutoImport_False_NoJob(t *testing.T) {
	jobQueue := &stubJobQueue{}
	svc := metadata.New(
		nil,
		jobQueue,
		newStubEntryRepo(),
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		"",
	)

	_, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-no-auto-1",
		Name:        "Manual Studio",
		ContentType: domain.ContentTypeAdult,
		AutoImport:  false,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if len(jobQueue.submitted) != 0 {
		t.Errorf("submitted job count = %d, want 0 (AutoImport=false)", len(jobQueue.submitted))
	}
}

func TestRefreshStudio_MonitorLatest(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorLatest, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	if err := refreshSvc(threeScenes(), entryRepo, itemRepo).RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(itemRepo.items) != 3 {
		t.Fatalf("item count = %d, want 3", len(itemRepo.items))
	}
	monitored := map[string]bool{}
	for _, it := range itemRepo.items {
		monitored[it.ExternalIDs[0].Value] = it.Monitored
	}
	// scene-2 has the latest date (2023-06-15) — only it should be monitored.
	if monitored["scene-1"] {
		t.Error("scene-1: monitored = true, want false")
	}
	if !monitored["scene-2"] {
		t.Error("scene-2 (latest): monitored = false, want true")
	}
	if monitored["scene-3"] {
		t.Error("scene-3: monitored = true, want false")
	}
}

// ── RefreshStudio enrichment ──────────────────────────────────────────────────

func scenesWithPeopleAndTags() []*domain.ExternalItem {
	return []*domain.ExternalItem{
		{
			Source:     domain.SourceStashDB,
			ExternalID: "scene-rich-1",
			Title:      "Rich Scene",
			Date:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Tags:       []string{"outdoor", "lesbian", "outdoor"}, // duplicate should collapse
			People: []*domain.ExternalPerson{
				{Source: domain.SourceStashDB, ExternalID: "perf-1", Name: "Alice", Role: domain.RolePerformer},
				{Source: domain.SourceStashDB, ExternalID: "perf-2", Name: "Bob", Role: domain.RolePerformer},
			},
		},
		{
			Source:     domain.SourceStashDB,
			ExternalID: "scene-rich-2",
			Title:      "Second Scene",
			Date:       time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			Tags:       []string{"outdoor"}, // same tag — should reuse, not duplicate
			People: []*domain.ExternalPerson{
				{Source: domain.SourceStashDB, ExternalID: "perf-1", Name: "Alice", Role: domain.RolePerformer},
			},
		},
	}
}

func TestRefreshStudio_ImportsPerformers(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	personRepo := &stubPersonRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src := &stubSource{scenes: scenesWithPeopleAndTags(), total: 2}
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, itemRepo, personRepo, &stubTagRepo{}, &stubExternalIDRepo{}, "")

	if err := svc.RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}

	// 2 unique performers across 2 scenes — should each be created once.
	if len(personRepo.saved) != 2 {
		t.Errorf("person records created = %d, want 2", len(personRepo.saved))
	}

	// First scene should have 2 performers linked.
	if len(itemRepo.items) != 2 {
		t.Fatalf("item count = %d, want 2", len(itemRepo.items))
	}
	if len(itemRepo.items[0].People) != 2 {
		t.Errorf("scene 1 people = %d, want 2", len(itemRepo.items[0].People))
	}
	// Second scene reuses perf-1 — still only 2 total Person records.
	if len(itemRepo.items[1].People) != 1 {
		t.Errorf("scene 2 people = %d, want 1", len(itemRepo.items[1].People))
	}
}

func TestRefreshStudio_ImportsTags(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	tagRepo := &stubTagRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src := &stubSource{scenes: scenesWithPeopleAndTags(), total: 2}
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, itemRepo, &stubPersonRepo{}, tagRepo, &stubExternalIDRepo{}, "")

	if err := svc.RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}

	// "outdoor" and "lesbian" — 2 unique tags despite appearing across both scenes.
	if len(tagRepo.saved) != 2 {
		t.Errorf("unique tags created = %d, want 2", len(tagRepo.saved))
	}
	// First scene has 2 unique tags (outdoor + lesbian; duplicate "outdoor" collapsed).
	if len(itemRepo.items[0].Tags) != 2 {
		t.Errorf("scene 1 tags = %d, want 2", len(itemRepo.items[0].Tags))
	}
	// Second scene has 1 tag (outdoor), reused from cache.
	if len(itemRepo.items[1].Tags) != 1 {
		t.Errorf("scene 2 tags = %d, want 1", len(itemRepo.items[1].Tags))
	}
}
