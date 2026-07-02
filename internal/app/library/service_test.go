package library_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/app/library"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── Hand-rolled mocks ─────────────────────────────────────────────────────────

type mockEntryRepo struct {
	data          map[string]*domain.LibraryEntry
	people        map[string][]domain.EntryPerson // keyed by entryID
	saveErr       error
	savePersonErr error
}

func newMockEntryRepo() *mockEntryRepo {
	return &mockEntryRepo{
		data:   make(map[string]*domain.LibraryEntry),
		people: make(map[string][]domain.EntryPerson),
	}
}

func (m *mockEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	if m.saveErr != nil {
		return nil, m.saveErr
	}
	e, ok := m.data[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return e, nil
}

func (m *mockEntryRepo) List(_ context.Context, f ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	var res []*domain.LibraryEntry
	for _, e := range m.data {
		if f.ParentID != "" && e.ParentID != f.ParentID {
			continue
		}
		res = append(res, e)
	}
	return res, len(res), nil
}

func (m *mockEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if e.ID == "" {
		e.ID = fmt.Sprintf("entry-%d", len(m.data)+1)
	}
	m.data[e.ID] = e
	return nil
}

func (m *mockEntryRepo) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func (m *mockEntryRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeDestroy}, nil
}

func (m *mockEntryRepo) GetPeople(_ context.Context, entryID string) ([]domain.EntryPerson, error) {
	return m.people[entryID], nil
}

func (m *mockEntryRepo) SavePerson(_ context.Context, entryID string, ep domain.EntryPerson) error {
	if m.savePersonErr != nil {
		return m.savePersonErr
	}
	m.people[entryID] = append(m.people[entryID], ep)
	return nil
}

func (m *mockEntryRepo) RemovePerson(_ context.Context, entryID, personID, role string) error {
	existing := m.people[entryID]
	updated := existing[:0]
	for _, ep := range existing {
		if ep.PersonID != personID || ep.Role != role {
			updated = append(updated, ep)
		}
	}
	m.people[entryID] = updated
	return nil
}

type mockPersonRepo struct {
	data    map[string]*domain.Person
	saveErr error
}

func newMockPersonRepo() *mockPersonRepo {
	return &mockPersonRepo{data: make(map[string]*domain.Person)}
}

func (m *mockPersonRepo) Get(_ context.Context, id string) (*domain.Person, error) {
	p, ok := m.data[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return p, nil
}

func (m *mockPersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	res := make([]*domain.Person, 0, len(m.data))
	for _, p := range m.data {
		res = append(res, p)
	}
	return res, len(res), nil
}

func (m *mockPersonRepo) Save(_ context.Context, p *domain.Person) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if p.ID == "" {
		p.ID = fmt.Sprintf("person-%d", len(m.data)+1)
	}
	m.data[p.ID] = p
	return nil
}

func (m *mockPersonRepo) ListRoles(_ context.Context) ([]domain.PersonRoleCount, error) {
	return nil, nil
}

func (m *mockPersonRepo) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func (m *mockPersonRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeUnlink}, nil
}

type mockGroupRepo struct {
	data map[string]*domain.Group
}

func newMockGroupRepo() *mockGroupRepo {
	return &mockGroupRepo{data: make(map[string]*domain.Group)}
}

func (m *mockGroupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	g, ok := m.data[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return g, nil
}

func (m *mockGroupRepo) List(_ context.Context, _ ports.GroupFilter) ([]*domain.Group, error) {
	res := make([]*domain.Group, 0, len(m.data))
	for _, g := range m.data {
		res = append(res, g)
	}
	return res, nil
}

func (m *mockGroupRepo) Save(_ context.Context, g *domain.Group) error {
	if g.ID == "" {
		g.ID = fmt.Sprintf("group-%d", len(m.data)+1)
	}
	m.data[g.ID] = g
	return nil
}

func (m *mockGroupRepo) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func (m *mockGroupRepo) DeleteByLibraryEntry(_ context.Context, entryID string) error {
	for id, g := range m.data {
		if g.LibraryEntryID == entryID {
			delete(m.data, id)
		}
	}
	return nil
}

func (m *mockGroupRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeDestroy}, nil
}

