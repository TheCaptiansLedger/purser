package afterdark_test

import (
	"context"
	dsbadger "purser/internal/adapters/datastore/badger"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/adapters/store/externalid"
	"purser/internal/adapters/store/item"
	"purser/internal/adapters/store/itemperson"
	"purser/internal/adapters/store/libraryentry"
	"purser/internal/adapters/store/mediafile"
	"purser/internal/adapters/store/performerprofile"
	"purser/internal/adapters/store/person"
	"purser/internal/adapters/store/tag"
	"purser/internal/adapters/store/tagassignment"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"sync"
	"testing"
)

// fakeAfterDarkOrganizer is a ports.Organizer test double recording every
// mediaFileID it was called with — see music/persister_test.go's
// fakeOrganizer for the reasoning (this package's own copy, since Go test
// packages don't share unexported types across packages).
type fakeAfterDarkOrganizer struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeAfterDarkOrganizer) Organize(_ context.Context, mediaFileID string) (*domain.MediaFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, mediaFileID)
	return &domain.MediaFile{ID: mediaFileID}, nil
}

func (f *fakeAfterDarkOrganizer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// persisterDeps bundles a Persister backed by real Badger-backed
// repositories (needed to genuinely exercise get-or-create races and
// ADR-0026's reservation mechanism, same reasoning music/persister_test.go
// gives) plus fakeStashDB/fakeThePornDB doubles (identifier_test.go) for
// the two provider ports.
type persisterDeps struct {
	stashDB           *fakeStashDB
	tpdb              *fakeThePornDB
	externalIDs       ports.ExternalIDRepository
	libraryEntries    ports.LibraryEntryRepository
	persons           ports.PersonRepository
	performerProfiles ports.PerformerProfileRepository
	itemPeople        ports.ItemPersonRepository
	items             ports.ItemRepository
	mediaFiles        ports.MediaFileRepository
	tags              ports.TagRepository
	tagAssignments    ports.TagAssignmentRepository
	organizer         *fakeAfterDarkOrganizer
}

func newPersisterDeps(t *testing.T) *persisterDeps {
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

	externalIDs, err := externalid.New("test", ds)
	if err != nil {
		t.Fatalf("externalid.New returned error: %v", err)
	}
	libraryEntries, err := libraryentry.New("test", ds)
	if err != nil {
		t.Fatalf("libraryentry.New returned error: %v", err)
	}
	persons, err := person.New("test", ds)
	if err != nil {
		t.Fatalf("person.New returned error: %v", err)
	}
	performerProfiles, err := performerprofile.New("test", ds)
	if err != nil {
		t.Fatalf("performerprofile.New returned error: %v", err)
	}
	itemPeople, err := itemperson.New("test", ds)
	if err != nil {
		t.Fatalf("itemperson.New returned error: %v", err)
	}
	items, err := item.New("test", ds)
	if err != nil {
		t.Fatalf("item.New returned error: %v", err)
	}
	mediaFiles, err := mediafile.New("test", ds)
	if err != nil {
		t.Fatalf("mediafile.New returned error: %v", err)
	}
	tags, err := tag.New("test", ds)
	if err != nil {
		t.Fatalf("tag.New returned error: %v", err)
	}
	tagAssignments, err := tagassignment.New("test", ds)
	if err != nil {
		t.Fatalf("tagassignment.New returned error: %v", err)
	}

	return &persisterDeps{
		stashDB: newFakeStashDB(), tpdb: newFakeThePornDB(),
		externalIDs: externalIDs, libraryEntries: libraryEntries, persons: persons,
		performerProfiles: performerProfiles, itemPeople: itemPeople, items: items,
		mediaFiles: mediaFiles, tags: tags, tagAssignments: tagAssignments,
		organizer: &fakeAfterDarkOrganizer{},
	}
}

// newPersister builds a Persister from d with providerPriority.
func (d *persisterDeps) newPersister(providerPriority []string) *afterdark.Persister {
	return afterdark.NewPersister(
		d.stashDB, d.tpdb, d.externalIDs, d.libraryEntries, d.persons, d.performerProfiles,
		d.itemPeople, d.items, d.mediaFiles, d.tags, d.tagAssignments, d.organizer, providerPriority,
	)
}

// stashScene is a fully-populated StashDB fixture scene, wired into
// d.stashDB under id.
func (d *persisterDeps) putStashScene(id string) ports.Scene {
	scene := ports.Scene{
		ID:          id,
		Title:       "Stash Title",
		Details:     "Stash overview",
		ReleaseDate: "2024-01-15",
		Studio:      &ports.Studio{ID: "stash-studio-1", Name: "Stash Studio"},
		Tags:        []ports.Tag{{ID: "t1", Name: "Anal"}, {ID: "t2", Name: "Blonde"}},
		Performers: []ports.PerformerAppearance{
			{
				Performer: ports.Performer{
					ID: "stash-perf-1", Name: "Jane Doe", Gender: "FEMALE",
					BirthDate: "1995-05-01", Country: "USA",
					CupSize: "C", BandSize: 34, BreastType: "Natural",
					CareerStartYear: 2015,
				},
				As: "Jane D.",
			},
		},
	}
	d.stashDB.scenesByID[id] = scene
	return scene
}

// tpdbScene is a fully-populated ThePornDB fixture scene, wired into
// d.tpdb under id.
func (d *persisterDeps) putTPDBScene(id string) ports.TPDBScene {
	scene := ports.TPDBScene{
		ID:          id,
		Title:       "TPDB Title",
		Description: "TPDB overview",
		Date:        "2024-01-16",
		Type:        "Scene",
		Site:        &ports.TPDBSite{UUID: "tpdb-site-1", Name: "TPDB Site"},
		Tags:        []ports.TPDBTag{{ID: 1, UUID: "tt1", Name: "Blowjob"}, {ID: 2, UUID: "tt2", Name: "anal"}},
		Performers: []ports.TPDBPerformer{
			{
				ID: "tpdb-perf-1", Name: "Jane Doe TPDB",
				Extras: ports.TPDBPerformerExtras{Gender: "Female", Nationality: "American", CareerStartYear: 2016},
			},
		},
	}
	d.tpdb.scenesByID[id] = scene
	return scene
}

// putTPDBAliasScene wires a scene whose only performer is a site-specific
// alias record (Parent != nil) — resolveTPDBPerformer's alias-handling
// path, per docs/technical/afterdark-data_model.md §5 decision 5.
func (d *persisterDeps) putTPDBAliasScene(id string) ports.TPDBScene {
	scene := ports.TPDBScene{
		ID: id, Title: "TPDB Title", Type: "Scene",
		Site: &ports.TPDBSite{UUID: "tpdb-site-1", Name: "TPDB Site"},
		Performers: []ports.TPDBPerformer{
			{
				ID: "tpdb-alias-1", Name: "Anikka",
				Parent: &ports.TPDBPerformer{
					ID: "tpdb-canonical-1", Name: "Anikka Albrite",
					Extras: ports.TPDBPerformerExtras{Gender: "Female"},
				},
			},
		},
	}
	d.tpdb.scenesByID[id] = scene
	return scene
}

func stashCandidate(externalRef string) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: externalRef, Title: "Stash Title", Score: 0.9,
		Tier: domain.MatchTierUniqueID, Metadata: map[string]any{"source": "stashdb"},
	}
}

