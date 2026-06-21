package metadata_test

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

func newService() *metadata.Service {
	return metadata.New(
		nil, // no metadata sources needed for import tests
		nil, // no job queue
		newStubEntryRepo(),
		nil, // no group repo needed for import tests
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		&stubImageDownloader{ext: ".jpg"},
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

func TestImportStudio_KindDefaultsToStudio(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-kind-default",
		Name:        "Default Kind Studio",
		ContentType: domain.ContentTypeAdult,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.Kind != domain.KindStudio {
		t.Errorf("Kind = %q, want %q", res.Studio.Kind, domain.KindStudio)
	}
}

func TestImportStudio_KindArtist(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  "artist-mbz-1",
		Name:        "Test Artist",
		ContentType: domain.ContentTypeMusic,
		Kind:        domain.KindArtist,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.Kind != domain.KindArtist {
		t.Errorf("Kind = %q, want %q", res.Studio.Kind, domain.KindArtist)
	}
}

func TestImportStudio_Idempotent(t *testing.T) {
	entryRepo := newStubEntryRepo()
	svc := metadata.New(nil, nil, entryRepo, nil, &stubItemRepo{}, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, nil)

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
	svc2 := metadata.New(nil, nil, entryRepo, nil, &stubItemRepo{}, &stubPersonRepo{}, &stubTagRepo{}, seededRepo, nil)

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
		nil, // no group repo needed for RefreshStudio tests
		itemRepo,
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		&stubImageDownloader{ext: ".jpg"},
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
		nil,
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
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

func TestImportStudio_AutoImport_KindArtist_EnqueuesRefreshArtist(t *testing.T) {
	jobQueue := &stubJobQueue{}
	svc := metadata.New(
		nil,
		jobQueue,
		newStubEntryRepo(),
		nil,
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
	)

	_, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  "artist-auto-1",
		Name:        "Fleetwood Mac",
		ContentType: domain.ContentTypeMusic,
		Kind:        domain.KindArtist,
		AutoImport:  true,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if len(jobQueue.submitted) != 1 {
		t.Fatalf("submitted job count = %d, want 1", len(jobQueue.submitted))
	}
	if jobQueue.submitted[0] != "RefreshArtist" {
		t.Errorf("job name = %q, want RefreshArtist", jobQueue.submitted[0])
	}
}

func TestImportStudio_AutoImport_False_NoJob(t *testing.T) {
	jobQueue := &stubJobQueue{}
	svc := metadata.New(
		nil,
		jobQueue,
		newStubEntryRepo(),
		nil,
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
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
	type itemState struct {
		monitored bool
		status    domain.ItemStatus
	}
	state := map[string]itemState{}
	for _, it := range itemRepo.items {
		state[it.ExternalIDs[0].Value] = itemState{monitored: it.Monitored, status: it.Status}
	}
	// scene-2 has the latest date (2023-06-15) — only it should be monitored+wanted;
	// others must be unmonitored and missing (not wanted).
	if state["scene-1"].monitored {
		t.Error("scene-1: monitored = true, want false")
	}
	if state["scene-1"].status != domain.StatusMissing {
		t.Errorf("scene-1: status = %q, want %q", state["scene-1"].status, domain.StatusMissing)
	}
	if !state["scene-2"].monitored {
		t.Error("scene-2 (latest): monitored = false, want true")
	}
	if state["scene-2"].status != domain.StatusWanted {
		t.Errorf("scene-2 (latest): status = %q, want %q", state["scene-2"].status, domain.StatusWanted)
	}
	if state["scene-3"].monitored {
		t.Error("scene-3: monitored = true, want false")
	}
	if state["scene-3"].status != domain.StatusMissing {
		t.Errorf("scene-3: status = %q, want %q", state["scene-3"].status, domain.StatusMissing)
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
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, nil, itemRepo, personRepo, &stubTagRepo{}, &stubExternalIDRepo{}, nil)

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
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, nil, itemRepo, &stubPersonRepo{}, tagRepo, &stubExternalIDRepo{}, nil)

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

func artistEntry(mode domain.MonitorMode, addedAt time.Time) *domain.LibraryEntry {
	return &domain.LibraryEntry{
		ID:          "artist-entry-1",
		ContentType: domain.ContentTypeMusic,
		Kind:        domain.KindArtist,
		Name:        "Test Artist",
		MonitorMode: mode,
		AddedAt:     addedAt,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceMusicBrainz, Value: "artist-mbz-1"}},
	}
}

func twoAlbumsWithTracks() (*stubMusicSource, []*domain.ExternalGroup, map[string][]*domain.ExternalItem) {
	albums := []*domain.ExternalGroup{
		{Source: domain.SourceMusicBrainz, ExternalID: "album-1", Title: "Album One", Year: 2020, PrimaryType: "Album"},
		{Source: domain.SourceMusicBrainz, ExternalID: "album-2", Title: "Album Two", Year: 2022, PrimaryType: "Album"},
	}
	tracks := map[string][]*domain.ExternalItem{
		"album-1": {
			{Source: domain.SourceMusicBrainz, ExternalID: "track-1", Title: "Track A1", Date: time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)},
			{Source: domain.SourceMusicBrainz, ExternalID: "track-2", Title: "Track A2", Date: time.Date(2020, 3, 2, 0, 0, 0, 0, time.UTC)},
		},
		"album-2": {
			{Source: domain.SourceMusicBrainz, ExternalID: "track-3", Title: "Track B1", Date: time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)},
			{Source: domain.SourceMusicBrainz, ExternalID: "track-4", Title: "Track B2", Date: time.Date(2022, 6, 15, 0, 0, 0, 0, time.UTC)},
		},
	}
	return &stubMusicSource{albums: albums, tracks: tracks}, albums, tracks
}