type mockItemRepo struct {
	data    map[string]*domain.Item
	saveErr error
}

func newMockItemRepo() *mockItemRepo {
	return &mockItemRepo{data: make(map[string]*domain.Item)}
}

func (m *mockItemRepo) Get(_ context.Context, id string) (*domain.Item, error) {
	item, ok := m.data[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return item, nil
}

func (m *mockItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	res := make([]*domain.Item, 0, len(m.data))
	for _, item := range m.data {
		res = append(res, item)
	}
	return res, len(res), nil
}

func (m *mockItemRepo) Save(_ context.Context, item *domain.Item) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if item.ID == "" {
		item.ID = fmt.Sprintf("item-%d", len(m.data)+1)
	}
	m.data[item.ID] = item
	return nil
}

func (m *mockItemRepo) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func (m *mockItemRepo) DeleteByGroup(_ context.Context, groupID string) error {
	for id, item := range m.data {
		if item.GroupID == groupID {
			delete(m.data, id)
		}
	}
	return nil
}

func (m *mockItemRepo) DeleteByLibraryEntry(_ context.Context, entryID string) error {
	for id, item := range m.data {
		if item.LibraryEntryID == entryID {
			delete(m.data, id)
		}
	}
	return nil
}

func (m *mockItemRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{Mode: domain.DeletionModeDestroy}, nil
}

func newSvc(entries *mockEntryRepo, groups *mockGroupRepo, items *mockItemRepo) *library.Service {
	return library.New(entries, groups, items, newMockPersonRepo(), nil)
}

func newSvcWithPersons(entries *mockEntryRepo, groups *mockGroupRepo, items *mockItemRepo, persons *mockPersonRepo) *library.Service {
	return library.New(entries, groups, items, persons, nil)
}

// ── Entry tests ───────────────────────────────────────────────────────────────

func TestCreateEntry_Valid(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Evil Angel",
	}
	if err := svc.CreateEntry(context.Background(), e); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if e.ID == "" {
		t.Error("ID should be populated after create")
	}
	if e.SortName != "Evil Angel" {
		t.Errorf("SortName = %q, want same as Name", e.SortName)
	}
	if e.MonitorMode != domain.MonitorAll {
		t.Errorf("MonitorMode = %q, want all", e.MonitorMode)
	}
	if e.Status != domain.EntryStatusActive {
		t.Errorf("Status = %q, want active", e.Status)
	}
	if len(entries.data) != 1 {
		t.Errorf("repo has %d entries, want 1", len(entries.data))
	}
}

func TestCreateEntry_EmptyName(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.CreateEntry(context.Background(), &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !errs.IsValidation(err) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestCreateEntry_InvalidContentType(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.CreateEntry(context.Background(), &domain.LibraryEntry{
		ContentType: "invalid",
		Kind:        domain.KindStudio,
		Name:        "Test",
	})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for invalid content type, got %v", err)
	}
}

func TestCreateEntry_InvalidKind(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.CreateEntry(context.Background(), &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        "invalid",
		Name:        "Test",
	})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for invalid kind, got %v", err)
	}
}

func TestCreateEntry_MovieAutoCreatesItem(t *testing.T) {
	entries := newMockEntryRepo()
	items := newMockItemRepo()
	svc := newSvc(entries, newMockGroupRepo(), items)

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMovie,
		Kind:        domain.KindMovie,
		Name:        "Inception",
		Monitored:   true,
	}
	if err := svc.CreateEntry(context.Background(), e); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if len(items.data) != 1 {
		t.Fatalf("expected 1 auto-created item, got %d", len(items.data))
	}
	for _, item := range items.data {
		if item.LibraryEntryID != e.ID {
			t.Errorf("item.LibraryEntryID = %q, want %q", item.LibraryEntryID, e.ID)
		}
		if item.Title != "Inception" {
			t.Errorf("item.Title = %q, want Inception", item.Title)
		}
		if item.Status != domain.StatusWanted {
			t.Errorf("item.Status = %q, want wanted", item.Status)
		}
		if item.ContentType != domain.ContentTypeMovie {
			t.Errorf("item.ContentType = %q, want movie", item.ContentType)
		}
	}
}

