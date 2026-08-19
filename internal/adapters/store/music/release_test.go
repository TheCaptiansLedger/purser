package music_test

import (
	"context"
	"errors"
	"path/filepath"
	"purser/internal/adapters/datastore"
	storeitem "purser/internal/adapters/store/item"
	storemusic "purser/internal/adapters/store/music"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"purser/internal/ports/musicreleasetest"
	"purser/internal/service"
	"testing"

	dsbadger "purser/internal/adapters/datastore/badger"
	dssql "purser/internal/adapters/datastore/sql"
)

func TestRepository_MusicReleaseRepositoryContract_Badger(t *testing.T) {
	musicreleasetest.TestMusicReleaseRepository(t, func(t *testing.T) ports.MusicReleaseRepository {
		return newRepository(t, newBadgerDatastore(t))
	})
}

func TestRepository_MusicReleaseRepositoryContract_SQL(t *testing.T) {
	musicreleasetest.TestMusicReleaseRepository(t, func(t *testing.T) ports.MusicReleaseRepository {
		return newRepository(t, newSQLDatastore(t))
	})
}

// TestRepository_ListTracksByRelease exercises ListTracksByRelease against
// real Item documents created through internal/adapters/store/item — the
// same code path ports.ItemRepository.Create uses in production — rather
// than the shared musicreleasetest contract suite, since it needs the
// "item" collection populated on the very same datastore.Datastore the
// MusicReleaseRepository under test reads from. See
// docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage" section.
func TestRepository_ListTracksByRelease_Badger(t *testing.T) {
	testListTracksByRelease(t, newBadgerDatastore(t))
}

func TestRepository_ListTracksByRelease_SQL(t *testing.T) {
	testListTracksByRelease(t, newSQLDatastore(t))
}