func artistRefreshSvc(src *stubMusicSource, entryRepo *stubEntryRepo, groupRepo *stubGroupRepo, itemRepo *stubItemRepo) *metadata.Service {
	return metadata.New(
		[]ports.MetadataSource{src},
		nil,
		entryRepo,
		groupRepo,
		itemRepo,
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
	)
}

// ── RefreshArtist tests ───────────────────────────────────────────────────────

func TestRefreshArtist_CreatesGroupsAndTracks(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	svc := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo)

	if err := svc.RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	if len(groupRepo.groups) != 2 {
		t.Errorf("group count = %d, want 2", len(groupRepo.groups))
	}
	if len(itemRepo.items) != 4 {
		t.Errorf("item count = %d, want 4", len(itemRepo.items))
	}
}

func TestRefreshArtist_MonitorAll(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	if err := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo).RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	for _, it := range itemRepo.items {
		if !it.Monitored {
			t.Errorf("track %q: monitored = false, want true (MonitorAll)", it.Title)
		}
	}
}

func TestRefreshArtist_MonitorNone(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorNone, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	if err := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo).RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	for _, it := range itemRepo.items {
		if it.Monitored {
			t.Errorf("track %q: monitored = true, want false (MonitorNone)", it.Title)
		}
	}
}

func TestRefreshArtist_MonitorFuture(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	// AddedAt between album-1 tracks (Mar 2020) and album-2 tracks (Jun 2022).
	entry := artistEntry(domain.MonitorFuture, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	if err := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo).RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	monitored := map[string]bool{}
	for _, it := range itemRepo.items {
		monitored[it.ExternalIDs[0].Value] = it.Monitored
	}
	if monitored["track-1"] || monitored["track-2"] {
		t.Error("album-1 tracks (2020): want unmonitored (before AddedAt)")
	}
	if !monitored["track-3"] || !monitored["track-4"] {
		t.Error("album-2 tracks (2022): want monitored (after AddedAt)")
	}
}

func TestRefreshArtist_MonitorLatest(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorLatest, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	if err := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo).RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	// track-4 (2022-06-15) is the newest across all albums.
	monitored := map[string]bool{}
	status := map[string]domain.ItemStatus{}
	for _, it := range itemRepo.items {
		extID := it.ExternalIDs[0].Value
		monitored[extID] = it.Monitored
		status[extID] = it.Status
	}
	for _, extID := range []string{"track-1", "track-2", "track-3"} {
		if monitored[extID] {
			t.Errorf("%s: monitored = true, want false", extID)
		}
		if status[extID] != domain.StatusMissing {
			t.Errorf("%s: status = %q, want missing", extID, status[extID])
		}
	}
	if !monitored["track-4"] {
		t.Error("track-4 (latest): monitored = false, want true")
	}
	if status["track-4"] != domain.StatusWanted {
		t.Errorf("track-4 (latest): status = %q, want wanted", status["track-4"])
	}
}