func TestCreateEntry_NonMovie_NoAutoItem(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)

	if err := svc.CreateEntry(context.Background(), &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Test Studio",
	}); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if len(items.data) != 0 {
		t.Errorf("expected no auto-created items for studio, got %d", len(items.data))
	}
}

func TestGetEntry_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	_, err := svc.GetEntry(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteEntry_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.DeleteEntry(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteEntry_CascadesToGroupsAndItems(t *testing.T) {
	entries := newMockEntryRepo()
	groups := newMockGroupRepo()
	items := newMockItemRepo()
	svc := newSvc(entries, groups, items)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Radiohead"}
	if err := svc.CreateEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	album := &domain.Group{LibraryEntryID: entry.ID, Title: "OK Computer"}
	if err := svc.CreateGroup(ctx, album); err != nil {
		t.Fatal(err)
	}
	track := &domain.Item{LibraryEntryID: entry.ID, GroupID: album.ID, Title: "Karma Police"}
	if err := svc.CreateItem(ctx, track); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetEntry(ctx, entry.ID); !errs.IsNotFound(err) {
		t.Error("entry should be deleted")
	}
	if _, err := svc.GetGroup(ctx, album.ID); !errs.IsNotFound(err) {
		t.Error("group should be deleted when entry is deleted")
	}
	if _, err := svc.GetItem(ctx, track.ID); !errs.IsNotFound(err) {
		t.Error("item should be deleted when entry is deleted")
	}
}

func TestDeleteGroup_CascadesToItems(t *testing.T) {
	entries := newMockEntryRepo()
	groups := newMockGroupRepo()
	items := newMockItemRepo()
	svc := newSvc(entries, groups, items)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Radiohead"}
	if err := svc.CreateEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	album := &domain.Group{LibraryEntryID: entry.ID, Title: "OK Computer"}
	if err := svc.CreateGroup(ctx, album); err != nil {
		t.Fatal(err)
	}
	track := &domain.Item{LibraryEntryID: entry.ID, GroupID: album.ID, Title: "Karma Police"}
	if err := svc.CreateItem(ctx, track); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteGroup(ctx, album.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetGroup(ctx, album.ID); !errs.IsNotFound(err) {
		t.Error("group should be deleted")
	}
	if _, err := svc.GetItem(ctx, track.ID); !errs.IsNotFound(err) {
		t.Error("item should be deleted when its group is deleted")
	}
	// entry itself must survive
	if _, err := svc.GetEntry(ctx, entry.ID); err != nil {
		t.Errorf("entry should survive group deletion: %v", err)
	}
}

func TestListEntries(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())

	for _, name := range []string{"A", "B", "C"} {
		svc.CreateEntry(context.Background(), &domain.LibraryEntry{ //nolint:errcheck
			ContentType: domain.ContentTypeAdult,
			Kind:        domain.KindStudio,
			Name:        name,
		})
	}

	list, total, err := svc.ListEntries(context.Background(), ports.LibraryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
}

// ── Group tests ───────────────────────────────────────────────────────────────

func TestCreateGroup_NoEntryID(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.CreateGroup(context.Background(), &domain.Group{Title: "Season 1"})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for missing libraryEntryId, got %v", err)
	}
}

func TestCreateGroup_SetsDefaults(t *testing.T) {
	groups := newMockGroupRepo()
	svc := newSvc(newMockEntryRepo(), groups, newMockItemRepo())

	g := &domain.Group{LibraryEntryID: "entry-1", Title: "Season 1", Number: 1}
	if err := svc.CreateGroup(context.Background(), g); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if g.MonitorMode != domain.MonitorAll {
		t.Errorf("MonitorMode = %q, want all", g.MonitorMode)
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	_, err := svc.GetGroup(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ── Item tests ────────────────────────────────────────────────────────────────

func TestCreateItem_NoEntryID(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.CreateItem(context.Background(), &domain.Item{Title: "Scene 1"})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for missing libraryEntryId, got %v", err)
	}
}

func TestCreateItem_SetsDefaultStatus(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)

	item := &domain.Item{
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: "entry-1",
		Title:          "Scene 1",
	}
	if err := svc.CreateItem(context.Background(), item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if item.Status != domain.StatusWanted {
		t.Errorf("Status = %q, want wanted", item.Status)
	}
}

func TestGetItem_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	_, err := svc.GetItem(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSaveEntry(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())
	ctx := context.Background()

	e := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio A",
	}
	svc.CreateEntry(ctx, e) //nolint:errcheck

	e.Name = "Studio B"
	if err := svc.SaveEntry(ctx, e); err != nil {
		t.Fatalf("SaveEntry: %v", err)
	}
	got, _ := svc.GetEntry(ctx, e.ID)
	if got.Name != "Studio B" {
		t.Errorf("Name = %q, want Studio B", got.Name)
	}
}

func TestSaveEntry_ValidationError(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.SaveEntry(context.Background(), &domain.LibraryEntry{
		ID: "existing", ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
	})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for empty name, got %v", err)
	}
}

func TestListChildren(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())
	ctx := context.Background()

	network := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindNetwork, Name: "Network",
	}
	svc.CreateEntry(ctx, network) //nolint:errcheck

	for _, name := range []string{"Studio 1", "Studio 2"} {
		svc.CreateEntry(ctx, &domain.LibraryEntry{ //nolint:errcheck
			ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
			Name: name, ParentID: network.ID,
		})
	}

	children, total, err := svc.ListChildren(ctx, network.ID)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if total != 2 || len(children) != 2 {
		t.Errorf("children: total=%d len=%d, want 2", total, len(children))
	}
}