func tpdbCandidate(externalRef string) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: externalRef, Title: "TPDB Title", Score: 0.85,
		Tier: domain.MatchTierUniqueID, Metadata: map[string]any{"source": "tpdb"},
	}
}

func sceneFile() *domain.UnmatchedFile {
	return &domain.UnmatchedFile{
		ID: domain.NewID(), Path: "/media/scene.mp4", ContentType: domain.ContentTypeAdult,
		GroupKey: "/media/scene.mp4", OSHash: "oshash-scene", SHA1: "sha1-scene",
		Status: domain.UnmatchedFileStatusPending,
	}
}

func TestPersister_ContentTypes(t *testing.T) {
	d := newPersisterDeps(t)
	p := d.newPersister(nil)
	got := p.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func TestPersister_Persist_SingleProviderClears(t *testing.T) {
	d := newPersisterDeps(t)
	d.putStashScene("stash-scene-1")
	p := d.newPersister([]string{"stashdb", "tpdb"})
	uf := sceneFile()

	if err := p.Persist(context.Background(), nil, []domain.MatchCandidate{stashCandidate("stash-scene-1")}, []*domain.UnmatchedFile{uf}); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	linked, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceStashDB, "stash-scene-1")
	if err != nil {
		t.Fatalf("GetByValue(item, stashdb) returned error: %v", err)
	}
	itemRow, err := d.items.Get(context.Background(), linked.EntityID)
	if err != nil {
		t.Fatalf("items.Get returned error: %v", err)
	}
	if itemRow.Title != "Stash Title" || itemRow.Overview != "Stash overview" {
		t.Errorf("item = %+v, want Title=Stash Title Overview=Stash overview", itemRow)
	}
	if itemRow.Status != domain.ItemStatusImported {
		t.Errorf("item.Status = %v, want imported", itemRow.Status)
	}
	if itemRow.Metadata["kind"] != "scene" {
		t.Errorf(`item.Metadata["kind"] = %v, want "scene"`, itemRow.Metadata["kind"])
	}

	studio, err := d.libraryEntries.Get(context.Background(), itemRow.LibraryEntryID)
	if err != nil {
		t.Fatalf("libraryEntries.Get returned error: %v", err)
	}
	if studio.Kind != domain.KindStudio || studio.Name != "Stash Studio" {
		t.Errorf("studio = %+v, want Kind=studio Name=Stash Studio", studio)
	}

	assignments, _, err := d.tagAssignments.List(context.Background(), "", domain.EntityTypeItem, itemRow.ID, 100, "")
	if err != nil {
		t.Fatalf("tagAssignments.List returned error: %v", err)
	}
	if len(assignments) != 2 {
		t.Errorf("len(assignments) = %d, want 2 (Anal, Blonde)", len(assignments))
	}

	credits, _, err := d.itemPeople.List(context.Background(), itemRow.ID, "", 100, "")
	if err != nil {
		t.Fatalf("itemPeople.List returned error: %v", err)
	}
	if len(credits) != 1 || credits[0].CreditedAs != "Jane D." || credits[0].Role != "performer" {
		t.Fatalf("credits = %+v, want one credit CreditedAs=Jane D. Role=performer", credits)
	}
	performer, err := d.persons.Get(context.Background(), credits[0].PersonID)
	if err != nil {
		t.Fatalf("persons.Get returned error: %v", err)
	}
	if performer.Name != "Jane Doe" || performer.Gender != domain.GenderFemale {
		t.Errorf("performer = %+v, want Name=Jane Doe Gender=FEMALE", performer)
	}
	profile, err := d.performerProfiles.Get(context.Background(), performer.ID)
	if err != nil {
		t.Fatalf("performerProfiles.Get returned error: %v", err)
	}
	if profile.CupSize != "C" || profile.BandSize != "34" {
		t.Errorf("profile = %+v, want CupSize=C BandSize=34", profile)
	}

	mediaFiles, _, err := d.mediaFiles.List(context.Background(), itemRow.ID, 100, "")
	if err != nil {
		t.Fatalf("mediaFiles.List returned error: %v", err)
	}
	if len(mediaFiles) != 1 || mediaFiles[0].Path != uf.Path {
		t.Fatalf("mediaFiles = %+v, want one row for %s", mediaFiles, uf.Path)
	}

	if d.organizer.callCount() != 1 {
		t.Errorf("organizer.callCount() = %d, want 1", d.organizer.callCount())
	}
}

