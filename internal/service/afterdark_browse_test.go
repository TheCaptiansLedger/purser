package service_test

import (
	"context"
	"fmt"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"purser/internal/service"
	"sort"
	"testing"
)

// The fakes in this file deliberately implement real filtering/pagination
// (unlike the simpler pass-through fakes elsewhere in this package) because
// AfterDarkBrowseService's entire job is composing across filtered List
// calls — a fake that ignores filters would validate nothing.

func paginateByID[T any](sorted []T, idOf func(T) string, pageSize int, pageToken string) ([]T, string) {
	start := 0
	if pageToken != "" {
		for i, v := range sorted {
			if idOf(v) == pageToken {
				start = i + 1
				break
			}
		}
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	end := min(start+pageSize, len(sorted))
	var next string
	if end < len(sorted) {
		next = idOf(sorted[end-1])
	}
	return sorted[start:end], next
}

type browseFakeLibraryEntryRepository struct {
	byID map[string]*domain.LibraryEntry
}

func (f *browseFakeLibraryEntryRepository) Create(context.Context, *domain.LibraryEntry) error {
	return nil
}

func (f *browseFakeLibraryEntryRepository) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return e, nil
}

func (f *browseFakeLibraryEntryRepository) Update(context.Context, *domain.LibraryEntry) error {
	return nil
}
func (f *browseFakeLibraryEntryRepository) Delete(context.Context, string) error { return nil }
func (f *browseFakeLibraryEntryRepository) List(_ context.Context, kind domain.Kind, parentID string, pageSize int, pageToken string) ([]*domain.LibraryEntry, string, error) {
	var matched []*domain.LibraryEntry
	for _, e := range f.byID {
		if kind != "" && e.Kind != kind {
			continue
		}
		if parentID != "" && e.ParentID != parentID {
			continue
		}
		matched = append(matched, e)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	page, next := paginateByID(matched, func(e *domain.LibraryEntry) string { return e.ID }, pageSize, pageToken)
	return page, next, nil
}

type browseFakeItemRepository struct {
	byID map[string]*domain.Item
}

func (f *browseFakeItemRepository) Create(context.Context, *domain.Item) error { return nil }
func (f *browseFakeItemRepository) Get(_ context.Context, id string) (*domain.Item, error) {
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return i, nil
}
func (f *browseFakeItemRepository) Update(context.Context, *domain.Item) error { return nil }
func (f *browseFakeItemRepository) Delete(context.Context, string) error       { return nil }
func (f *browseFakeItemRepository) List(_ context.Context, libraryEntryID, contentType, groupID string, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	var matched []*domain.Item
	for _, i := range f.byID {
		if libraryEntryID != "" && i.LibraryEntryID != libraryEntryID {
			continue
		}
		if contentType != "" && string(i.ContentType) != contentType {
			continue
		}
		if groupID != "" && i.GroupID != groupID {
			continue
		}
		matched = append(matched, i)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	page, next := paginateByID(matched, func(i *domain.Item) string { return i.ID }, pageSize, pageToken)
	return page, next, nil
}

type browseFakeItemPersonRepository struct {
	rows []*domain.ItemPerson
}

func (f *browseFakeItemPersonRepository) Create(context.Context, *domain.ItemPerson) error {
	return nil
}

func (f *browseFakeItemPersonRepository) Get(context.Context, string, string, string) (*domain.ItemPerson, error) {
	return nil, ports.ErrNotFound
}

func (f *browseFakeItemPersonRepository) Update(context.Context, *domain.ItemPerson) error {
	return nil
}

func (f *browseFakeItemPersonRepository) Delete(context.Context, string, string, string) error {
	return nil
}

func (f *browseFakeItemPersonRepository) List(_ context.Context, itemID, personID string, pageSize int, pageToken string) ([]*domain.ItemPerson, string, error) {
	var matched []*domain.ItemPerson
	for _, r := range f.rows {
		if itemID != "" && r.ItemID != itemID {
			continue
		}
		if personID != "" && r.PersonID != personID {
			continue
		}
		matched = append(matched, r)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ItemID+matched[i].PersonID < matched[j].ItemID+matched[j].PersonID
	})
	key := func(r *domain.ItemPerson) string { return r.ItemID + "|" + r.PersonID + "|" + r.Role }
	page, next := paginateByID(matched, key, pageSize, pageToken)
	return page, next, nil
}

type browseFakePersonRepository struct {
	byID map[string]*domain.Person
}

func (f *browseFakePersonRepository) Create(context.Context, *domain.Person) error { return nil }
func (f *browseFakePersonRepository) Get(_ context.Context, id string) (*domain.Person, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return p, nil
}
func (f *browseFakePersonRepository) Update(context.Context, *domain.Person) error { return nil }
func (f *browseFakePersonRepository) Delete(context.Context, string) error         { return nil }
func (f *browseFakePersonRepository) List(context.Context, int, string) ([]*domain.Person, string, error) {
	return nil, "", nil
}

type browseFakePerformerProfileRepository struct {
	byID      map[string]*afterdark.PerformerProfile
	getErr    error
	deleteErr error
}

func (f *browseFakePerformerProfileRepository) Create(context.Context, *afterdark.PerformerProfile) error {
	return nil
}

func (f *browseFakePerformerProfileRepository) Get(_ context.Context, personID string) (*afterdark.PerformerProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.byID[personID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return p, nil
}

func (f *browseFakePerformerProfileRepository) Update(context.Context, *afterdark.PerformerProfile) error {
	return nil
}

func (f *browseFakePerformerProfileRepository) Delete(_ context.Context, personID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[personID]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, personID)
	return nil
}

func (f *browseFakePerformerProfileRepository) List(_ context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerProfile, string, error) {
	sorted := make([]*afterdark.PerformerProfile, 0, len(f.byID))
	for _, p := range f.byID {
		sorted = append(sorted, p)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PersonID < sorted[j].PersonID })
	page, next := paginateByID(sorted, func(p *afterdark.PerformerProfile) string { return p.PersonID }, pageSize, pageToken)
	return page, next, nil
}