func TestListGroups(t *testing.T) {
	groups := newMockGroupRepo()
	svc := newSvc(newMockEntryRepo(), groups, newMockItemRepo())
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		svc.CreateGroup(ctx, &domain.Group{ //nolint:errcheck
			LibraryEntryID: "series-1", Title: "Season", Number: i,
		})
	}

	list, err := svc.ListGroups(ctx, ports.GroupFilter{LibraryEntryID: "series-1"})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
}

func TestSaveGroup(t *testing.T) {
	groups := newMockGroupRepo()
	svc := newSvc(newMockEntryRepo(), groups, newMockItemRepo())
	ctx := context.Background()

	g := &domain.Group{LibraryEntryID: "series-1", Title: "Season 1", Number: 1}
	svc.CreateGroup(ctx, g) //nolint:errcheck

	g.Title = "Season One"
	if err := svc.SaveGroup(ctx, g); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	got, _ := svc.GetGroup(ctx, g.ID)
	if got.Title != "Season One" {
		t.Errorf("Title = %q, want Season One", got.Title)
	}
}

func TestDeleteGroup(t *testing.T) {
	groups := newMockGroupRepo()
	svc := newSvc(newMockEntryRepo(), groups, newMockItemRepo())
	ctx := context.Background()

	g := &domain.Group{LibraryEntryID: "series-1", Title: "Season 1"}
	svc.CreateGroup(ctx, g) //nolint:errcheck

	if err := svc.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := svc.GetGroup(ctx, g.ID); !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListItems(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		svc.CreateItem(ctx, &domain.Item{ //nolint:errcheck
			ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio-1",
			Title: "Scene",
		})
	}

	list, total, err := svc.ListItems(ctx, ports.ItemFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Errorf("items: total=%d len=%d, want 3", total, len(list))
	}
}