func TestRefreshArtist_SkipsDuplicates(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	svc := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo)

	if err := svc.RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("first RefreshArtist: %v", err)
	}
	firstGroups := len(groupRepo.groups)
	firstItems := len(itemRepo.items)

	// Seed the external ID repo so the second call sees everything as existing.
	seeded := &seededArtistExternalIDRepo{
		groupIDs: map[string]string{
			"mbz:album-1": groupRepo.groups[0].ID,
			"mbz:album-2": groupRepo.groups[1].ID,
		},
		itemIDs: make(map[string]string),
	}
	for _, it := range itemRepo.items {
		seeded.itemIDs["mbz:"+it.ExternalIDs[0].Value] = it.ID
	}
	svc2 := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, groupRepo, itemRepo, &stubPersonRepo{}, &stubTagRepo{}, seeded, nil)

	if err := svc2.RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("second RefreshArtist: %v", err)
	}
	if len(groupRepo.groups) != firstGroups {
		t.Errorf("group count after second refresh = %d, want %d (no duplicates)", len(groupRepo.groups), firstGroups)
	}
	if len(itemRepo.items) != firstItems {
		t.Errorf("item count after second refresh = %d, want %d (no duplicates)", len(itemRepo.items), firstItems)
	}
}

// ── FetchArtistDiscography ────────────────────────────────────────────────────

func TestFetchArtistDiscography_ReturnsGroups(t *testing.T) {
	src, albums, _ := twoAlbumsWithTracks()
	svc := metadata.New([]ports.MetadataSource{src}, nil, newStubEntryRepo(), nil, &stubItemRepo{}, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, nil)

	groups, total, err := svc.FetchArtistDiscography(context.Background(), domain.SourceMusicBrainz, domain.ContentTypeMusic, "artist-mbz-1", 1, 50)
	if err != nil {
		t.Fatalf("FetchArtistDiscography: %v", err)
	}
	if total != len(albums) {
		t.Errorf("total = %d, want %d", total, len(albums))
	}
	if len(groups) != len(albums) {
		t.Errorf("groups = %d, want %d", len(groups), len(albums))
	}
}

