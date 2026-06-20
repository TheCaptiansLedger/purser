package db

import (
	"context"
	"database/sql"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// ── Open / migrations ─────────────────────────────────────────────────────────

func TestOpen_CreatesSchema(t *testing.T) {
	database := setupTestDB(t)

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("schema_migrations missing: %v", err)
	}
	if count != 7 {
		t.Errorf("migration count = %d, want 7", count)
	}

	tables := []string{
		"library_entries", "groups", "items",
		"people", "people_aliases", "item_people",
		"external_ids", "tags", "item_tags", "entry_tags",
		"media_files", "releases", "downloads",
		"entry_people",
	}
	for _, table := range tables {
		var n int
		if err := database.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

func TestOpen_IdempotentMigrations(t *testing.T) {
	// Rerunning migrations on an already-migrated DB should not error.
	database := setupTestDB(t)
	if err := runMigrations(database); err != nil {
		t.Fatalf("second runMigrations: %v", err)
	}
}

// ── LibraryEntryRepo ──────────────────────────────────────────────────────────

func TestLibraryEntryRepo_SaveAndGet(t *testing.T) {
	repo := NewLibraryEntryRepo(setupTestDB(t))
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Evil Angel",
		SortName:    "Evil Angel",
		Monitored:   true,
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceStashDB, Value: "abc-123"}},
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if e.ID == "" {
		t.Fatal("ID should be set by Save")
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Evil Angel" {
		t.Errorf("Name = %q, want Evil Angel", got.Name)
	}
	if got.ContentType != domain.ContentTypeAdult {
		t.Errorf("ContentType = %q, want adult", got.ContentType)
	}
	if !got.Monitored {
		t.Error("Monitored should be true")
	}
	if len(got.ExternalIDs) != 1 || got.ExternalIDs[0].Value != "abc-123" {
		t.Errorf("ExternalIDs = %v, want [{stashdb abc-123}]", got.ExternalIDs)
	}
}

func TestLibraryEntryRepo_Update(t *testing.T) {
	repo := NewLibraryEntryRepo(setupTestDB(t))
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Original",
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	e.Name = "Updated"
	e.Monitored = true
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name after update = %q, want Updated", got.Name)
	}
	if !got.Monitored {
		t.Error("Monitored should be true after update")
	}
}

func TestLibraryEntryRepo_List_Filters(t *testing.T) {
	repo := NewLibraryEntryRepo(setupTestDB(t))
	ctx := context.Background()

	entries := []*domain.LibraryEntry{
		{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "A", Monitored: true, MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive},
		{ContentType: domain.ContentTypeAdult, Kind: domain.KindNetwork, Name: "B", Monitored: false, MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive},
		{ContentType: domain.ContentTypeTV, Kind: domain.KindSeries, Name: "C", Monitored: true, MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive},
	}
	for _, e := range entries {
		if err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	all, total, err := repo.List(ctx, ports.LibraryFilter{Limit: 50})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Errorf("all: total=%d, len=%d, want 3,3", total, len(all))
	}

	adult, atotal, err := repo.List(ctx, ports.LibraryFilter{ContentType: domain.ContentTypeAdult, Limit: 50})
	if err != nil {
		t.Fatalf("List adult: %v", err)
	}
	if atotal != 2 || len(adult) != 2 {
		t.Errorf("adult: total=%d, len=%d, want 2,2", atotal, len(adult))
	}

	monTrue := true
	mon, _, err := repo.List(ctx, ports.LibraryFilter{Monitored: &monTrue, Limit: 50})
	if err != nil {
		t.Fatalf("List monitored: %v", err)
	}
	if len(mon) != 2 {
		t.Errorf("monitored count = %d, want 2", len(mon))
	}

	srch, stotal, _ := repo.List(ctx, ports.LibraryFilter{Search: "A", Limit: 50})
	if stotal != 1 || len(srch) != 1 {
		t.Errorf("search: total=%d, len=%d, want 1,1", stotal, len(srch))
	}
}

func TestLibraryEntryRepo_Delete(t *testing.T) {
	repo := NewLibraryEntryRepo(setupTestDB(t))
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Delete Me",
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, e.ID); err == nil {
		t.Error("Get after delete should error, got nil")
	}
}