func TestSaveItem(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)
	ctx := context.Background()

	item := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio-1", Title: "Scene A",
	}
	svc.CreateItem(ctx, item) //nolint:errcheck

	item.Title = "Scene B"
	if err := svc.SaveItem(ctx, item); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
	got, _ := svc.GetItem(ctx, item.ID)
	if got.Title != "Scene B" {
		t.Errorf("Title = %q, want Scene B", got.Title)
	}
}

func TestDeleteItem(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)
	ctx := context.Background()

	item := &domain.Item{
		ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio-1", Title: "Scene",
	}
	svc.CreateItem(ctx, item) //nolint:errcheck

	if err := svc.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if _, err := svc.GetItem(ctx, item.ID); !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// ── ImportArtistMembers tests ─────────────────────────────────────────────────

func TestImportArtistMembers_LinksMembers(t *testing.T) {
	entries := newMockEntryRepo()
	persons := newMockPersonRepo()
	svc := newSvcWithPersons(entries, newMockGroupRepo(), newMockItemRepo(), persons)
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles",
	}
	svc.CreateEntry(ctx, artist) //nolint:errcheck

	members := []domain.Person{
		{Name: "John Lennon"},
		{Name: "Paul McCartney"},
	}
	if err := svc.ImportArtistMembers(ctx, artist.ID, members, "member"); err != nil {
		t.Fatalf("ImportArtistMembers: %v", err)
	}

	if len(persons.data) != 2 {
		t.Errorf("persons count = %d, want 2", len(persons.data))
	}
	eps := entries.people[artist.ID]
	if len(eps) != 2 {
		t.Errorf("entry_people count = %d, want 2", len(eps))
	}
	for _, ep := range eps {
		if ep.Role != "member" {
			t.Errorf("role = %q, want member", ep.Role)
		}
		if ep.PersonID == "" {
			t.Error("PersonID should be set after import")
		}
	}
}

func TestImportArtistMembers_NotArtist(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvcWithPersons(entries, newMockGroupRepo(), newMockItemRepo(), newMockPersonRepo())
	ctx := context.Background()

	studio := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Evil Angel",
	}
	svc.CreateEntry(ctx, studio) //nolint:errcheck

	err := svc.ImportArtistMembers(ctx, studio.ID, []domain.Person{{Name: "Someone"}}, "member")
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for non-artist entry, got %v", err)
	}
}

func TestImportArtistMembers_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())
	err := svc.ImportArtistMembers(context.Background(), "nonexistent", nil, "member")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestImportArtistMembers_PersonSaveError(t *testing.T) {
	entries := newMockEntryRepo()
	persons := newMockPersonRepo()
	persons.saveErr = fmt.Errorf("db unavailable")
	svc := newSvcWithPersons(entries, newMockGroupRepo(), newMockItemRepo(), persons)
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles",
	}
	svc.CreateEntry(ctx, artist) //nolint:errcheck

	err := svc.ImportArtistMembers(ctx, artist.ID, []domain.Person{{Name: "Ringo Starr"}}, "member")
	if err == nil {
		t.Fatal("expected error from person Save, got nil")
	}
}

func TestImportArtistMembers_LinkSaveError(t *testing.T) {
	entries := newMockEntryRepo()
	entries.savePersonErr = fmt.Errorf("link insert failed")
	svc := newSvcWithPersons(entries, newMockGroupRepo(), newMockItemRepo(), newMockPersonRepo())
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles",
	}
	svc.CreateEntry(ctx, artist) //nolint:errcheck

	err := svc.ImportArtistMembers(ctx, artist.ID, []domain.Person{{Name: "George Harrison"}}, "member")
	if err == nil {
		t.Fatal("expected error from entry SavePerson, got nil")
	}
}

// ── Entry people method tests ─────────────────────────────────────────────────

func TestGetEntryPeople(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles",
	}
	svc.CreateEntry(ctx, artist) //nolint:errcheck

	entries.people[artist.ID] = []domain.EntryPerson{{PersonID: "p1", Role: "member"}}

	people, err := svc.GetEntryPeople(ctx, artist.ID)
	if err != nil {
		t.Fatalf("GetEntryPeople: %v", err)
	}
	if len(people) != 1 || people[0].PersonID != "p1" {
		t.Errorf("people = %v, want [{p1 member}]", people)
	}
}