func TestFetchArtistDiscography_UnknownSource(t *testing.T) {
	svc := newService()
	_, _, err := svc.FetchArtistDiscography(context.Background(), "nonexistent", domain.ContentTypeMusic, "some-id", 1, 50)
	if err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if !errs.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ── ImportAlbum ───────────────────────────────────────────────────────────────

func importAlbumSvc(src *stubMusicSource, entryRepo *stubEntryRepo, groupRepo *stubGroupRepo, itemRepo *stubItemRepo) *metadata.Service {
	return metadata.New(
		[]ports.MetadataSource{src},
		nil,
		entryRepo,
		groupRepo,
		itemRepo,
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
	)
}

func TestImportAlbum_CreatesGroupAndTracks(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, albums, _ := twoAlbumsWithTracks()
	svc := importAlbumSvc(src, entryRepo, groupRepo, itemRepo)

	g, err := svc.ImportAlbum(context.Background(), &metadata.ImportAlbumRequest{
		Source:         domain.SourceMusicBrainz,
		ExternalID:     albums[0].ExternalID,
		LibraryEntryID: entry.ID,
		Title:          albums[0].Title,
		Year:           albums[0].Year,
		Monitored:      true,
		MonitorMode:    domain.MonitorAll,
	})
	if err != nil {
		t.Fatalf("ImportAlbum: %v", err)
	}
	if len(groupRepo.groups) != 1 {
		t.Errorf("group count = %d, want 1", len(groupRepo.groups))
	}
	if g.Title != albums[0].Title {
		t.Errorf("group title = %q, want %q", g.Title, albums[0].Title)
	}
	// album-1 has 2 tracks
	if len(itemRepo.items) != 2 {
		t.Errorf("item count = %d, want 2", len(itemRepo.items))
	}
	for _, it := range itemRepo.items {
		if it.GroupID != g.ID {
			t.Errorf("item %q: GroupID = %q, want %q", it.Title, it.GroupID, g.ID)
		}
		if it.LibraryEntryID != entry.ID {
			t.Errorf("item %q: LibraryEntryID = %q, want %q", it.Title, it.LibraryEntryID, entry.ID)
		}
	}
}

func TestImportAlbum_Idempotent(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, albums, _ := twoAlbumsWithTracks()
	svc := importAlbumSvc(src, entryRepo, groupRepo, itemRepo)

	req := &metadata.ImportAlbumRequest{
		Source:         domain.SourceMusicBrainz,
		ExternalID:     albums[0].ExternalID,
		LibraryEntryID: entry.ID,
		Title:          albums[0].Title,
		Year:           albums[0].Year,
		Monitored:      true,
		MonitorMode:    domain.MonitorAll,
	}
	g1, err := svc.ImportAlbum(context.Background(), req)
	if err != nil {
		t.Fatalf("first ImportAlbum: %v", err)
	}

	// Seed external IDs so the second call finds the existing group.
	seeded := &seededArtistExternalIDRepo{
		groupIDs: map[string]string{"mbz:" + albums[0].ExternalID: g1.ID},
		itemIDs:  make(map[string]string),
	}
	svc2 := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, groupRepo, itemRepo, &stubPersonRepo{}, &stubTagRepo{}, seeded, nil)

	g2, err := svc2.ImportAlbum(context.Background(), req)
	if err != nil {
		t.Fatalf("second ImportAlbum: %v", err)
	}
	if g2.ID != g1.ID {
		t.Errorf("idempotent call returned different ID: %q vs %q", g2.ID, g1.ID)
	}
	if len(groupRepo.groups) != 1 {
		t.Errorf("group count = %d, want 1 (no duplicate)", len(groupRepo.groups))
	}
	if len(itemRepo.items) != 2 {
		t.Errorf("item count = %d, want 2 (no duplicate tracks)", len(itemRepo.items))
	}
}

func TestImportAlbum_UnknownSource(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	src, albums, _ := twoAlbumsWithTracks()
	svc := importAlbumSvc(src, entryRepo, groupRepo, itemRepo)

	_, err := svc.ImportAlbum(context.Background(), &metadata.ImportAlbumRequest{
		Source:         "nonexistent",
		ExternalID:     albums[0].ExternalID,
		LibraryEntryID: "entry-1",
		Title:          "Test Album",
	})
	if err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if !errs.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ── RefreshStudio image downloader ────────────────────────────────────────────

func TestRefreshStudio_FetchesImageForItemsWithURL(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	scenes := []*domain.ExternalItem{
		{
			Source: domain.SourceStashDB, ExternalID: "s-1", Title: "Scene 1",
			Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), ImageURL: "https://example.com/cover.jpg",
		},
	}
	dl := &stubImageDownloader{ext: ".jpg"}
	src := &stubSource{scenes: scenes, total: 1}
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, nil, itemRepo, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, dl)

	if err := svc.RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(dl.calls) != 1 || dl.calls[0] != "https://example.com/cover.jpg" {
		t.Errorf("Download calls = %v, want [https://example.com/cover.jpg]", dl.calls)
	}
	if itemRepo.items[0].CoverPath != ".jpg" {
		t.Errorf("CoverPath = %q, want .jpg", itemRepo.items[0].CoverPath)
	}
}

func TestRefreshStudio_SkipsImageForItemsWithoutURL(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	dl := &stubImageDownloader{ext: ".jpg"}
	src := &stubSource{scenes: threeScenes(), total: 3}
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, nil, itemRepo, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, dl)

	if err := svc.RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio: %v", err)
	}
	if len(dl.calls) != 0 {
		t.Errorf("Download called %d times, want 0 (no ImageURL on scenes)", len(dl.calls))
	}
}