func TestLibraryEntryRepo_EntryPeople(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	personRepo := NewPersonRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "The Beatles", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	if err := entryRepo.Save(ctx, entry); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	person := &domain.Person{Name: "John Lennon", MonitorMode: domain.MonitorAll}
	if err := personRepo.Save(ctx, person); err != nil {
		t.Fatalf("Save person: %v", err)
	}

	// SavePerson with start/end dates
	ep := domain.EntryPerson{
		PersonID:  person.ID,
		Role:      "member",
		StartDate: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(1970, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	if err := entryRepo.SavePerson(ctx, entry.ID, ep); err != nil {
		t.Fatalf("SavePerson: %v", err)
	}

	// GetPeople returns the link with person details and dates
	people, err := entryRepo.GetPeople(ctx, entry.ID)
	if err != nil {
		t.Fatalf("GetPeople: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("people count = %d, want 1", len(people))
	}
	if people[0].PersonID != person.ID {
		t.Errorf("PersonID = %q, want %q", people[0].PersonID, person.ID)
	}
	if people[0].Role != "member" {
		t.Errorf("Role = %q, want member", people[0].Role)
	}
	if people[0].StartDate.IsZero() {
		t.Error("StartDate should be set")
	}
	if people[0].EndDate.IsZero() {
		t.Error("EndDate should be set")
	}
	if people[0].Person == nil || people[0].Person.Name != "John Lennon" {
		t.Errorf("Person.Name = %v, want John Lennon", people[0].Person)
	}

	// Get entry also loads people
	got, err := entryRepo.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.People) != 1 {
		t.Errorf("entry.People count = %d, want 1", len(got.People))
	}

	// Upsert updates dates for same (entry, person, role) triple
	ep.StartDate = time.Date(1962, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := entryRepo.SavePerson(ctx, entry.ID, ep); err != nil {
		t.Fatalf("SavePerson upsert: %v", err)
	}
	people, _ = entryRepo.GetPeople(ctx, entry.ID)
	if len(people) != 1 {
		t.Errorf("after upsert, people count = %d, want 1", len(people))
	}

	// RemovePerson deletes the link
	if err := entryRepo.RemovePerson(ctx, entry.ID, person.ID, "member"); err != nil {
		t.Fatalf("RemovePerson: %v", err)
	}
	people, _ = entryRepo.GetPeople(ctx, entry.ID)
	if len(people) != 0 {
		t.Errorf("after remove, people count = %d, want 0", len(people))
	}
}

func TestLibraryEntryRepo_List_FilterByPersonID(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	personRepo := NewPersonRepo(database)
	ctx := context.Background()

	beatles := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	wings := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Wings", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, beatles) //nolint:errcheck
	entryRepo.Save(ctx, wings)   //nolint:errcheck

	paul := &domain.Person{Name: "Paul McCartney", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, paul) //nolint:errcheck

	entryRepo.SavePerson(ctx, beatles.ID, domain.EntryPerson{PersonID: paul.ID, Role: "member"}) //nolint:errcheck

	res, total, err := entryRepo.List(ctx, ports.LibraryFilter{PersonID: paul.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by PersonID: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("filtered by PersonID: total=%d, len=%d, want 1", total, len(res))
	}
	if res[0].Name != "The Beatles" {
		t.Errorf("result name = %q, want The Beatles", res[0].Name)
	}
}

// ── PersonRepo ────────────────────────────────────────────────────────────────

func TestPersonRepo_SaveGetWithAliases(t *testing.T) {
	repo := NewPersonRepo(setupTestDB(t))
	ctx := context.Background()

	p := &domain.Person{
		Name:        "Jane Doe",
		SortName:    "Doe, Jane",
		Monitored:   true,
		MonitorMode: domain.MonitorAll,
		Aliases:     []string{"J. Doe", "JD"},
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceStashDB, Value: "performer-uuid"}},
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID should be set by Save")
	}

	got, err := repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Jane Doe" {
		t.Errorf("Name = %q, want Jane Doe", got.Name)
	}
	if len(got.Aliases) != 2 {
		t.Errorf("Aliases count = %d, want 2", len(got.Aliases))
	}
	if len(got.ExternalIDs) != 1 {
		t.Errorf("ExternalIDs count = %d, want 1", len(got.ExternalIDs))
	}
}

func TestPersonRepo_List_BySearch(t *testing.T) {
	repo := NewPersonRepo(setupTestDB(t))
	ctx := context.Background()

	people := []*domain.Person{
		{Name: "Alice Smith", MonitorMode: domain.MonitorAll},
		{Name: "Bob Jones", MonitorMode: domain.MonitorAll, Aliases: []string{"Bobby"}},
		{Name: "Carol White", MonitorMode: domain.MonitorAll},
	}
	for _, p := range people {
		if err := repo.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// Search by name
	res, total, err := repo.List(ctx, ports.PersonFilter{Search: "alice", Limit: 50})
	if err != nil {
		t.Fatalf("List search name: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("search alice: total=%d, len=%d, want 1", total, len(res))
	}

	// Search by alias
	res, total, err = repo.List(ctx, ports.PersonFilter{Search: "bobby", Limit: 50})
	if err != nil {
		t.Fatalf("List search alias: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("search bobby alias: total=%d, len=%d, want 1", total, len(res))
	}
}

// ── ItemRepo ──────────────────────────────────────────────────────────────────

func TestItemRepo_SaveAndGet(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	personRepo := NewPersonRepo(database)
	ctx := context.Background()

	// Need a library entry first (FK constraint)
	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Studio",
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
	}
	if err := entryRepo.Save(ctx, entry); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	// Create a person for the item_people junction
	person := &domain.Person{Name: "Jane Doe", MonitorMode: domain.MonitorAll}
	if err := personRepo.Save(ctx, person); err != nil {
		t.Fatalf("Save person: %v", err)
	}

	item := &domain.Item{
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: entry.ID,
		Title:          "Test Scene",
		Monitored:      true,
		Status:         domain.StatusWanted,
		People:         []domain.ItemPerson{{PersonID: person.ID, Role: domain.RolePerformer}},
	}
	if err := itemRepo.Save(ctx, item); err != nil {
		t.Fatalf("Save item: %v", err)
	}

	got, err := itemRepo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get item: %v", err)
	}
	if got.Title != "Test Scene" {
		t.Errorf("Title = %q, want Test Scene", got.Title)
	}
	if len(got.People) != 1 {
		t.Errorf("People count = %d, want 1", len(got.People))
	}
	if got.People[0].Role != domain.RolePerformer {
		t.Errorf("role = %q, want performer", got.People[0].Role)
	}
}

func TestItemRepo_List_ByPersonID(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	personRepo := NewPersonRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	p1 := &domain.Person{Name: "Alice", MonitorMode: domain.MonitorAll}
	p2 := &domain.Person{Name: "Bob", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, p1) //nolint:errcheck
	personRepo.Save(ctx, p2) //nolint:errcheck

	// Item with p1 only
	i1 := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene 1", Status: domain.StatusWanted, People: []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}}}
	// Item with both
	i2 := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene 2", Status: domain.StatusWanted, People: []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}, {PersonID: p2.ID, Role: domain.RolePerformer}}}
	itemRepo.Save(ctx, i1) //nolint:errcheck
	itemRepo.Save(ctx, i2) //nolint:errcheck

	res, total, err := itemRepo.List(ctx, ports.ItemFilter{PersonID: p1.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by person: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("p1 item total = %d, len = %d, want 2", total, len(res))
	}

	res, total, err = itemRepo.List(ctx, ports.ItemFilter{PersonID: p2.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by person 2: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("p2 item total = %d, want 1", total)
	}
}

// ── TagRepo ───────────────────────────────────────────────────────────────────

func TestTagRepo_SaveListDelete(t *testing.T) {
	repo := NewTagRepo(setupTestDB(t))
	ctx := context.Background()

	t1 := &domain.Tag{Name: "blonde", Scope: domain.TagScopeMetadata}
	t2 := &domain.Tag{Name: "favourite", Scope: domain.TagScopeUser}
	for _, tag := range []*domain.Tag{t1, t2} {
		if err := repo.Save(ctx, tag); err != nil {
			t.Fatalf("Save tag: %v", err)
		}
	}

	all, err := repo.List(ctx, ports.TagFilter{})
	if err != nil {
		t.Fatalf("List all tags: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all tags = %d, want 2", len(all))
	}

	user, err := repo.List(ctx, ports.TagFilter{Scope: domain.TagScopeUser})
	if err != nil {
		t.Fatalf("List user tags: %v", err)
	}
	if len(user) != 1 || user[0].Name != "favourite" {
		t.Errorf("user tags = %v, want [favourite]", user)
	}

	if err := repo.Delete(ctx, t1.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	remaining, _ := repo.List(ctx, ports.TagFilter{})
	if len(remaining) != 1 {
		t.Errorf("after delete, tag count = %d, want 1", len(remaining))
	}
}

// ── GroupRepo ─────────────────────────────────────────────────────────────────

func TestGroupRepo_SaveGetListDelete(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	repo := NewGroupRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeTV, Kind: domain.KindSeries,
		Name: "Breaking Bad", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	if err := entryRepo.Save(ctx, entry); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	groups := []*domain.Group{
		{LibraryEntryID: entry.ID, Title: "Season 1", Number: 1, Year: 2008, Monitored: true, MonitorMode: domain.MonitorAll},
		{LibraryEntryID: entry.ID, Title: "Season 2", Number: 2, Year: 2009, Monitored: false, MonitorMode: domain.MonitorNone},
	}
	for _, g := range groups {
		if err := repo.Save(ctx, g); err != nil {
			t.Fatalf("Save group: %v", err)
		}
		if g.ID == "" {
			t.Fatal("group ID should be set")
		}
	}

	// Get
	got, err := repo.Get(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("Get group: %v", err)
	}
	if got.Title != "Season 1" {
		t.Errorf("Title = %q, want Season 1", got.Title)
	}
	if got.Number != 1 {
		t.Errorf("Number = %d, want 1", got.Number)
	}

	// List all
	all, err := repo.List(ctx, ports.GroupFilter{LibraryEntryID: entry.ID})
	if err != nil {
		t.Fatalf("List groups: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("group count = %d, want 2", len(all))
	}

	// List monitored only
	monTrue := true
	mon, err := repo.List(ctx, ports.GroupFilter{LibraryEntryID: entry.ID, Monitored: &monTrue})
	if err != nil {
		t.Fatalf("List monitored: %v", err)
	}
	if len(mon) != 1 {
		t.Errorf("monitored group count = %d, want 1", len(mon))
	}

	// Update
	groups[0].Title = "Season One"
	if err := repo.Save(ctx, groups[0]); err != nil {
		t.Fatalf("Update group: %v", err)
	}
	got, _ = repo.Get(ctx, groups[0].ID)
	if got.Title != "Season One" {
		t.Errorf("after update Title = %q, want Season One", got.Title)
	}

	// Delete
	if err := repo.Delete(ctx, groups[0].ID); err != nil {
		t.Fatalf("Delete group: %v", err)
	}
	if _, err := repo.Get(ctx, groups[0].ID); err == nil {
		t.Error("Get after delete should error")
	}
}

// ── MediaFileRepo ─────────────────────────────────────────────────────────────

func TestMediaFileRepo_SaveAndGet(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	mfRepo := NewMediaFileRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	item := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID,
		Title: "Scene", Status: domain.StatusWanted,
	}
	itemRepo.Save(ctx, item) //nolint:errcheck

	mf := &domain.MediaFile{
		ItemID:    item.ID,
		Path:      "/media/adult/Studio/scene.mp4",
		Size:      1_000_000_000,
		OSHash:    "abc123def456",
		Quality:   domain.Quality1080,
		Codec:     "h264",
		Container: "mp4",
	}
	if err := mfRepo.Save(ctx, mf); err != nil {
		t.Fatalf("Save media file: %v", err)
	}
	if mf.ID == "" {
		t.Fatal("ID should be set")
	}

	// GetByItemID
	got, err := mfRepo.GetByItemID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetByItemID: %v", err)
	}
	if got.Path != mf.Path {
		t.Errorf("Path = %q, want %q", got.Path, mf.Path)
	}
	if got.Quality != domain.Quality1080 {
		t.Errorf("Quality = %q, want 1080p", got.Quality)
	}

	// GetByOSHash
	got, err = mfRepo.GetByOSHash(ctx, "abc123def456")
	if err != nil {
		t.Fatalf("GetByOSHash: %v", err)
	}
	if got.ItemID != item.ID {
		t.Errorf("ItemID = %q, want %q", got.ItemID, item.ID)
	}

	// Delete
	if err := mfRepo.Delete(ctx, mf.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := mfRepo.GetByItemID(ctx, item.ID); err == nil {
		t.Error("Get after delete should error")
	}
}

func TestPersonRepo_Delete(t *testing.T) {
	repo := NewPersonRepo(setupTestDB(t))
	ctx := context.Background()

	p := &domain.Person{Name: "Delete Me", MonitorMode: domain.MonitorAll}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, p.ID); err == nil {
		t.Error("Get after delete should error")
	}
}

func TestPersonRepo_List_MonitoredFilter(t *testing.T) {
	repo := NewPersonRepo(setupTestDB(t))
	ctx := context.Background()

	monitored := true
	people := []*domain.Person{
		{Name: "Alice", MonitorMode: domain.MonitorAll, Monitored: true},
		{Name: "Bob", MonitorMode: domain.MonitorAll, Monitored: false},
		{Name: "Carol", MonitorMode: domain.MonitorAll, Monitored: true},
	}
	for _, p := range people {
		if err := repo.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	res, total, err := repo.List(ctx, ports.PersonFilter{Monitored: &monitored, Limit: 50})
	if err != nil {
		t.Fatalf("List monitored: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("monitored total = %d, len = %d, want 2", total, len(res))
	}
}

func TestPersonRepo_Save_WithMetadata(t *testing.T) {
	repo := NewPersonRepo(setupTestDB(t))
	ctx := context.Background()

	p := &domain.Person{
		Name:        "Meta Person",
		MonitorMode: domain.MonitorAll,
		Metadata:    map[string]any{"hair": "blonde", "rating": 4.5},
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if got.Metadata["hair"] != "blonde" {
		t.Errorf("Metadata hair = %v, want blonde", got.Metadata["hair"])
	}
}

func TestTagRepo_Get(t *testing.T) {
	repo := NewTagRepo(setupTestDB(t))
	ctx := context.Background()

	tag := &domain.Tag{Name: "brunette", Scope: domain.TagScopeMetadata}
	if err := repo.Save(ctx, tag); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Get(ctx, tag.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "brunette" {
		t.Errorf("Name = %q, want brunette", got.Name)
	}
	if got.Scope != domain.TagScopeMetadata {
		t.Errorf("Scope = %q, want metadata", got.Scope)
	}

	if _, err := repo.Get(ctx, "no-such-id"); err == nil {
		t.Error("Get non-existent should error")
	}
}

func TestTagRepo_Category(t *testing.T) {
	repo := NewTagRepo(setupTestDB(t))
	ctx := context.Background()

	genre := &domain.Tag{Name: "Romance", Scope: domain.TagScopeMetadata, Category: domain.TagCategoryGenre}
	warn := &domain.Tag{Name: "Explicit", Scope: domain.TagScopeMetadata, Category: domain.TagCategoryContentWarning}
	general := &domain.Tag{Name: "featured", Scope: domain.TagScopeUser}

	for _, tag := range []*domain.Tag{genre, warn, general} {
		if err := repo.Save(ctx, tag); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// Get round-trips category correctly.
	got, err := repo.Get(ctx, genre.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Category != domain.TagCategoryGenre {
		t.Errorf("Category = %q, want genre", got.Category)
	}

	// List filtered by category returns only matching tags.
	genres, err := repo.List(ctx, ports.TagFilter{Category: domain.TagCategoryGenre})
	if err != nil {
		t.Fatalf("List genres: %v", err)
	}
	if len(genres) != 1 || genres[0].Name != "Romance" {
		t.Errorf("genre tags = %v, want [Romance]", genres)
	}

	// List with no category filter returns all tags.
	all, err := repo.List(ctx, ports.TagFilter{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all tags = %d, want 3", len(all))
	}

	// Zero-value category is preserved (not defaulted to something else).
	got2, err := repo.Get(ctx, general.ID)
	if err != nil {
		t.Fatalf("Get general: %v", err)
	}
	if got2.Category != "" {
		t.Errorf("Category = %q, want empty", got2.Category)
	}
}

func TestLibraryEntryRepo_List_KindAndParentFilters(t *testing.T) {
	repo := NewLibraryEntryRepo(setupTestDB(t))
	ctx := context.Background()

	network := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindNetwork,
		Name: "Network A", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	if err := repo.Save(ctx, network); err != nil {
		t.Fatalf("Save network: %v", err)
	}

	studios := []*domain.LibraryEntry{
		{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio 1", ParentID: network.ID, MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive},
		{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio 2", ParentID: network.ID, MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive},
	}
	for _, s := range studios {
		if err := repo.Save(ctx, s); err != nil {
			t.Fatalf("Save studio: %v", err)
		}
	}

	// Filter by kind
	res, total, err := repo.List(ctx, ports.LibraryFilter{Kind: domain.KindStudio, Limit: 50})
	if err != nil {
		t.Fatalf("List by kind: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("studio total = %d, len = %d, want 2", total, len(res))
	}

	// Filter by parentId
	res, total, err = repo.List(ctx, ports.LibraryFilter{ParentID: network.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by parentId: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("children total = %d, want 2", total)
	}
}

func TestLibraryEntryRepo_Save_WithTagsAndMetadata(t *testing.T) {
	database := setupTestDB(t)
	repo := NewLibraryEntryRepo(database)
	tagRepo := NewTagRepo(database)
	ctx := context.Background()

	tag := &domain.Tag{Name: "featured", Scope: domain.TagScopeUser}
	if err := tagRepo.Save(ctx, tag); err != nil {
		t.Fatalf("Save tag: %v", err)
	}

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name:        "Tagged Studio",
		MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
		Tags:     []domain.Tag{*tag},
		Metadata: map[string]any{"rating": "4K"},
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != "featured" {
		t.Errorf("Tags = %v, want [featured]", got.Tags)
	}
	if got.Metadata == nil || got.Metadata["rating"] != "4K" {
		t.Errorf("Metadata = %v, want {rating:4K}", got.Metadata)
	}
}

func TestItemRepo_Save_WithTagsAndExternalIDsAndDate(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	tagRepo := NewTagRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	tag := &domain.Tag{Name: "4k", Scope: domain.TagScopeMetadata}
	tagRepo.Save(ctx, tag) //nolint:errcheck

	item := &domain.Item{
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: entry.ID,
		Title:          "Tagged Scene",
		Status:         domain.StatusWanted,
		Date:           strToDate("2024-06-01"),
		Metadata:       map[string]any{"explicit": true},
		Tags:           []domain.Tag{*tag},
		ExternalIDs:    []domain.ExternalID{{Source: domain.SourceStashDB, Value: "scene-uuid"}},
	}
	if err := itemRepo.Save(ctx, item); err != nil {
		t.Fatalf("Save item: %v", err)
	}

	got, err := itemRepo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	if got.Date.Format("2006-01-02") != "2024-06-01" {
		t.Errorf("Date = %q, want 2024-06-01", got.Date.Format("2006-01-02"))
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != "4k" {
		t.Errorf("Tags = %v, want [4k]", got.Tags)
	}
	if len(got.ExternalIDs) != 1 || got.ExternalIDs[0].Value != "scene-uuid" {
		t.Errorf("ExternalIDs = %v", got.ExternalIDs)
	}
	if got.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestItemRepo_List_Filters(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	groupRepo := NewGroupRepo(database)
	itemRepo := NewItemRepo(database)
	tagRepo := NewTagRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeTV, Kind: domain.KindSeries,
		Name: "Show", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	group := &domain.Group{
		LibraryEntryID: entry.ID, Title: "Season 1", Number: 1, MonitorMode: domain.MonitorAll,
	}
	groupRepo.Save(ctx, group) //nolint:errcheck

	tag := &domain.Tag{Name: "drama", Scope: domain.TagScopeMetadata}
	tagRepo.Save(ctx, tag) //nolint:errcheck

	items := []*domain.Item{
		{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, GroupID: group.ID, Title: "Pilot", Status: domain.StatusWanted, Monitored: true, Tags: []domain.Tag{*tag}},
		{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, GroupID: group.ID, Title: "Episode 2", Status: domain.StatusImported, Monitored: false},
		{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, Title: "Special", Status: domain.StatusWanted, Monitored: true},
	}
	for _, i := range items {
		if err := itemRepo.Save(ctx, i); err != nil {
			t.Fatalf("Save item: %v", err)
		}
	}

	// Filter by groupId
	res, total, err := itemRepo.List(ctx, ports.ItemFilter{GroupID: group.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by groupId: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("groupId filter total = %d, len = %d, want 2", total, len(res))
	}

	// Filter by status
	_, total, err = itemRepo.List(ctx, ports.ItemFilter{Status: domain.StatusImported, Limit: 50})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if total != 1 {
		t.Errorf("status=imported total = %d, want 1", total)
	}

	// Filter by contentType
	_, total, err = itemRepo.List(ctx, ports.ItemFilter{ContentType: domain.ContentTypeTV, Limit: 50})
	if err != nil {
		t.Fatalf("List by contentType: %v", err)
	}
	if total != 3 {
		t.Errorf("contentType=tv total = %d, want 3", total)
	}

	// Filter by search
	_, total, err = itemRepo.List(ctx, ports.ItemFilter{Search: "Pilot", Limit: 50})
	if err != nil {
		t.Fatalf("List by search: %v", err)
	}
	if total != 1 {
		t.Errorf("search=Pilot total = %d, want 1", total)
	}

	// Filter by tagIds
	_, total, err = itemRepo.List(ctx, ports.ItemFilter{TagIDs: []string{tag.ID}, Limit: 50})
	if err != nil {
		t.Fatalf("List by tagIds: %v", err)
	}
	if total != 1 {
		t.Errorf("tagIds filter total = %d, want 1", total)
	}

	// Filter by monitored
	monTrue := true
	res, total, err = itemRepo.List(ctx, ports.ItemFilter{Monitored: &monTrue, Limit: 50})
	if err != nil {
		t.Fatalf("List monitored: %v", err)
	}
	if total != 2 {
		t.Errorf("monitored total = %d, want 2", total)
	}
	_ = res
}

func TestItemRepo_List_SortByTitle(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	for _, title := range []string{"Zebra Scene", "Apple Scene", "Mango Scene"} {
		i := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: title, Status: domain.StatusWanted}
		if err := itemRepo.Save(ctx, i); err != nil {
			t.Fatalf("Save item %q: %v", title, err)
		}
	}

	res, _, err := itemRepo.List(ctx, ports.ItemFilter{Sort: "title", SortDir: "asc", Limit: 50})
	if err != nil {
		t.Fatalf("List sort=title asc: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 items, got %d", len(res))
	}
	if res[0].Title != "Apple Scene" || res[1].Title != "Mango Scene" || res[2].Title != "Zebra Scene" {
		t.Errorf("unexpected order: %v, %v, %v", res[0].Title, res[1].Title, res[2].Title)
	}
}

func TestItemRepo_List_SortByDate(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	dates := []string{"2023-01-01", "2025-06-01", "2024-03-15"}
	for i, d := range dates {
		item := &domain.Item{
			ContentType:    domain.ContentTypeAdult,
			LibraryEntryID: entry.ID,
			Title:          "Scene",
			Status:         domain.StatusWanted,
			Date:           strToDate(d),
		}
		item.Title = "Scene " + string(rune('A'+i))
		if err := itemRepo.Save(ctx, item); err != nil {
			t.Fatalf("Save item: %v", err)
		}
	}

	// Default (desc) — newest first
	res, _, err := itemRepo.List(ctx, ports.ItemFilter{Sort: "date", SortDir: "desc", Limit: 50})
	if err != nil {
		t.Fatalf("List sort=date desc: %v", err)
	}
	if res[0].Date.Year() != 2025 {
		t.Errorf("first item year = %d, want 2025", res[0].Date.Year())
	}
	if res[2].Date.Year() != 2023 {
		t.Errorf("last item year = %d, want 2023", res[2].Date.Year())
	}

	// Asc — oldest first
	res, _, err = itemRepo.List(ctx, ports.ItemFilter{Sort: "date", SortDir: "asc", Limit: 50})
	if err != nil {
		t.Fatalf("List sort=date asc: %v", err)
	}
	if res[0].Date.Year() != 2023 {
		t.Errorf("first item year (asc) = %d, want 2023", res[0].Date.Year())
	}
}

// ── ExternalIDRepo ────────────────────────────────────────────────────────────

func TestExternalIDRepo_FindEntity(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	extRepo := NewExternalIDRepo(database)
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "StashDB Studio",
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceStashDB, Value: "ext-uuid-abc"}},
	}
	if err := entryRepo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	id, err := extRepo.FindEntity(ctx, "library_entry", string(domain.SourceStashDB), "ext-uuid-abc")
	if err != nil {
		t.Fatalf("FindEntity: %v", err)
	}
	if id != e.ID {
		t.Errorf("FindEntity returned %q, want %q", id, e.ID)
	}
}

func TestExternalIDRepo_FindEntity_NotFound(t *testing.T) {
	extRepo := NewExternalIDRepo(setupTestDB(t))
	ctx := context.Background()

	_, err := extRepo.FindEntity(ctx, "library_entry", "stashdb", "no-such-value")
	if err == nil {
		t.Fatal("expected error for missing external ID, got nil")
	}
}

func TestItemRepo_Delete(t *testing.T) {
	database := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(database)
	itemRepo := NewItemRepo(database)
	ctx := context.Background()

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	item := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID,
		Title: "Scene", Status: domain.StatusWanted,
	}
	itemRepo.Save(ctx, item) //nolint:errcheck

	if err := itemRepo.Delete(ctx, item.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := itemRepo.Get(ctx, item.ID); err == nil {
		t.Error("Get after delete should error")
	}
}

// ── LibraryEntryRepo — entry people ──────────────────────────────────────────

func TestLibraryEntryRepo_SaveAndGetPeople(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(db)
	personRepo := NewPersonRepo(db)
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "The Beatles", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	if err := entryRepo.Save(ctx, artist); err != nil {
		t.Fatalf("Save artist: %v", err)
	}

	john := &domain.Person{Name: "John Lennon", SortName: "Lennon, John", MonitorMode: domain.MonitorAll}
	paul := &domain.Person{Name: "Paul McCartney", SortName: "McCartney, Paul", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, john) //nolint:errcheck
	personRepo.Save(ctx, paul) //nolint:errcheck

	if err := entryRepo.SavePerson(ctx, artist.ID, domain.EntryPerson{PersonID: john.ID, Role: "member"}); err != nil {
		t.Fatalf("SavePerson John: %v", err)
	}
	if err := entryRepo.SavePerson(ctx, artist.ID, domain.EntryPerson{PersonID: paul.ID, Role: "member"}); err != nil {
		t.Fatalf("SavePerson Paul: %v", err)
	}

	people, err := entryRepo.GetPeople(ctx, artist.ID)
	if err != nil {
		t.Fatalf("GetPeople: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("people count = %d, want 2", len(people))
	}
	names := map[string]bool{}
	for _, ep := range people {
		if ep.Person == nil {
			t.Error("Person stub should be loaded")
		} else {
			names[ep.Person.Name] = true
		}
		if ep.Role != "member" {
			t.Errorf("role = %q, want member", ep.Role)
		}
	}
	if !names["John Lennon"] || !names["Paul McCartney"] {
		t.Errorf("expected both members, got names: %v", names)
	}
}

func TestLibraryEntryRepo_GetIncludesPeople(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(db)
	personRepo := NewPersonRepo(db)
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "Pink Floyd", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, artist) //nolint:errcheck

	member := &domain.Person{Name: "Roger Waters", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, member) //nolint:errcheck

	entryRepo.SavePerson(ctx, artist.ID, domain.EntryPerson{PersonID: member.ID, Role: "member"}) //nolint:errcheck

	got, err := entryRepo.Get(ctx, artist.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.People) != 1 {
		t.Fatalf("People count = %d, want 1", len(got.People))
	}
	if got.People[0].Person == nil || got.People[0].Person.Name != "Roger Waters" {
		t.Errorf("People[0].Person = %+v, want Roger Waters stub", got.People[0].Person)
	}
}

func TestLibraryEntryRepo_RemovePerson(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(db)
	personRepo := NewPersonRepo(db)
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, artist) //nolint:errcheck

	p := &domain.Person{Name: "Member One", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, p)                                                                  //nolint:errcheck
	entryRepo.SavePerson(ctx, artist.ID, domain.EntryPerson{PersonID: p.ID, Role: "member"}) //nolint:errcheck

	if err := entryRepo.RemovePerson(ctx, artist.ID, p.ID, "member"); err != nil {
		t.Fatalf("RemovePerson: %v", err)
	}

	people, err := entryRepo.GetPeople(ctx, artist.ID)
	if err != nil {
		t.Fatalf("GetPeople after remove: %v", err)
	}
	if len(people) != 0 {
		t.Errorf("people count = %d, want 0 after remove", len(people))
	}
}

func TestLibraryEntryRepo_List_PersonIDFilter(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := NewLibraryEntryRepo(db)
	personRepo := NewPersonRepo(db)
	ctx := context.Background()

	beatles := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "The Beatles", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	wings := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "Wings", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, beatles) //nolint:errcheck
	entryRepo.Save(ctx, wings)   //nolint:errcheck

	paul := &domain.Person{Name: "Paul McCartney", MonitorMode: domain.MonitorAll}
	john := &domain.Person{Name: "John Lennon", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, paul) //nolint:errcheck
	personRepo.Save(ctx, john) //nolint:errcheck

	entryRepo.SavePerson(ctx, beatles.ID, domain.EntryPerson{PersonID: paul.ID, Role: "member"}) //nolint:errcheck
	entryRepo.SavePerson(ctx, beatles.ID, domain.EntryPerson{PersonID: john.ID, Role: "member"}) //nolint:errcheck
	entryRepo.SavePerson(ctx, wings.ID, domain.EntryPerson{PersonID: paul.ID, Role: "member"})   //nolint:errcheck

	// Paul is in 2 bands
	paulBands, total, err := entryRepo.List(ctx, ports.LibraryFilter{PersonID: paul.ID})
	if err != nil {
		t.Fatalf("List by PersonID: %v", err)
	}
	if total != 2 || len(paulBands) != 2 {
		t.Errorf("Paul bands: total=%d len=%d, want 2", total, len(paulBands))
	}

	// John is in 1 band
	johnBands, total, err := entryRepo.List(ctx, ports.LibraryFilter{PersonID: john.ID})
	if err != nil {
		t.Fatalf("List by PersonID: %v", err)
	}
	if total != 1 || len(johnBands) != 1 {
		t.Errorf("John bands: total=%d len=%d, want 1", total, len(johnBands))
	}
	if johnBands[0].Name != "The Beatles" {
		t.Errorf("band name = %q, want The Beatles", johnBands[0].Name)
	}
}