func TestPersister_Persist_BothProvidersMergeIntoOneScene(t *testing.T) {
	d := newPersisterDeps(t)
	d.putStashScene("stash-scene-1")
	d.putTPDBScene("tpdb-scene-1")
	p := d.newPersister([]string{"stashdb", "tpdb"})

	err := p.Persist(context.Background(), nil,
		[]domain.MatchCandidate{stashCandidate("stash-scene-1"), tpdbCandidate("tpdb-scene-1")},
		[]*domain.UnmatchedFile{sceneFile()})
	if err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	fromStash, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceStashDB, "stash-scene-1")
	if err != nil {
		t.Fatalf("GetByValue(item, stashdb) returned error: %v", err)
	}
	fromTPDB, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceTPDB, "tpdb-scene-1")
	if err != nil {
		t.Fatalf("GetByValue(item, tpdb) returned error: %v", err)
	}
	if fromStash.EntityID != fromTPDB.EntityID {
		t.Fatalf("candidates resolved to two different items: %s vs %s", fromStash.EntityID, fromTPDB.EntityID)
	}

	itemRow, err := d.items.Get(context.Background(), fromStash.EntityID)
	if err != nil {
		t.Fatalf("items.Get returned error: %v", err)
	}
	// Scalars come from the priority-selected primary (stashdb, first in
	// providerPriority) — see TestPersister_Persist_ProviderPriorityControlsScalars
	// for the reverse-priority case.
	if itemRow.Title != "Stash Title" || itemRow.Overview != "Stash overview" {
		t.Errorf("item = %+v, want the stashdb-primary scalars", itemRow)
	}

	// Tags union across both providers, deduplicated case-insensitively
	// (StashDB's "Anal" and ThePornDB's "anal" collapse to one Tag).
	assignments, _, err := d.tagAssignments.List(context.Background(), "", domain.EntityTypeItem, itemRow.ID, 100, "")
	if err != nil {
		t.Fatalf("tagAssignments.List returned error: %v", err)
	}
	if len(assignments) != 3 {
		t.Errorf("len(assignments) = %d, want 3 (Anal, Blonde, Blowjob)", len(assignments))
	}

	// Performers union: both providers' distinct Person rows are credited,
	// no cross-provider merge attempted (see persistPerformers' doc
	// comment).
	credits, _, err := d.itemPeople.List(context.Background(), itemRow.ID, "", 100, "")
	if err != nil {
		t.Fatalf("itemPeople.List returned error: %v", err)
	}
	if len(credits) != 2 {
		t.Fatalf("len(credits) = %d, want 2 (one per provider's performer)", len(credits))
	}
	names := map[string]bool{}
	for _, c := range credits {
		person, err := d.persons.Get(context.Background(), c.PersonID)
		if err != nil {
			t.Fatalf("persons.Get returned error: %v", err)
		}
		names[person.Name] = true
	}
	if !names["Jane Doe"] || !names["Jane Doe TPDB"] {
		t.Errorf("credited performer names = %v, want both Jane Doe and Jane Doe TPDB", names)
	}
}

