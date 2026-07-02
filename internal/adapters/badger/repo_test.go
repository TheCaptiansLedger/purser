package badger_test

import (
	"context"
	"purser/internal/adapters/badger"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
)

func setupTestDB(t *testing.T) *badgerdb.DB {
	t.Helper()
	opts := badgerdb.DefaultOptions(t.TempDir())
	opts = opts.WithLogger(nil)
	db, err := badgerdb.Open(opts)
	if err != nil {
		t.Fatalf("Open badger: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ── LibraryEntryRepo ──────────────────────────────────────────────────────────

func TestBadger_LibraryEntryRepo_SaveAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewLibraryEntryRepo(db)
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

func TestBadger_LibraryEntryRepo_LockedFields(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewLibraryEntryRepo(db)
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType:  domain.ContentTypeAdult,
		Kind:         domain.KindStudio,
		Name:         "Locked Studio",
		MonitorMode:  domain.MonitorAll,
		Status:       domain.EntryStatusActive,
		LockedFields: []string{"name", "overview"},
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.LockedFields) != 2 {
		t.Fatalf("LockedFields len = %d, want 2", len(got.LockedFields))
	}
	if got.LockedFields[0] != "name" || got.LockedFields[1] != "overview" {
		t.Errorf("LockedFields = %v, want [name overview]", got.LockedFields)
	}

	got.LockedFields = nil
	if err := repo.Save(ctx, got); err != nil {
		t.Fatalf("Save cleared: %v", err)
	}
	cleared, err := repo.Get(ctx, e.ID)
	if err != nil {
		t.Fatalf("Get cleared: %v", err)
	}
	if len(cleared.LockedFields) != 0 {
		t.Errorf("LockedFields after clear = %v, want empty", cleared.LockedFields)
	}
}

func TestBadger_LibraryEntryRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewLibraryEntryRepo(db)
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

func TestBadger_LibraryEntryRepo_List_Filters(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewLibraryEntryRepo(db)
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

func TestBadger_LibraryEntryRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewLibraryEntryRepo(db)
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

func TestBadger_LibraryEntryRepo_EntryPeople(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	personRepo := badger.NewPersonRepo(db)
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

	ep := domain.EntryPerson{
		PersonID:  person.ID,
		Role:      "member",
		StartDate: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(1970, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	if err := entryRepo.SavePerson(ctx, entry.ID, ep); err != nil {
		t.Fatalf("SavePerson: %v", err)
	}

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

	// Upsert updates dates for same triple
	ep.StartDate = time.Date(1962, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := entryRepo.SavePerson(ctx, entry.ID, ep); err != nil {
		t.Fatalf("SavePerson upsert: %v", err)
	}
	people, _ = entryRepo.GetPeople(ctx, entry.ID)
	if len(people) != 1 {
		t.Errorf("after upsert, people count = %d, want 1", len(people))
	}

	// RemovePerson
	if err := entryRepo.RemovePerson(ctx, entry.ID, person.ID, "member"); err != nil {
		t.Fatalf("RemovePerson: %v", err)
	}
	people, _ = entryRepo.GetPeople(ctx, entry.ID)
	if len(people) != 0 {
		t.Errorf("after remove, people count = %d, want 0", len(people))
	}
}

func TestBadger_LibraryEntryRepo_List_PersonIDFilter(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	personRepo := badger.NewPersonRepo(db)
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

func TestBadger_PersonRepo_SaveGetWithAliases(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewPersonRepo(db)
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

func TestBadger_PersonRepo_List_BySearch(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewPersonRepo(db)
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

	res, total, err := repo.List(ctx, ports.PersonFilter{Search: "alice", Limit: 50})
	if err != nil {
		t.Fatalf("List search name: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("search alice: total=%d, len=%d, want 1", total, len(res))
	}

	res, total, err = repo.List(ctx, ports.PersonFilter{Search: "bobby", Limit: 50})
	if err != nil {
		t.Fatalf("List search alias: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("search bobby alias: total=%d, len=%d, want 1", total, len(res))
	}
}

func TestBadger_PersonRepo_Roles(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewPersonRepo(db)
	ctx := context.Background()

	p1 := &domain.Person{Name: "Alice", MonitorMode: domain.MonitorAll, Roles: []domain.PersonRole{domain.RolePerformer, domain.RoleDirector}}
	p2 := &domain.Person{Name: "Bob", MonitorMode: domain.MonitorAll, Roles: []domain.PersonRole{domain.RolePerformer}}
	repo.Save(ctx, p1) //nolint:errcheck
	repo.Save(ctx, p2) //nolint:errcheck

	roles, err := repo.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	roleMap := make(map[domain.PersonRole]int)
	for _, r := range roles {
		roleMap[r.Role] = r.Count
	}
	if roleMap[domain.RolePerformer] != 2 {
		t.Errorf("performer count = %d, want 2", roleMap[domain.RolePerformer])
	}
	if roleMap[domain.RoleDirector] != 1 {
		t.Errorf("director count = %d, want 1", roleMap[domain.RoleDirector])
	}
}

func TestBadger_PersonRepo_List_ByRole(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewPersonRepo(db)
	ctx := context.Background()

	p1 := &domain.Person{Name: "Alice", MonitorMode: domain.MonitorAll, Roles: []domain.PersonRole{domain.RolePerformer}}
	p2 := &domain.Person{Name: "Bob", MonitorMode: domain.MonitorAll, Roles: []domain.PersonRole{domain.RoleDirector}}
	repo.Save(ctx, p1) //nolint:errcheck
	repo.Save(ctx, p2) //nolint:errcheck

	res, total, err := repo.List(ctx, ports.PersonFilter{Role: domain.RolePerformer, Limit: 50})
	if err != nil {
		t.Fatalf("List by role: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("role=performer: total=%d, len=%d, want 1", total, len(res))
	}
	if res[0].Name != "Alice" {
		t.Errorf("name = %q, want Alice", res[0].Name)
	}
}

func TestBadger_PersonRepo_List_MultiContentType(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	personRepo := badger.NewPersonRepo(db)
	ctx := context.Background()

	adultEntry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	musicEntry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, adultEntry) //nolint:errcheck
	entryRepo.Save(ctx, musicEntry) //nolint:errcheck

	p1 := &domain.Person{Name: "Alice", MonitorMode: domain.MonitorAll}
	p2 := &domain.Person{Name: "Bob", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, p1) //nolint:errcheck
	personRepo.Save(ctx, p2) //nolint:errcheck

	// p1 on adult item, p2 on music entry
	i1 := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: adultEntry.ID, Title: "Scene", Status: domain.StatusWanted, People: []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}}}
	itemRepo.Save(ctx, i1)                                                                        //nolint:errcheck
	entryRepo.SavePerson(ctx, musicEntry.ID, domain.EntryPerson{PersonID: p2.ID, Role: "member"}) //nolint:errcheck

	resAdult, _, err := personRepo.List(ctx, ports.PersonFilter{ContentTypes: []domain.ContentType{domain.ContentTypeAdult}, Limit: 50})
	if err != nil {
		t.Fatalf("List adult people: %v", err)
	}
	if len(resAdult) != 1 || resAdult[0].Name != "Alice" {
		t.Errorf("adult people = %v, want [Alice]", resAdult)
	}

	resMusic, _, err := personRepo.List(ctx, ports.PersonFilter{ContentTypes: []domain.ContentType{domain.ContentTypeMusic}, Limit: 50})
	if err != nil {
		t.Fatalf("List music people: %v", err)
	}
	if len(resMusic) != 1 || resMusic[0].Name != "Bob" {
		t.Errorf("music people = %v, want [Bob]", resMusic)
	}
}

// ── ItemRepo ──────────────────────────────────────────────────────────────────

func TestBadger_ItemRepo_SaveAndGet(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	personRepo := badger.NewPersonRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	person := &domain.Person{Name: "Jane Doe", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, person) //nolint:errcheck

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
	// Person stub should be populated
	if got.People[0].Person == nil || got.People[0].Person.Name != "Jane Doe" {
		t.Errorf("Person stub name = %v, want Jane Doe", got.People[0].Person)
	}
}

func TestBadger_ItemRepo_List_ByPersonID(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	personRepo := badger.NewPersonRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	p1 := &domain.Person{Name: "Alice", MonitorMode: domain.MonitorAll}
	p2 := &domain.Person{Name: "Bob", MonitorMode: domain.MonitorAll}
	personRepo.Save(ctx, p1) //nolint:errcheck
	personRepo.Save(ctx, p2) //nolint:errcheck

	i1 := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene 1", Status: domain.StatusWanted, People: []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}}}
	i2 := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene 2", Status: domain.StatusWanted, People: []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}, {PersonID: p2.ID, Role: domain.RolePerformer}}}
	itemRepo.Save(ctx, i1) //nolint:errcheck
	itemRepo.Save(ctx, i2) //nolint:errcheck

	res, total, err := itemRepo.List(ctx, ports.ItemFilter{PersonID: p1.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by p1: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Errorf("p1 item total = %d, len = %d, want 2", total, len(res))
	}

	res, total, err = itemRepo.List(ctx, ports.ItemFilter{PersonID: p2.ID, Limit: 50})
	if err != nil {
		t.Fatalf("List by p2: %v", err)
	}
	if total != 1 || len(res) != 1 {
		t.Errorf("p2 item total = %d, want 1", total)
	}
}

func TestBadger_ItemRepo_List_SortByTitle(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	titles := []string{"Bravo", "Alpha", "Charlie"}
	for _, title := range titles {
		itemRepo.Save(ctx, &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: title, Status: domain.StatusWanted}) //nolint:errcheck
	}

	asc, _, _ := itemRepo.List(ctx, ports.ItemFilter{Sort: "title", SortDir: "ASC", Limit: 50})
	if len(asc) != 3 || asc[0].Title != "Alpha" || asc[2].Title != "Charlie" {
		t.Errorf("sort asc = %v", titlesOf(asc))
	}

	desc, _, _ := itemRepo.List(ctx, ports.ItemFilter{Sort: "title", SortDir: "DESC", Limit: 50})
	if len(desc) != 3 || desc[0].Title != "Charlie" || desc[2].Title != "Alpha" {
		t.Errorf("sort desc = %v", titlesOf(desc))
	}
}

func titlesOf(items []*domain.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func TestBadger_ItemRepo_List_SortByDate(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	dates := []time.Time{
		time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	for i, d := range dates {
		itemRepo.Save(ctx, &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene", Date: d, Status: domain.StatusWanted, Sequence: string(rune('A' + i))}) //nolint:errcheck
	}

	// Default (desc by date) — most recent first.
	desc, _, _ := itemRepo.List(ctx, ports.ItemFilter{Limit: 50})
	if len(desc) != 3 || !desc[0].Date.Equal(dates[1]) {
		t.Errorf("default sort (desc date): first item date = %v, want %v", desc[0].Date, dates[1])
	}

	asc, _, _ := itemRepo.List(ctx, ports.ItemFilter{Sort: "date", SortDir: "ASC", Limit: 50})
	if len(asc) != 3 || !asc[0].Date.Equal(dates[0]) {
		t.Errorf("asc date: first item date = %v, want %v", asc[0].Date, dates[0])
	}
}

// ── TagRepo ───────────────────────────────────────────────────────────────────

func TestBadger_TagRepo_SaveListDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewTagRepo(db)
	ctx := context.Background()

	t1 := &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"}
	t2 := &domain.Tag{Key: "genre", Value: "Rock", Scope: "music"}
	repo.Save(ctx, t1) //nolint:errcheck
	repo.Save(ctx, t2) //nolint:errcheck

	if t1.ID == "" || t2.ID == "" {
		t.Fatal("ID should be set by Save")
	}

	all, err := repo.List(ctx, ports.TagFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List count = %d, want 2", len(all))
	}

	got, err := repo.Get(ctx, t1.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != "Jazz" {
		t.Errorf("Value = %q, want Jazz", got.Value)
	}

	if err := repo.Delete(ctx, t1.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	remaining, _ := repo.List(ctx, ports.TagFilter{})
	if len(remaining) != 1 {
		t.Errorf("after delete, count = %d, want 1", len(remaining))
	}
}

func TestBadger_GroupRepo_Tags(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	group := &domain.Group{LibraryEntryID: entry.ID, Title: "Album", Number: 1}
	groupRepo.Save(ctx, group) //nolint:errcheck

	tag := &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"}
	tagRepo.Save(ctx, tag) //nolint:errcheck

	if err := tagRepo.AddGroupTag(ctx, group.ID, tag.ID); err != nil {
		t.Fatalf("AddGroupTag: %v", err)
	}

	// Group.Get should include the tag.
	got, err := groupRepo.Get(ctx, group.ID)
	if err != nil {
		t.Fatalf("Get group: %v", err)
	}
	if len(got.Tags) != 1 || got.Tags[0].Value != "Jazz" {
		t.Errorf("group tags = %v, want [Jazz]", got.Tags)
	}

	if err := tagRepo.RemoveGroupTag(ctx, group.ID, tag.ID); err != nil {
		t.Fatalf("RemoveGroupTag: %v", err)
	}
	got, _ = groupRepo.Get(ctx, group.ID)
	if len(got.Tags) != 0 {
		t.Errorf("after remove, tags = %v, want empty", got.Tags)
	}
}

func TestBadger_TagRepo_List_FilterByGroupID(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	g1 := &domain.Group{LibraryEntryID: entry.ID, Title: "Album 1", Number: 1}
	g2 := &domain.Group{LibraryEntryID: entry.ID, Title: "Album 2", Number: 2}
	groupRepo.Save(ctx, g1) //nolint:errcheck
	groupRepo.Save(ctx, g2) //nolint:errcheck

	tagJazz := &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"}
	tagRock := &domain.Tag{Key: "genre", Value: "Rock", Scope: "music"}
	tagRepo.Save(ctx, tagJazz) //nolint:errcheck
	tagRepo.Save(ctx, tagRock) //nolint:errcheck

	tagRepo.AddGroupTag(ctx, g1.ID, tagJazz.ID) //nolint:errcheck
	tagRepo.AddGroupTag(ctx, g2.ID, tagRock.ID) //nolint:errcheck

	res, err := tagRepo.List(ctx, ports.TagFilter{GroupID: g1.ID})
	if err != nil {
		t.Fatalf("List by group: %v", err)
	}
	if len(res) != 1 || res[0].Value != "Jazz" {
		t.Errorf("group1 tags = %v, want [Jazz]", res)
	}
}

func TestBadger_TagRepo_List_FilterByContentType_IncludesGroupTags(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	musicEntry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	adultEntry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, musicEntry) //nolint:errcheck
	entryRepo.Save(ctx, adultEntry) //nolint:errcheck

	album := &domain.Group{LibraryEntryID: musicEntry.ID, Title: "Album", Number: 1}
	groupRepo.Save(ctx, album) //nolint:errcheck

	tagJazz := &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"}
	tagAdult := &domain.Tag{Key: "genre", Value: "Gonzo", Scope: "adult"}
	tagRepo.Save(ctx, tagJazz)  //nolint:errcheck
	tagRepo.Save(ctx, tagAdult) //nolint:errcheck

	// Jazz tag on music group
	tagRepo.AddGroupTag(ctx, album.ID, tagJazz.ID) //nolint:errcheck
	// Gonzo tag on adult entry
	adultEntry.Tags = []domain.Tag{*tagAdult}
	entryRepo.Save(ctx, adultEntry) //nolint:errcheck

	res, err := tagRepo.List(ctx, ports.TagFilter{ContentTypes: []domain.ContentType{domain.ContentTypeMusic}})
	if err != nil {
		t.Fatalf("List music tags: %v", err)
	}
	if len(res) != 1 || res[0].Value != "Jazz" {
		t.Errorf("music tags = %v, want [Jazz]", res)
	}

	res, err = tagRepo.List(ctx, ports.TagFilter{ContentTypes: []domain.ContentType{domain.ContentTypeAdult}})
	if err != nil {
		t.Fatalf("List adult tags: %v", err)
	}
	if len(res) != 1 || res[0].Value != "Gonzo" {
		t.Errorf("adult tags = %v, want [Gonzo]", res)
	}
}

func TestBadger_TagRepo_List_FilterByValue(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewTagRepo(db)
	ctx := context.Background()

	repo.Save(ctx, &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"})  //nolint:errcheck
	repo.Save(ctx, &domain.Tag{Key: "genre", Value: "Rock", Scope: "music"})  //nolint:errcheck
	repo.Save(ctx, &domain.Tag{Key: "genre", Value: "Blues", Scope: "music"}) //nolint:errcheck

	res, err := repo.List(ctx, ports.TagFilter{Value: "Jazz"})
	if err != nil {
		t.Fatalf("List by value: %v", err)
	}
	if len(res) != 1 || res[0].Value != "Jazz" {
		t.Errorf("value filter = %v, want [Jazz]", res)
	}
}

// ── GroupRepo ─────────────────────────────────────────────────────────────────

func TestBadger_GroupRepo_SaveGetListDelete(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Band", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	g := &domain.Group{LibraryEntryID: entry.ID, Title: "Album", Number: 1, Year: 2020}
	if err := groupRepo.Save(ctx, g); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if g.ID == "" {
		t.Fatal("ID should be set by Save")
	}

	got, err := groupRepo.Get(ctx, g.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Album" || got.Year != 2020 {
		t.Errorf("got title=%q year=%d, want Album/2020", got.Title, got.Year)
	}

	all, err := groupRepo.List(ctx, ports.GroupFilter{LibraryEntryID: entry.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("list count = %d, want 1", len(all))
	}

	if err := groupRepo.Delete(ctx, g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := groupRepo.Get(ctx, g.ID); err == nil {
		t.Error("Get after delete should error")
	}
}

// ── ExternalIDRepo ────────────────────────────────────────────────────────────

func TestBadger_ExternalIDRepo_FindEntity(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	eidRepo := badger.NewExternalIDRepo(db)
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio",
		MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
		ExternalIDs: []domain.ExternalID{{Source: domain.SourceStashDB, Value: "studio-uuid"}},
	}
	entryRepo.Save(ctx, e) //nolint:errcheck

	id, err := eidRepo.FindEntity(ctx, "library_entry", string(domain.SourceStashDB), "studio-uuid")
	if err != nil {
		t.Fatalf("FindEntity: %v", err)
	}
	if id != e.ID {
		t.Errorf("FindEntity id = %q, want %q", id, e.ID)
	}
}

func TestBadger_ExternalIDRepo_FindEntity_NotFound(t *testing.T) {
	db := setupTestDB(t)
	eidRepo := badger.NewExternalIDRepo(db)
	ctx := context.Background()

	_, err := eidRepo.FindEntity(ctx, "library_entry", "stashdb", "does-not-exist")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ── MediaFileRepo ─────────────────────────────────────────────────────────────

func TestBadger_MediaFileRepo_SaveAndGet(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	mfRepo := badger.NewMediaFileRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "S", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	item := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Scene", Status: domain.StatusWanted}
	itemRepo.Save(ctx, item) //nolint:errcheck

	mf := &domain.MediaFile{
		ItemID: item.ID,
		Path:   "/media/scene.mp4",
		Size:   1024,
		OSHash: "aabbcc112233",
	}
	if err := mfRepo.Save(ctx, mf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if mf.ID == "" {
		t.Fatal("ID should be set by Save")
	}

	byItem, err := mfRepo.GetByItemID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetByItemID: %v", err)
	}
	if byItem.Path != "/media/scene.mp4" {
		t.Errorf("Path = %q, want /media/scene.mp4", byItem.Path)
	}

	byHash, err := mfRepo.GetByOSHash(ctx, "aabbcc112233")
	if err != nil {
		t.Fatalf("GetByOSHash: %v", err)
	}
	if byHash.ID != mf.ID {
		t.Errorf("byHash ID = %q, want %q", byHash.ID, mf.ID)
	}

	if err := mfRepo.Delete(ctx, mf.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := mfRepo.GetByItemID(ctx, item.ID); !errs.IsNotFound(err) {
		t.Errorf("GetByItemID after delete: expected ErrNotFound, got %v", err)
	}
}

// ── SettingsRepo ──────────────────────────────────────────────────────────────

func TestBadger_SettingsRepo_GetMissingKey(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewSettingsRepo(db)
	ctx := context.Background()

	_, err := repo.Get(ctx, "missing-key")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBadger_SettingsRepo_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewSettingsRepo(db)
	ctx := context.Background()

	if err := repo.Set(ctx, "theme", "dark"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := repo.Get(ctx, "theme")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "dark" {
		t.Errorf("value = %q, want dark", val)
	}
}

func TestBadger_SettingsRepo_Set_Upsert(t *testing.T) {
	db := setupTestDB(t)
	repo := badger.NewSettingsRepo(db)
	ctx := context.Background()

	repo.Set(ctx, "key", "v1") //nolint:errcheck
	repo.Set(ctx, "key", "v2") //nolint:errcheck

	val, _ := repo.Get(ctx, "key")
	if val != "v2" {
		t.Errorf("upsert value = %q, want v2", val)
	}
}

// ── Tag-filter regression tests ───────────────────────────────────────────────
//
// These tests guard against the bug where TagKey/TagValue filters were silently
// ignored by all three Badger repos, causing tag browse pages to return every
// record in the database regardless of tag assignment.

func TestBadger_ItemRepo_List_TagKeyValueFilter(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	itemRepo := badger.NewItemRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, entry) //nolint:errcheck

	tagYoga := &domain.Tag{Key: "adult", Value: "Yoga", Scope: "metadata"}
	tagFitness := &domain.Tag{Key: "adult", Value: "Fitness", Scope: "metadata"}
	tagRepo.Save(ctx, tagYoga)    //nolint:errcheck
	tagRepo.Save(ctx, tagFitness) //nolint:errcheck

	// itemWithYoga has the Yoga tag.
	itemWithYoga := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID,
		Title: "Yoga Scene", Status: domain.StatusWanted,
		Tags: []domain.Tag{*tagYoga},
	}
	// itemWithFitness has a different tag — must NOT appear in Yoga results.
	itemWithFitness := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID,
		Title: "Fitness Scene", Status: domain.StatusWanted,
		Tags: []domain.Tag{*tagFitness},
	}
	// itemNoTags has no tags — must NOT appear in any tag-filtered result.
	itemNoTags := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID,
		Title: "Untagged Scene", Status: domain.StatusWanted,
	}
	itemRepo.Save(ctx, itemWithYoga)    //nolint:errcheck
	itemRepo.Save(ctx, itemWithFitness) //nolint:errcheck
	itemRepo.Save(ctx, itemNoTags)      //nolint:errcheck

	tests := []struct {
		name       string
		filter     ports.ItemFilter
		wantTitles []string
	}{
		{
			name:       "key+value matches only Yoga item",
			filter:     ports.ItemFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50},
			wantTitles: []string{"Yoga Scene"},
		},
		{
			name:       "key only matches items with that key regardless of value",
			filter:     ports.ItemFilter{TagKey: "adult", Limit: 50},
			wantTitles: []string{"Yoga Scene", "Fitness Scene"},
		},
		{
			name:       "non-existent value returns empty",
			filter:     ports.ItemFilter{TagKey: "adult", TagValue: "Pilates", Limit: 50},
			wantTitles: []string{},
		},
		{
			name:       "untagged item is excluded by any tag filter",
			filter:     ports.ItemFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50},
			wantTitles: []string{"Yoga Scene"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := itemRepo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			got := titlesOf(res)
			if len(got) != len(tc.wantTitles) {
				t.Fatalf("got %d results %v, want %d %v", len(got), got, len(tc.wantTitles), tc.wantTitles)
			}
			wantSet := make(map[string]struct{}, len(tc.wantTitles))
			for _, w := range tc.wantTitles {
				wantSet[w] = struct{}{}
			}
			for _, g := range got {
				if _, ok := wantSet[g]; !ok {
					t.Errorf("unexpected result %q", g)
				}
			}
		})
	}
}

func TestBadger_LibraryEntryRepo_List_TagKeyValueFilter(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	tagYoga := &domain.Tag{Key: "adult", Value: "Yoga", Scope: "metadata"}
	tagFitness := &domain.Tag{Key: "adult", Value: "Fitness", Scope: "metadata"}
	tagRepo.Save(ctx, tagYoga)    //nolint:errcheck
	tagRepo.Save(ctx, tagFitness) //nolint:errcheck

	entryWithYoga := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Yoga Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
		Tags: []domain.Tag{*tagYoga},
	}
	entryWithFitness := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Fitness Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
		Tags: []domain.Tag{*tagFitness},
	}
	// entryNoTags must NOT appear in any tag-filtered result.
	entryNoTags := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "Dana Fuchs", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, entryWithYoga)    //nolint:errcheck
	entryRepo.Save(ctx, entryWithFitness) //nolint:errcheck
	entryRepo.Save(ctx, entryNoTags)      //nolint:errcheck

	tests := []struct {
		name      string
		filter    ports.LibraryFilter
		wantNames []string
	}{
		{
			name:      "key+value returns only matching entry",
			filter:    ports.LibraryFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50},
			wantNames: []string{"Yoga Studio"},
		},
		{
			name:      "key only returns all entries with that key",
			filter:    ports.LibraryFilter{TagKey: "adult", Limit: 50},
			wantNames: []string{"Yoga Studio", "Fitness Studio"},
		},
		{
			name:      "non-existent value returns empty",
			filter:    ports.LibraryFilter{TagKey: "adult", TagValue: "Pilates", Limit: 50},
			wantNames: []string{},
		},
		{
			name:      "untagged entry (Dana Fuchs) is excluded",
			filter:    ports.LibraryFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50},
			wantNames: []string{"Yoga Studio"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := entryRepo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(res) != len(tc.wantNames) {
				names := make([]string, len(res))
				for i, e := range res {
					names[i] = e.Name
				}
				t.Fatalf("got %d results %v, want %d %v", len(res), names, len(tc.wantNames), tc.wantNames)
			}
			wantSet := make(map[string]struct{}, len(tc.wantNames))
			for _, w := range tc.wantNames {
				wantSet[w] = struct{}{}
			}
			for _, e := range res {
				if _, ok := wantSet[e.Name]; !ok {
					t.Errorf("unexpected entry %q", e.Name)
				}
			}
		})
	}
}

func TestBadger_GroupRepo_List_TagKeyValueFilter(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	musicEntry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Artist", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
	entryRepo.Save(ctx, musicEntry) //nolint:errcheck

	tagJazz := &domain.Tag{Key: "genre", Value: "Jazz", Scope: "music"}
	tagRock := &domain.Tag{Key: "genre", Value: "Rock", Scope: "music"}
	tagRepo.Save(ctx, tagJazz) //nolint:errcheck
	tagRepo.Save(ctx, tagRock) //nolint:errcheck

	albumJazz := &domain.Group{LibraryEntryID: musicEntry.ID, Title: "Jazz Album", Number: 1}
	albumRock := &domain.Group{LibraryEntryID: musicEntry.ID, Title: "Rock Album", Number: 2}
	albumNone := &domain.Group{LibraryEntryID: musicEntry.ID, Title: "Untagged Album", Number: 3}
	groupRepo.Save(ctx, albumJazz) //nolint:errcheck
	groupRepo.Save(ctx, albumRock) //nolint:errcheck
	groupRepo.Save(ctx, albumNone) //nolint:errcheck

	tagRepo.AddGroupTag(ctx, albumJazz.ID, tagJazz.ID) //nolint:errcheck
	tagRepo.AddGroupTag(ctx, albumRock.ID, tagRock.ID) //nolint:errcheck

	tests := []struct {
		name       string
		filter     ports.GroupFilter
		wantTitles []string
	}{
		{
			name:       "key+value returns only Jazz album",
			filter:     ports.GroupFilter{TagKey: "genre", TagValue: "Jazz"},
			wantTitles: []string{"Jazz Album"},
		},
		{
			name:       "key only returns all genre-tagged albums",
			filter:     ports.GroupFilter{TagKey: "genre"},
			wantTitles: []string{"Jazz Album", "Rock Album"},
		},
		{
			name:       "non-existent value returns empty",
			filter:     ports.GroupFilter{TagKey: "genre", TagValue: "Blues"},
			wantTitles: []string{},
		},
		{
			name:       "untagged album is excluded",
			filter:     ports.GroupFilter{TagKey: "genre", TagValue: "Jazz"},
			wantTitles: []string{"Jazz Album"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := groupRepo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(res) != len(tc.wantTitles) {
				got := make([]string, len(res))
				for i, g := range res {
					got[i] = g.Title
				}
				t.Fatalf("got %d results %v, want %d %v", len(res), got, len(tc.wantTitles), tc.wantTitles)
			}
			wantSet := make(map[string]struct{}, len(tc.wantTitles))
			for _, w := range tc.wantTitles {
				wantSet[w] = struct{}{}
			}
			for _, g := range res {
				if _, ok := wantSet[g.Title]; !ok {
					t.Errorf("unexpected group %q", g.Title)
				}
			}
		})
	}
}

// TestBadger_TagBrowse_UntaggedEntitiesNeverLeakThrough is the direct regression
// test for the bug where `/tags/adult/Yoga` returned Dana Fuchs and her tracks
// despite having no tags — tag filters were silently ignored by all three repos.
func TestBadger_TagBrowse_UntaggedEntitiesNeverLeakThrough(t *testing.T) {
	db := setupTestDB(t)
	entryRepo := badger.NewLibraryEntryRepo(db)
	groupRepo := badger.NewGroupRepo(db)
	itemRepo := badger.NewItemRepo(db)
	tagRepo := badger.NewTagRepo(db)
	ctx := context.Background()

	// Dana Fuchs — music artist, no tags whatsoever.
	dana := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "Dana Fuchs", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	entryRepo.Save(ctx, dana) //nolint:errcheck

	danaAlbum := &domain.Group{LibraryEntryID: dana.ID, Title: "Love Lives On", Number: 1}
	groupRepo.Save(ctx, danaAlbum) //nolint:errcheck

	danaTrack := &domain.Item{
		ContentType: domain.ContentTypeMusic, LibraryEntryID: dana.ID, GroupID: danaAlbum.ID,
		Title: "Naked In the Morning", Status: domain.StatusWanted,
	}
	itemRepo.Save(ctx, danaTrack) //nolint:errcheck

	// The Yoga tag only lives on an adult entry — not on any music entity.
	yogaTag := &domain.Tag{Key: "adult", Value: "Yoga", Scope: "metadata"}
	tagRepo.Save(ctx, yogaTag) //nolint:errcheck

	yogaEntry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Yoga Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
		Tags: []domain.Tag{*yogaTag},
	}
	entryRepo.Save(ctx, yogaEntry) //nolint:errcheck

	// Filter by adult/Yoga — music entities must be completely absent.
	entries, _, err := entryRepo.List(ctx, ports.LibraryFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50})
	if err != nil {
		t.Fatalf("entries List: %v", err)
	}
	for _, e := range entries {
		if e.Name == "Dana Fuchs" {
			t.Error("Dana Fuchs (untagged music entry) leaked through the adult/Yoga entry filter")
		}
	}
	if len(entries) != 1 || entries[0].Name != "Yoga Studio" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		t.Errorf("entries = %v, want [Yoga Studio]", names)
	}

	groups, err := groupRepo.List(ctx, ports.GroupFilter{TagKey: "adult", TagValue: "Yoga"})
	if err != nil {
		t.Fatalf("groups List: %v", err)
	}
	for _, g := range groups {
		if g.Title == "Love Lives On" {
			t.Error("Dana Fuchs album (untagged) leaked through the adult/Yoga group filter")
		}
	}
	if len(groups) != 0 {
		t.Errorf("groups = %d, want 0 (no groups have the Yoga tag)", len(groups))
	}

	items, _, err := itemRepo.List(ctx, ports.ItemFilter{TagKey: "adult", TagValue: "Yoga", Limit: 50})
	if err != nil {
		t.Fatalf("items List: %v", err)
	}
	for _, i := range items {
		if i.Title == "Naked In the Morning" {
			t.Error("Dana Fuchs track (untagged) leaked through the adult/Yoga item filter")
		}
	}
	if len(items) != 0 {
		t.Errorf("items = %d, want 0 (no items have the Yoga tag)", len(items))
	}
}