// browseFixture builds a network -> studio -> scene -> performer graph:
//
//	network1 -> studio1 -> scene1 {p1, p2}, scene2 {p1}
//	network1 -> studio2 -> scene3 {p3}
//	network2 -> studio3 -> scene4 {p1}  (a different network entirely)
//
// p4 has a PerformerProfile but no credits anywhere. p5 is credited on
// scene1 but deliberately has no PerformerProfile — it must never appear
// in a PerformerView.
func browseFixture() *service.AfterDarkBrowseService {
	libraryEntries := &browseFakeLibraryEntryRepository{byID: map[string]*domain.LibraryEntry{
		"network1": {ID: "network1", Kind: domain.KindNetwork},
		"network2": {ID: "network2", Kind: domain.KindNetwork},
		"studio1":  {ID: "studio1", Kind: domain.KindStudio, ParentID: "network1"},
		"studio2":  {ID: "studio2", Kind: domain.KindStudio, ParentID: "network1"},
		"studio3":  {ID: "studio3", Kind: domain.KindStudio, ParentID: "network2"},
	}}
	items := &browseFakeItemRepository{byID: map[string]*domain.Item{
		"scene1": {ID: "scene1", ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio1"},
		"scene2": {ID: "scene2", ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio1"},
		"scene3": {ID: "scene3", ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio2"},
		"scene4": {ID: "scene4", ContentType: domain.ContentTypeAdult, LibraryEntryID: "studio3"},
	}}
	itemPeople := &browseFakeItemPersonRepository{rows: []*domain.ItemPerson{
		{ItemID: "scene1", PersonID: "p1", Role: "performer"},
		{ItemID: "scene1", PersonID: "p2", Role: "performer"},
		{ItemID: "scene1", PersonID: "p5", Role: "performer"},
		{ItemID: "scene2", PersonID: "p1", Role: "performer"},
		{ItemID: "scene3", PersonID: "p3", Role: "performer"},
		{ItemID: "scene4", PersonID: "p1", Role: "performer"},
	}}
	people := &browseFakePersonRepository{byID: map[string]*domain.Person{
		"p1": {ID: "p1", Name: "Performer One"},
		"p2": {ID: "p2", Name: "Performer Two"},
		"p3": {ID: "p3", Name: "Performer Three"},
		"p4": {ID: "p4", Name: "Performer Four"},
		"p5": {ID: "p5", Name: "Performer Five (no profile)"},
	}}
	profiles := &browseFakePerformerProfileRepository{byID: map[string]*afterdark.PerformerProfile{
		"p1": {PersonID: "p1"},
		"p2": {PersonID: "p2"},
		"p3": {PersonID: "p3"},
		"p4": {PersonID: "p4"},
	}}

	return service.NewAfterDarkBrowseService(libraryEntries, items, itemPeople, people, profiles)
}

func itemIDs(items []*domain.Item) []string {
	ids := make([]string, 0, len(items))
	for _, i := range items {
		ids = append(ids, i.ID)
	}
	sort.Strings(ids)
	return ids
}

func performerIDs(views []*afterdark.PerformerView) []string {
	ids := make([]string, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.Person.ID)
	}
	sort.Strings(ids)
	return ids
}

func assertIDs(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got IDs %v, want %v", got, want)
	}
}

func TestAfterDarkBrowseService_ListScenesInNetwork(t *testing.T) {
	svc := browseFixture()

	scenes, _, err := svc.ListScenesInNetwork(context.Background(), "network1", 10, "")
	if err != nil {
		t.Fatalf("ListScenesInNetwork returned error: %v", err)
	}
	assertIDs(t, itemIDs(scenes), []string{"scene1", "scene2", "scene3"})
}

func TestAfterDarkBrowseService_ListScenesInNetwork_Paginates(t *testing.T) {
	svc := browseFixture()

	var got []string
	pageToken := ""
	for {
		scenes, next, err := svc.ListScenesInNetwork(context.Background(), "network1", 1, pageToken)
		if err != nil {
			t.Fatalf("ListScenesInNetwork returned error: %v", err)
		}
		got = append(got, itemIDs(scenes)...)
		if next == "" {
			break
		}
		pageToken = next
	}
	sort.Strings(got)
	assertIDs(t, got, []string{"scene1", "scene2", "scene3"})
}

func TestAfterDarkBrowseService_ListScenesForPerformer(t *testing.T) {
	svc := browseFixture()

	scenes, _, err := svc.ListScenesForPerformer(context.Background(), "p1", 10, "")
	if err != nil {
		t.Fatalf("ListScenesForPerformer returned error: %v", err)
	}
	assertIDs(t, itemIDs(scenes), []string{"scene1", "scene2", "scene4"})
}

func TestAfterDarkBrowseService_ListScenesForPerformer_NoCredits(t *testing.T) {
	svc := browseFixture()

	scenes, _, err := svc.ListScenesForPerformer(context.Background(), "p4", 10, "")
	if err != nil {
		t.Fatalf("ListScenesForPerformer returned error: %v", err)
	}
	if len(scenes) != 0 {
		t.Fatalf("ListScenesForPerformer for an uncredited performer returned %d scenes, want 0", len(scenes))
	}
}

func TestAfterDarkBrowseService_ListPerformersForScene(t *testing.T) {
	svc := browseFixture()

	views, _, err := svc.ListPerformersForScene(context.Background(), "scene1", 10, "")
	if err != nil {
		t.Fatalf("ListPerformersForScene returned error: %v", err)
	}
	// p5 is credited on scene1 but has no PerformerProfile — must be excluded.
	assertIDs(t, performerIDs(views), []string{"p1", "p2"})
}

func TestAfterDarkBrowseService_ListPerformersForStudio(t *testing.T) {
	svc := browseFixture()

	views, _, err := svc.ListPerformersForStudio(context.Background(), "studio1", 10, "")
	if err != nil {
		t.Fatalf("ListPerformersForStudio returned error: %v", err)
	}
	assertIDs(t, performerIDs(views), []string{"p1", "p2"})
}

func TestAfterDarkBrowseService_ListPerformersForNetwork(t *testing.T) {
	svc := browseFixture()

	views, _, err := svc.ListPerformersForNetwork(context.Background(), "network1", 10, "")
	if err != nil {
		t.Fatalf("ListPerformersForNetwork returned error: %v", err)
	}
	assertIDs(t, performerIDs(views), []string{"p1", "p2", "p3"})
}

func TestAfterDarkBrowseService_ListPerformers(t *testing.T) {
	svc := browseFixture()

	views, _, err := svc.ListPerformers(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("ListPerformers returned error: %v", err)
	}
	// Every Person with a PerformerProfile, regardless of credits — p4 has
	// no scenes but does have a profile, so it appears here even though it
	// never appears in any of the scene/studio/network-scoped listings.
	assertIDs(t, performerIDs(views), []string{"p1", "p2", "p3", "p4"})
}

func TestAfterDarkBrowseService_ListScenesInNetwork_UnknownNetworkReturnsEmpty(t *testing.T) {
	svc := browseFixture()

	scenes, _, err := svc.ListScenesInNetwork(context.Background(), "no-such-network", 10, "")
	if err != nil {
		t.Fatalf("ListScenesInNetwork returned error: %v", err)
	}
	if len(scenes) != 0 {
		t.Fatalf("ListScenesInNetwork for an unknown network returned %d scenes, want 0", len(scenes))
	}
}

func TestAfterDarkBrowseService_PropagatesRepositoryErrors(t *testing.T) {
	items := &browseFakeItemRepository{byID: map[string]*domain.Item{}}
	libraryEntries := &browseFakeLibraryEntryRepository{byID: map[string]*domain.LibraryEntry{
		"studio1": {ID: "studio1", Kind: domain.KindStudio, ParentID: "network1"},
	}}
	itemPeople := &browseFakeItemPersonRepository{}
	people := &browseFakePersonRepository{byID: map[string]*domain.Person{}}
	profiles := &browseFakePerformerProfileRepository{byID: map[string]*afterdark.PerformerProfile{}}

	svc := service.NewAfterDarkBrowseService(libraryEntries, items, itemPeople, people, profiles)

	_, _, err := svc.ListScenesForPerformer(context.Background(), "missing", 10, "")
	if err != nil {
		t.Fatalf("ListScenesForPerformer with no credits returned error: %v, want nil (empty result)", err)
	}

	// ListScenesInNetwork should tolerate a studio with no scenes at all.
	scenes, _, err := svc.ListScenesInNetwork(context.Background(), "network1", 10, "")
	if err != nil {
		t.Fatalf("ListScenesInNetwork returned error: %v", err)
	}
	if len(scenes) != 0 {
		t.Fatalf("ListScenesInNetwork for a studio with no scenes returned %d scenes, want 0", len(scenes))
	}
}