func TestPersister_Persist_ProviderPriorityControlsScalars(t *testing.T) {
	d := newPersisterDeps(t)
	d.putStashScene("stash-scene-1")
	d.putTPDBScene("tpdb-scene-1")
	// Reversed priority from the default — tpdb wins scalars this time,
	// proving the order is read from configuration, not hardcoded.
	p := d.newPersister([]string{"tpdb", "stashdb"})

	err := p.Persist(context.Background(), nil,
		[]domain.MatchCandidate{stashCandidate("stash-scene-1"), tpdbCandidate("tpdb-scene-1")},
		[]*domain.UnmatchedFile{sceneFile()})
	if err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	linked, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceTPDB, "tpdb-scene-1")
	if err != nil {
		t.Fatalf("GetByValue(item, tpdb) returned error: %v", err)
	}
	itemRow, err := d.items.Get(context.Background(), linked.EntityID)
	if err != nil {
		t.Fatalf("items.Get returned error: %v", err)
	}
	if itemRow.Title != "TPDB Title" || itemRow.Overview != "TPDB overview" {
		t.Errorf("item = %+v, want the tpdb-primary scalars", itemRow)
	}
}

func TestPersister_Persist_JAVCodeLinksAcrossProviderRescans(t *testing.T) {
	d := newPersisterDeps(t)
	d.putTPDBScene("tpdb-scene-jav")
	p := d.newPersister([]string{"stashdb", "tpdb"})

	first := tpdbCandidate("tpdb-scene-jav")
	first.Metadata["jav_code"] = "SSIS-001"
	if err := p.Persist(context.Background(), nil, []domain.MatchCandidate{first}, []*domain.UnmatchedFile{sceneFile()}); err != nil {
		t.Fatalf("first Persist returned error: %v", err)
	}

	javLink, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceJAVCode, "SSIS-001")
	if err != nil {
		t.Fatalf("GetByValue(item, jav_code) returned error: %v", err)
	}

	// A later rescan clears StashDB's independent copy of the same JAV
	// title, under a StashDB scene ID never seen before — the jav_code
	// link (not a shared provider ID, since none exists between StashDB
	// and ThePornDB) is what recognizes this as the same scene.
	d.putStashScene("stash-scene-jav")
	second := stashCandidate("stash-scene-jav")
	second.Metadata["jav_code"] = "SSIS-001"
	if err := p.Persist(context.Background(), nil, []domain.MatchCandidate{second}, nil); err != nil {
		t.Fatalf("second Persist returned error: %v", err)
	}

	fromStash, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceStashDB, "stash-scene-jav")
	if err != nil {
		t.Fatalf("GetByValue(item, stashdb) returned error: %v", err)
	}
	if fromStash.EntityID != javLink.EntityID {
		t.Fatalf("second Persist created a new item (%s) instead of reusing the jav_code-linked one (%s)", fromStash.EntityID, javLink.EntityID)
	}

	itemRow, err := d.items.Get(context.Background(), javLink.EntityID)
	if err != nil {
		t.Fatalf("items.Get returned error: %v", err)
	}
	if itemRow.Metadata["kind"] != "jav_title" {
		t.Errorf(`item.Metadata["kind"] = %v, want "jav_title"`, itemRow.Metadata["kind"])
	}
}

