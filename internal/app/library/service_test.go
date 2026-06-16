package library_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"purser/internal/app/errs"
	"purser/internal/app/library"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── Hand-rolled mocks ─────────────────────────────────────────────────────────

type mockEntryRepo struct {
	data    map[string]*domain.LibraryEntry
	saveErr error
}

func newMockEntryRepo() *mockEntryRepo {
	return &mockEntryRepo{data: make(map[string]*domain.LibraryEntry)}
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
	var res []*domain.Group
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
	var res []*domain.Item
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

func newSvc(entries *mockEntryRepo, groups *mockGroupRepo, items *mockItemRepo) *library.Service {
	return library.New(entries, groups, items)
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