func TestRefreshStudio_ImageDownloaderFailure(t *testing.T) {
	entryRepo := newStubEntryRepo()
	itemRepo := &stubItemRepo{}
	entry := studioEntry(domain.MonitorAll, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	scenes := []*domain.ExternalItem{
		{
			Source: domain.SourceStashDB, ExternalID: "s-1", Title: "Scene 1",
			Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), ImageURL: "https://example.com/cover.jpg",
		},
	}
	dl := &stubImageDownloader{ext: ""}
	src := &stubSource{scenes: scenes, total: 1}
	svc := metadata.New([]ports.MetadataSource{src}, nil, entryRepo, nil, itemRepo, &stubPersonRepo{}, &stubTagRepo{}, &stubExternalIDRepo{}, dl)

	if err := svc.RefreshStudio(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshStudio failed on downloader error: %v", err)
	}
	if len(itemRepo.items) != 1 {
		t.Fatalf("item count = %d, want 1 (refresh must complete despite downloader failure)", len(itemRepo.items))
	}
	if itemRepo.items[0].CoverPath != "" {
		t.Errorf("CoverPath = %q, want empty on downloader failure", itemRepo.items[0].CoverPath)
	}
}

func TestRefreshArtist_HeroImagePrefersAudioDB(t *testing.T) {
	// Two sources both return a hero image for the same artist.
	// mbz is priority 0 (registered first), audiodb is priority 1.
	// The fix must pick the audiodb image regardless of registration order.
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := &domain.LibraryEntry{
		ID:          "artist-image-test",
		ContentType: domain.ContentTypeMusic,
		Kind:        domain.KindArtist,
		Name:        "Whitesnake",
		MonitorMode: domain.MonitorAll,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceMusicBrainz, Value: "whitesnake-mbid"}},
	}
	entryRepo.data[entry.ID] = entry

	mbzSrc := &stubMusicSource{
		albums: []*domain.ExternalGroup{},
		tracks: map[string][]*domain.ExternalItem{},
		findItem: &domain.ExternalItem{
			Source: domain.SourceMusicBrainz,
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypeHero, URL: "https://mbz.example.com/hero.jpg"},
			},
		},
	}
	audiodbSrc := &stubImageSource{
		sourceName: string(domain.SourceTheAudioDB),
		findItem: &domain.ExternalItem{
			Source: domain.SourceTheAudioDB,
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypeHero, URL: "https://audiodb.example.com/hero.jpg"},
			},
		},
	}

	dl := &stubImageDownloader{ext: ".jpg"}
	svc := metadata.New(
		[]ports.MetadataSource{mbzSrc, audiodbSrc},
		nil,
		entryRepo,
		groupRepo,
		itemRepo,
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		dl,
	)

	if err := svc.RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}
	if len(dl.calls) == 0 {
		t.Fatal("expected hero image download, got none")
	}
	if dl.calls[0] != "https://audiodb.example.com/hero.jpg" {
		t.Errorf("Download URL = %q, want audiodb hero image", dl.calls[0])
	}
	if entry.ImagePath != ".jpg" {
		t.Errorf("entry.ImagePath = %q, want .jpg", entry.ImagePath)
	}
}

func TestRefreshArtist_GroupLinkedToItem(t *testing.T) {
	entryRepo := newStubEntryRepo()
	groupRepo := &stubGroupRepo{}
	itemRepo := &stubItemRepo{}
	entry := artistEntry(domain.MonitorAll, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	entryRepo.data[entry.ID] = entry

	src, _, _ := twoAlbumsWithTracks()
	if err := artistRefreshSvc(src, entryRepo, groupRepo, itemRepo).RefreshArtist(context.Background(), entry.ID, nil); err != nil {
		t.Fatalf("RefreshArtist: %v", err)
	}

	// Build a map from internal group ID to album external ID for verification.
	groupByID := map[string]*domain.Group{}
	for _, g := range groupRepo.groups {
		groupByID[g.ID] = g
	}

	for _, it := range itemRepo.items {
		if it.GroupID == "" {
			t.Errorf("item %q: GroupID is empty", it.Title)
			continue
		}
		if _, ok := groupByID[it.GroupID]; !ok {
			t.Errorf("item %q: GroupID %q does not match any saved group", it.Title, it.GroupID)
		}
	}
}
