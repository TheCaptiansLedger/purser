package afterdark_test

import (
	"context"
	"errors"
	dsbadger "purser/internal/adapters/datastore/badger"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/adapters/store/externalid"
	"purser/internal/adapters/store/itemperson"
	"purser/internal/adapters/store/libraryentry"
	"purser/internal/adapters/store/person"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// templateDataBuilderDeps bundles a TemplateDataBuilder backed by real
// Badger-backed repositories, the same real-storage convention
// persisterDeps and music's own templateDataBuilderDeps use.
type templateDataBuilderDeps struct {
	libraryEntries ports.LibraryEntryRepository
	itemPeople     ports.ItemPersonRepository
	people         ports.PersonRepository
	externalIDs    ports.ExternalIDRepository
	builder        *afterdark.TemplateDataBuilder
}

func newTemplateDataBuilderDeps(t *testing.T) *templateDataBuilderDeps {
	t.Helper()
	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})
	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}

	libraryEntries, err := libraryentry.New("test", ds)
	if err != nil {
		t.Fatalf("libraryentry.New returned error: %v", err)
	}
	itemPeople, err := itemperson.New("test", ds)
	if err != nil {
		t.Fatalf("itemperson.New returned error: %v", err)
	}
	people, err := person.New("test", ds)
	if err != nil {
		t.Fatalf("person.New returned error: %v", err)
	}
	externalIDs, err := externalid.New("test", ds)
	if err != nil {
		t.Fatalf("externalid.New returned error: %v", err)
	}

	return &templateDataBuilderDeps{
		libraryEntries: libraryEntries, itemPeople: itemPeople, people: people, externalIDs: externalIDs,
		builder: afterdark.NewTemplateDataBuilder(libraryEntries, itemPeople, people, externalIDs),
	}
}

// seedStudio creates a LibraryEntry(Kind=studio), returning its ID.
func (d *templateDataBuilderDeps) seedStudio(t *testing.T, name string) string {
	t.Helper()
	studio := &domain.LibraryEntry{
		ID: domain.NewID(), ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: name, MonitorMode: domain.MonitorModeAll,
	}
	if err := d.libraryEntries.Create(context.Background(), studio); err != nil {
		t.Fatalf("creating fixture studio: %v", err)
	}
	return studio.ID
}

// seedScene creates a fixture Item(ContentType=adult) belonging to studioID.
func (d *templateDataBuilderDeps) seedScene(t *testing.T, studioID string, date *time.Time) *domain.Item {
	t.Helper()
	item := &domain.Item{
		ID: domain.NewID(), ContentType: domain.ContentTypeAdult, LibraryEntryID: studioID,
		Title: "Scene Title", Date: date, Status: domain.ItemStatusImported,
	}
	if err := item.Validate(); err != nil {
		t.Fatalf("validating fixture item: %v", err)
	}
	return item
}

// creditPerformer creates a Person and an ItemPerson credit on itemID —
// creditedAs empty means the credit uses the Person's own canonical Name.
func (d *templateDataBuilderDeps) creditPerformer(t *testing.T, itemID, name, creditedAs string) {
	t.Helper()
	ctx := context.Background()

	p := &domain.Person{ID: domain.NewID(), Name: name, Gender: domain.GenderFemale, MonitorMode: domain.MonitorModeAll}
	if err := d.people.Create(ctx, p); err != nil {
		t.Fatalf("creating fixture person: %v", err)
	}

	ip := &domain.ItemPerson{ItemID: itemID, PersonID: p.ID, Role: "performer", CreditedAs: creditedAs}
	if err := ip.Validate(); err != nil {
		t.Fatalf("validating fixture item person: %v", err)
	}
	if err := d.itemPeople.Create(ctx, ip); err != nil {
		t.Fatalf("creating fixture item person: %v", err)
	}
}

// linkJAVCode links itemID to code via a jav_code ExternalID.
func (d *templateDataBuilderDeps) linkJAVCode(t *testing.T, itemID, code string) {
	t.Helper()
	ext := &domain.ExternalID{EntityType: domain.EntityTypeItem, EntityID: itemID, Source: domain.ExternalIDSourceJAVCode, Value: code}
	if err := d.externalIDs.Create(context.Background(), ext); err != nil {
		t.Fatalf("creating fixture jav code external id: %v", err)
	}
}

func TestTemplateDataBuilder_ContentTypes(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	got := deps.builder.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func TestTemplateDataBuilder_BuildTemplateData_FullMapping(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	studioID := deps.seedStudio(t, "Stash Studio")
	date := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	item := deps.seedScene(t, studioID, &date)
	deps.creditPerformer(t, item.ID, "Jane Doe", "Jane D.")
	deps.creditPerformer(t, item.ID, "John Roe", "")
	deps.linkJAVCode(t, item.ID, "ABC-123")

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}

	if data["Studio"] != "Stash Studio" {
		t.Errorf("data[\"Studio\"] = %v, want %q", data["Studio"], "Stash Studio")
	}
	if data["SceneTitle"] != "Scene Title" {
		t.Errorf("data[\"SceneTitle\"] = %v, want %q", data["SceneTitle"], "Scene Title")
	}
	if data["SceneDate"] != "2024-01-15" {
		t.Errorf("data[\"SceneDate\"] = %v, want %q", data["SceneDate"], "2024-01-15")
	}
	if data["SceneCode"] != "ABC-123" {
		t.Errorf("data[\"SceneCode\"] = %v, want %q", data["SceneCode"], "ABC-123")
	}
	performers, ok := data["Performers"].([]string)
	if !ok {
		t.Fatalf("data[\"Performers\"] is %T, want []string", data["Performers"])
	}
	want := map[string]bool{"Jane D.": true, "John Roe": true}
	if len(performers) != 2 {
		t.Fatalf("Performers = %v, want 2 entries", performers)
	}
	for _, p := range performers {
		if !want[p] {
			t.Errorf("Performers contains unexpected entry %q", p)
		}
	}
}

func TestTemplateDataBuilder_BuildTemplateData_NoDateOmitsSceneDate(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	studioID := deps.seedStudio(t, "Studio")
	item := deps.seedScene(t, studioID, nil)

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if data["SceneDate"] != "" {
		t.Errorf("data[\"SceneDate\"] = %v, want \"\" for a scene with no Date", data["SceneDate"])
	}
}

func TestTemplateDataBuilder_BuildTemplateData_NoJAVCodeOmitsSceneCode(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	studioID := deps.seedStudio(t, "Studio")
	item := deps.seedScene(t, studioID, nil)

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if data["SceneCode"] != "" {
		t.Errorf("data[\"SceneCode\"] = %v, want \"\" for a non-JAV scene", data["SceneCode"])
	}
}

func TestTemplateDataBuilder_BuildTemplateData_NoPerformersReturnsEmptySlice(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	studioID := deps.seedStudio(t, "Studio")
	item := deps.seedScene(t, studioID, nil)

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if len(data["Performers"].([]string)) != 0 {
		t.Errorf("data[\"Performers\"] = %v, want empty for a scene with no credits", data["Performers"])
	}
}

func TestTemplateDataBuilder_BuildTemplateData_MissingStudioIsError(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	item := &domain.Item{
		ID: domain.NewID(), ContentType: domain.ContentTypeAdult, LibraryEntryID: "no-such-studio",
		Title: "Orphan Scene", Status: domain.ItemStatusImported,
	}

	if _, err := deps.builder.BuildTemplateData(context.Background(), item); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("BuildTemplateData returned %v, want ErrNotFound for a missing Studio", err)
	}
}