func TestSaveAndRemoveEntryPerson(t *testing.T) {
	entries := newMockEntryRepo()
	svc := newSvc(entries, newMockGroupRepo(), newMockItemRepo())
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "The Beatles",
	}
	svc.CreateEntry(ctx, artist) //nolint:errcheck

	ep := domain.EntryPerson{PersonID: "p1", Role: "member"}
	if err := svc.SaveEntryPerson(ctx, artist.ID, ep); err != nil {
		t.Fatalf("SaveEntryPerson: %v", err)
	}
	if len(entries.people[artist.ID]) != 1 {
		t.Errorf("people count = %d, want 1", len(entries.people[artist.ID]))
	}

	if err := svc.RemoveEntryPerson(ctx, artist.ID, "p1", "member"); err != nil {
		t.Fatalf("RemoveEntryPerson: %v", err)
	}
	if len(entries.people[artist.ID]) != 0 {
		t.Errorf("people count = %d, want 0 after remove", len(entries.people[artist.ID]))
	}
}

// ── UpdateItemStatus tests ────────────────────────────────────────────────────

func TestUpdateItemStatus_ValidTransitions(t *testing.T) {
	cases := []struct {
		name       string
		fromStatus domain.ItemStatus
		toStatus   domain.ItemStatus
	}{
		{"wanted→wanted", domain.StatusWanted, domain.StatusWanted},
		{"wanted→skipped", domain.StatusWanted, domain.StatusSkipped},
		{"skipped→wanted", domain.StatusSkipped, domain.StatusWanted},
		{"missing→wanted", domain.StatusMissing, domain.StatusWanted},
		{"missing→skipped", domain.StatusMissing, domain.StatusSkipped},
		{"imported→wanted", domain.StatusImported, domain.StatusWanted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := newMockItemRepo()
			svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)

			item := &domain.Item{ID: "i1", LibraryEntryID: "e1", Status: tc.fromStatus}
			items.data["i1"] = item

			if err := svc.UpdateItemStatus(context.Background(), "i1", tc.toStatus); err != nil {
				t.Fatalf("UpdateItemStatus: %v", err)
			}
			if items.data["i1"].Status != tc.toStatus {
				t.Errorf("status = %q, want %q", items.data["i1"].Status, tc.toStatus)
			}
		})
	}
}

func TestUpdateItemStatus_InvalidUserStatus(t *testing.T) {
	pipelineStatuses := []domain.ItemStatus{
		domain.StatusGrabbed,
		domain.StatusDownloading,
		domain.StatusImported,
		domain.StatusMissing,
	}
	for _, s := range pipelineStatuses {
		t.Run(string(s), func(t *testing.T) {
			items := newMockItemRepo()
			svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)
			items.data["i1"] = &domain.Item{ID: "i1", LibraryEntryID: "e1", Status: domain.StatusWanted}

			err := svc.UpdateItemStatus(context.Background(), "i1", s)
			if !errors.Is(err, library.ErrInvalidStatusForUserUpdate) {
				t.Errorf("expected ErrInvalidStatusForUserUpdate, got %v", err)
			}
		})
	}
}

func TestUpdateItemStatus_InvalidTransition(t *testing.T) {
	items := newMockItemRepo()
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), items)
	// grabbed is pipeline-locked; no user transitions are allowed from it
	items.data["i1"] = &domain.Item{ID: "i1", LibraryEntryID: "e1", Status: domain.StatusGrabbed}

	err := svc.UpdateItemStatus(context.Background(), "i1", domain.StatusWanted)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestUpdateItemStatus_NotFound(t *testing.T) {
	svc := newSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo())

	err := svc.UpdateItemStatus(context.Background(), "nonexistent", domain.StatusWanted)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