func testListTracksByRelease(t *testing.T, ds datastore.Datastore) {
	t.Helper()
	releases := newRepository(t, ds)
	items := newItemRepository(t, ds)
	ctx := context.Background()

	// Two releases (editions) of the same Release Group — proves the
	// group_id pre-filter's in-memory refine step actually narrows to one
	// release, not every release in the group.
	relA := &music.Release{ID: "relA", GroupID: "group1", LibraryEntryID: "entry1", Title: "Release A", Status: music.ReleaseStatusStub}
	relB := &music.Release{ID: "relB", GroupID: "group1", LibraryEntryID: "entry1", Title: "Release B", Status: music.ReleaseStatusStub}
	mustCreateRelease(t, releases, relA)
	mustCreateRelease(t, releases, relB)

	trackA := &domain.Item{ID: "trackA", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group1", Title: "Track A", Status: domain.ItemStatusImported, Metadata: map[string]any{"release_id": "relA"}}
	trackB := &domain.Item{ID: "trackB", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group1", Title: "Track B", Status: domain.ItemStatusImported, Metadata: map[string]any{"release_id": "relB"}}
	// An item in a different Release Group entirely — proves the group_id
	// pre-filter itself isn't leaking across groups.
	trackOther := &domain.Item{ID: "trackOther", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group2", Title: "Track Other", Status: domain.ItemStatusImported, Metadata: map[string]any{"release_id": "relOther"}}
	mustCreateItem(t, items, trackA)
	mustCreateItem(t, items, trackB)
	mustCreateItem(t, items, trackOther)

	got, _, err := releases.ListTracksByRelease(ctx, "relA", 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease(relA) returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "trackA" {
		t.Fatalf("ListTracksByRelease(relA) returned %v, want exactly trackA", got)
	}

	got, _, err = releases.ListTracksByRelease(ctx, "relB", 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease(relB) returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "trackB" {
		t.Fatalf("ListTracksByRelease(relB) returned %v, want exactly trackB", got)
	}

	if _, _, err := releases.ListTracksByRelease(ctx, "missing", 10, ""); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ListTracksByRelease(missing) returned %v, want ErrNotFound", err)
	}
}

// TestRepository_CreateTrack exercises CreateTrack against both backends —
// the created track must round-trip through ListTracksByRelease and (since
// CreateTrack writes directly into the shared "item" collection, not
// through ports.ItemRepository) must also be visible to
// ports.ItemRepository.List, proving its Document.Index entry really does
// match internal/adapters/store/item's own indexOf shape.
func TestRepository_CreateTrack_Badger(t *testing.T) {
	testCreateTrack(t, newBadgerDatastore(t))
}

func TestRepository_CreateTrack_SQL(t *testing.T) {
	testCreateTrack(t, newSQLDatastore(t))
}

func testCreateTrack(t *testing.T, ds datastore.Datastore) {
	t.Helper()
	releases := newRepository(t, ds)
	items := newItemRepository(t, ds)
	ctx := context.Background()

	rel := &music.Release{ID: "rel1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Release", Status: music.ReleaseStatusStub}
	mustCreateRelease(t, releases, rel)

	track := &domain.Item{ID: "track1", ContentType: domain.ContentTypeMusic, Title: "Track 1", Sequence: "1", Status: domain.ItemStatusMissing}
	if err := releases.CreateTrack(ctx, "rel1", track); err != nil {
		t.Fatalf("CreateTrack returned error: %v", err)
	}
	if track.GroupID != "group1" || track.LibraryEntryID != "entry1" {
		t.Fatalf("CreateTrack left GroupID=%q LibraryEntryID=%q, want group1/entry1", track.GroupID, track.LibraryEntryID)
	}
	if got := track.Metadata["release_id"]; got != "rel1" {
		t.Fatalf("CreateTrack left Metadata[release_id]=%v, want rel1", got)
	}

	got, _, err := releases.ListTracksByRelease(ctx, "rel1", 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "track1" {
		t.Fatalf("ListTracksByRelease returned %v, want exactly track1", got)
	}

	// Proves the raw itemCollection write's Document.Index matches
	// internal/adapters/store/item's own indexOf — a drift here would make
	// tracks created through this path invisible to ItemRepository.List.
	viaItemRepo, _, err := items.List(ctx, "entry1", string(domain.ContentTypeMusic), "group1", domain.ItemStatusMissing, 10, "")
	if err != nil {
		t.Fatalf("ItemRepository.List returned error: %v", err)
	}
	if len(viaItemRepo) != 1 || viaItemRepo[0].ID != "track1" {
		t.Fatalf("ItemRepository.List returned %v, want exactly track1", viaItemRepo)
	}

	if err := releases.CreateTrack(ctx, "missing", &domain.Item{ID: "track2", ContentType: domain.ContentTypeMusic, Title: "Track 2", Status: domain.ItemStatusMissing}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("CreateTrack(missing release) returned %v, want ErrNotFound", err)
	}
}

// TestMusicReleaseDeletionService_Delete_DetachesCreateTrackTrack exercises
// internal/service.MusicReleaseDeletionService.Delete — the same
// composing-deletion code GroupDeletionService's own cascade delegates to
// for its per-release Unlink-delete — against real store adapters and a
// track created through CreateTrack, not internal/adapters/pipeline/music's
// buildTrackItem. Per docs/adr/0021-music-domain-model.md's Track ↔
// Release linkage section and #721's own acceptance criteria, a
// manually-/MusicBrainz-populated track must be indistinguishable in shape
// from a pipeline-scanned one — this proves the deletion cascade can't
// tell the difference.
func TestMusicReleaseDeletionService_Delete_DetachesCreateTrackTrack(t *testing.T) {
	ds := newBadgerDatastore(t)
	releases := newRepository(t, ds)
	items := newItemRepository(t, ds)
	ctx := context.Background()

	rel := &music.Release{ID: "rel1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Release", Status: music.ReleaseStatusStub}
	mustCreateRelease(t, releases, rel)

	track := &domain.Item{ID: "track1", ContentType: domain.ContentTypeMusic, Title: "Track 1", Sequence: "1", Status: domain.ItemStatusMissing}
	if err := releases.CreateTrack(ctx, "rel1", track); err != nil {
		t.Fatalf("CreateTrack returned error: %v", err)
	}

	deletion := service.NewMusicReleaseDeletionService(releases, items)
	if err := deletion.Delete(ctx, "rel1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	got, err := items.Get(ctx, "track1")
	if err != nil {
		t.Fatalf("items.Get(track1) returned error: %v", err)
	}
	if _, stillLinked := got.Metadata["release_id"]; stillLinked {
		t.Fatalf("track1.Metadata[release_id] still set after Delete, want cleared (Unlink)")
	}

	if _, err := releases.Get(ctx, "rel1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("releases.Get(rel1) after Delete returned %v, want ErrNotFound", err)
	}
}

func mustCreateRelease(t *testing.T, repo ports.MusicReleaseRepository, r *music.Release) {
	t.Helper()
	if err := repo.Create(context.Background(), r); err != nil {
		t.Fatalf("Create(%q) returned error: %v", r.ID, err)
	}
}

func mustCreateItem(t *testing.T, repo ports.ItemRepository, i *domain.Item) {
	t.Helper()
	if err := repo.Create(context.Background(), i); err != nil {
		t.Fatalf("Create(%q) returned error: %v", i.ID, err)
	}
}

func newItemRepository(t *testing.T, ds datastore.Datastore) ports.ItemRepository {
	t.Helper()
	repo, err := storeitem.New("test-item", ds)
	if err != nil {
		t.Fatalf("item.New returned error: %v", err)
	}
	return repo
}

func newRepository(t *testing.T, ds datastore.Datastore) ports.MusicReleaseRepository {
	t.Helper()
	repo, err := storemusic.New("test", ds)
	if err != nil {
		t.Fatalf("music.New returned error: %v", err)
	}
	return repo
}

func newBadgerDatastore(t *testing.T) datastore.Datastore {
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

	store, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}
	return store
}

func newSQLDatastore(t *testing.T) datastore.Datastore {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := dssql.Open(dssql.Options{Dialect: dssql.DialectSQLite, DSN: dsn})
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	store, err := dssql.New("test", db, dssql.DialectSQLite)
	if err != nil {
		t.Fatalf("sql.New returned error: %v", err)
	}
	return store
}
