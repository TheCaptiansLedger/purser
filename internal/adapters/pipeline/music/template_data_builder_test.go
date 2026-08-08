package music_test

import (
	"context"
	"errors"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/adapters/store/group"
	"purser/internal/adapters/store/libraryentry"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"

	dsbadger "purser/internal/adapters/datastore/badger"

	storemusic "purser/internal/adapters/store/music"

	musicdomain "purser/internal/domain/music"
)

// templateDataBuilderDeps bundles a TemplateDataBuilder backed by real
// Badger-backed repositories (GroupRepository/MusicReleaseRepository/
// LibraryEntryRepository), the same real-storage convention
// persisterDeps uses, since Item.Metadata's int/float64 JSON round-trip
// (discNumberOf) needs a real Datastore to exercise honestly.
type templateDataBuilderDeps struct {
	groups         ports.GroupRepository
	releases       ports.MusicReleaseRepository
	libraryEntries ports.LibraryEntryRepository
	builder        *music.TemplateDataBuilder
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

	groups, err := group.New("test", ds)
	if err != nil {
		t.Fatalf("group.New returned error: %v", err)
	}
	releases, err := storemusic.New("test", ds)
	if err != nil {
		t.Fatalf("storemusic.New returned error: %v", err)
	}
	libraryEntries, err := libraryentry.New("test", ds)
	if err != nil {
		t.Fatalf("libraryentry.New returned error: %v", err)
	}

	return &templateDataBuilderDeps{
		groups: groups, releases: releases, libraryEntries: libraryEntries,
		builder: music.NewTemplateDataBuilder(groups, releases, libraryEntries),
	}
}

// seedFixture creates a LibraryEntry(Artist) -> Group(Release Group) ->
// music.Release chain, returning the built Item a track for it would
// carry (GroupID + Metadata["release_id"] set, matching
// Persister.buildTrackItem's own shape).
func (d *templateDataBuilderDeps) seedFixture(t *testing.T, releaseDate *time.Time, mediumCount int) *domain.Item {
	t.Helper()
	ctx := context.Background()

	artist := &domain.LibraryEntry{
		ID: domain.NewID(), ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist,
		Name: "REO Speedwagon", MonitorMode: domain.MonitorModeAll,
	}
	if err := d.libraryEntries.Create(ctx, artist); err != nil {
		t.Fatalf("creating fixture artist: %v", err)
	}

	grp := &domain.Group{
		ID: domain.NewID(), LibraryEntryID: artist.ID, Title: "Hi Infidelity", MonitorMode: domain.MonitorModeAll,
	}
	if err := d.groups.Create(ctx, grp); err != nil {
		t.Fatalf("creating fixture group: %v", err)
	}

	release := &musicdomain.Release{
		ID: domain.NewID(), GroupID: grp.ID, LibraryEntryID: artist.ID, Title: "Hi Infidelity",
		Date: releaseDate, MediumCount: mediumCount, Status: musicdomain.ReleaseStatusPartial,
	}
	if err := d.releases.Create(ctx, release); err != nil {
		t.Fatalf("creating fixture release: %v", err)
	}

	return &domain.Item{
		ID: domain.NewID(), ContentType: domain.ContentTypeMusic, LibraryEntryID: artist.ID,
		GroupID: grp.ID, Title: "Keep on Loving You", Sequence: "2", Status: domain.ItemStatusImported,
		Metadata: map[string]any{"release_id": release.ID, "disc_number": 1},
	}
}

func TestTemplateDataBuilder_ContentTypes(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	got := deps.builder.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestTemplateDataBuilder_BuildTemplateData_FullMapping(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	releaseDate := time.Date(1980, time.November, 21, 0, 0, 0, 0, time.UTC)
	item := deps.seedFixture(t, &releaseDate, 2)

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}

	want := map[string]any{
		"TrackTitle": "Keep on Loving You", "TrackNumber": 2, "DiscNumber": 1,
		"AlbumTitle": "Hi Infidelity", "Year": 1980, "DiscCount": 2, "ArtistName": "REO Speedwagon",
	}
	for k, v := range want {
		if data[k] != v {
			t.Errorf("data[%q] = %v, want %v", k, data[k], v)
		}
	}
}

func TestTemplateDataBuilder_BuildTemplateData_NoReleaseDateOmitsYear(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	item := deps.seedFixture(t, nil, 1)

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if data["Year"] != 0 {
		t.Errorf("data[\"Year\"] = %v, want 0 for a release with no Date", data["Year"])
	}
}

func TestTemplateDataBuilder_BuildTemplateData_NonNumericSequenceFallsBackToZero(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	item := deps.seedFixture(t, nil, 1)
	item.Sequence = "A1"

	data, err := deps.builder.BuildTemplateData(context.Background(), item)
	if err != nil {
		t.Fatalf("BuildTemplateData returned error: %v", err)
	}
	if data["TrackNumber"] != 0 {
		t.Errorf("data[\"TrackNumber\"] = %v, want 0 for a non-numeric Sequence", data["TrackNumber"])
	}
}

func TestTemplateDataBuilder_BuildTemplateData_MissingGroupIsError(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	item := &domain.Item{
		ID: domain.NewID(), ContentType: domain.ContentTypeMusic, LibraryEntryID: "whoever",
		GroupID: "no-such-group", Title: "Orphan Track", Status: domain.ItemStatusImported,
	}

	if _, err := deps.builder.BuildTemplateData(context.Background(), item); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("BuildTemplateData returned %v, want ErrNotFound for a missing Group", err)
	}
}

func TestTemplateDataBuilder_BuildTemplateData_MissingReleaseIsError(t *testing.T) {
	deps := newTemplateDataBuilderDeps(t)
	ctx := context.Background()
	artist := &domain.LibraryEntry{ID: domain.NewID(), ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "Someone", MonitorMode: domain.MonitorModeAll}
	if err := deps.libraryEntries.Create(ctx, artist); err != nil {
		t.Fatalf("creating fixture artist: %v", err)
	}
	grp := &domain.Group{ID: domain.NewID(), LibraryEntryID: artist.ID, Title: "Untethered", MonitorMode: domain.MonitorModeAll}
	if err := deps.groups.Create(ctx, grp); err != nil {
		t.Fatalf("creating fixture group: %v", err)
	}

	item := &domain.Item{
		ID: domain.NewID(), ContentType: domain.ContentTypeMusic, LibraryEntryID: artist.ID,
		GroupID: grp.ID, Title: "Track", Status: domain.ItemStatusImported,
		Metadata: map[string]any{"release_id": "no-such-release"},
	}

	if _, err := deps.builder.BuildTemplateData(ctx, item); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("BuildTemplateData returned %v, want ErrNotFound for a missing MusicRelease", err)
	}
}