func TestPersister_Persist_RetryDoesNotDuplicateMediaFile(t *testing.T) {
	d := newPersisterDeps(t)
	d.putStashScene("stash-scene-1")
	p := d.newPersister([]string{"stashdb", "tpdb"})
	uf := sceneFile()

	for i := 0; i < 2; i++ {
		if err := p.Persist(context.Background(), nil, []domain.MatchCandidate{stashCandidate("stash-scene-1")}, []*domain.UnmatchedFile{uf}); err != nil {
			t.Fatalf("Persist call %d returned error: %v", i+1, err)
		}
	}

	linked, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceStashDB, "stash-scene-1")
	if err != nil {
		t.Fatalf("GetByValue returned error: %v", err)
	}
	mediaFiles, _, err := d.mediaFiles.List(context.Background(), linked.EntityID, 100, "")
	if err != nil {
		t.Fatalf("mediaFiles.List returned error: %v", err)
	}
	if len(mediaFiles) != 1 {
		t.Fatalf("len(mediaFiles) = %d after retry, want 1 (no duplicate)", len(mediaFiles))
	}
	// The retry finds the MediaFile via GetByHash and only reconciles the
	// Item's Status (ensureItemImported) — Organize is only called from the
	// "create a new MediaFile" branch, so a retry that hits the short-circuit
	// never calls it again, same as music.Persister's persistOneTrack.
	if d.organizer.callCount() != 1 {
		t.Errorf("organizer.callCount() = %d, want 1 (not called again on the idempotent retry)", d.organizer.callCount())
	}
}

func TestPersister_Persist_TPDBAliasPerformerLinksToCanonicalPerson(t *testing.T) {
	d := newPersisterDeps(t)
	d.putTPDBAliasScene("tpdb-scene-alias")
	p := d.newPersister([]string{"stashdb", "tpdb"})

	if err := p.Persist(context.Background(), nil, []domain.MatchCandidate{tpdbCandidate("tpdb-scene-alias")}, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	linked, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypeItem, domain.ExternalIDSourceTPDB, "tpdb-scene-alias")
	if err != nil {
		t.Fatalf("GetByValue(item, tpdb) returned error: %v", err)
	}
	credits, _, err := d.itemPeople.List(context.Background(), linked.EntityID, "", 100, "")
	if err != nil {
		t.Fatalf("itemPeople.List returned error: %v", err)
	}
	if len(credits) != 1 {
		t.Fatalf("len(credits) = %d, want 1", len(credits))
	}
	if credits[0].CreditedAs != "Anikka" {
		t.Errorf("CreditedAs = %q, want the alias's own display name %q", credits[0].CreditedAs, "Anikka")
	}

	// The Person is the canonical parent record, not the alias.
	performer, err := d.persons.Get(context.Background(), credits[0].PersonID)
	if err != nil {
		t.Fatalf("persons.Get returned error: %v", err)
	}
	if performer.Name != "Anikka Albrite" {
		t.Errorf("performer.Name = %q, want the canonical name %q", performer.Name, "Anikka Albrite")
	}
	foundAlias := false
	for _, a := range performer.Aliases {
		if strings.EqualFold(a, "Anikka") {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Errorf("performer.Aliases = %v, want it to include the alias display name %q", performer.Aliases, "Anikka")
	}

	// The Person is resolved via the canonical ID, never the alias's own ID
	// — ExternalID's storage identity is (entityType, entityID, source), so
	// a Person can only carry one tpdb-sourced value (see resolvedPerformer's
	// doc comment). The alias's own ID is deliberately not linked as a
	// second row.
	fromCanonical, err := d.externalIDs.GetByValue(context.Background(), domain.EntityTypePerson, domain.ExternalIDSourceTPDB, "tpdb-canonical-1")
	if err != nil {
		t.Fatalf("GetByValue(person, tpdb-canonical-1) returned error: %v", err)
	}
	if fromCanonical.EntityID != performer.ID {
		t.Errorf("fromCanonical.EntityID = %s, want %s", fromCanonical.EntityID, performer.ID)
	}
}
