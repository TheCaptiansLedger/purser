// Package musicreleasetest is the shared contract test suite for the
// ports.MusicReleaseRepository port. See internal/ports/grouptest for the
// convention this follows.
package musicreleasetest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty MusicReleaseRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.MusicReleaseRepository

// TestMusicReleaseRepository runs the shared MusicReleaseRepository
// contract against newRepo.
func TestMusicReleaseRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the release", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing release", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing release returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a release", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing release returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created release across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("GetByMBID returns the release with that MBID", func(t *testing.T) { testGetByMBID(t, newRepo) })
	t.Run("GetByMBID on an unknown MBID returns ErrNotFound", func(t *testing.T) { testGetByMBIDNotFound(t, newRepo) })
	t.Run("GetByBarcode returns the release with that barcode", func(t *testing.T) { testGetByBarcode(t, newRepo) })
	t.Run("GetByBarcode on an unknown barcode returns ErrNotFound", func(t *testing.T) { testGetByBarcodeNotFound(t, newRepo) })
	t.Run("ListByGroup returns only releases in that Group", func(t *testing.T) { testListByGroup(t, newRepo) })
	t.Run("ListByEntry returns only releases under that LibraryEntry", func(t *testing.T) { testListByEntry(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.MusicReleaseRepository, rel *music.Release) {
	t.Helper()
	if err := r.Create(context.Background(), rel); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	rel := sampleRelease("r1")
	mustCreate(t, r, rel)

	got, err := r.Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != rel.Title {
		t.Fatalf("Get returned Title %q, want %q", got.Title, rel.Title)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleRelease("r1"))

	err := r.Create(context.Background(), sampleRelease("r1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	rel := sampleRelease("r1")
	mustCreate(t, r, rel)

	rel.Title = "Updated Title"
	if err := r.Update(context.Background(), rel); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated Title")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleRelease("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing release returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleRelease("r1"))

	if err := r.Delete(context.Background(), "r1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "r1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing release returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("r%d", i)
		mustCreate(t, r, sampleRelease(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		releases, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, rel := range releases {
			got[rel.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d releases, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing release %q", id)
		}
	}
}

func sampleRelease(id string) *music.Release {
	return &music.Release{
		ID:             id,
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Test Release",
		Status:         music.ReleaseStatusStub,
	}
}

// sampleReleaseWithFields builds a release with explicit GroupID,
// LibraryEntryID, MBID, and Barcode — used by the point-lookup and
// filtered-List tests, which need distinct values across fixtures.
func sampleReleaseWithFields(id, groupID, libraryEntryID, mbid, barcode string) *music.Release {
	rel := sampleRelease(id)
	rel.GroupID = groupID
	rel.LibraryEntryID = libraryEntryID
	rel.MBID = mbid
	rel.Barcode = barcode
	return rel
}

func testGetByMBID(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleReleaseWithFields("r1", "group1", "entry1", "mbid-1", "barcode-1"))
	mustCreate(t, r, sampleReleaseWithFields("r2", "group1", "entry1", "mbid-2", "barcode-2"))

	got, err := r.GetByMBID(context.Background(), "mbid-2")
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if got.ID != "r2" {
		t.Fatalf("GetByMBID returned release %q, want %q", got.ID, "r2")
	}
}

func testGetByMBIDNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleReleaseWithFields("r1", "group1", "entry1", "mbid-1", "barcode-1"))

	_, err := r.GetByMBID(context.Background(), "missing-mbid")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByMBID on unknown MBID returned %v, want ErrNotFound", err)
	}
}

func testGetByBarcode(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleReleaseWithFields("r1", "group1", "entry1", "mbid-1", "barcode-1"))
	mustCreate(t, r, sampleReleaseWithFields("r2", "group1", "entry1", "mbid-2", "barcode-2"))

	got, err := r.GetByBarcode(context.Background(), "barcode-2")
	if err != nil {
		t.Fatalf("GetByBarcode returned error: %v", err)
	}
	if got.ID != "r2" {
		t.Fatalf("GetByBarcode returned release %q, want %q", got.ID, "r2")
	}
}

func testGetByBarcodeNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleReleaseWithFields("r1", "group1", "entry1", "mbid-1", "barcode-1"))

	_, err := r.GetByBarcode(context.Background(), "missing-barcode")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByBarcode on unknown barcode returned %v, want ErrNotFound", err)
	}
}

func testListByGroup(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	mustCreate(t, r, sampleReleaseWithFields("r1", "groupA", "entry1", "mbid-a1", "barcode-a1"))
	mustCreate(t, r, sampleReleaseWithFields("r2", "groupA", "entry1", "mbid-a2", "barcode-a2"))
	mustCreate(t, r, sampleReleaseWithFields("r3", "groupB", "entry1", "mbid-b1", "barcode-b1"))

	releases, _, err := r.ListByGroup(ctx, "groupA", 10, "")
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	got := map[string]bool{}
	for _, rel := range releases {
		got[rel.ID] = true
	}
	if len(got) != 2 || !got["r1"] || !got["r2"] {
		t.Fatalf("ListByGroup(groupA) returned %v, want exactly {r1, r2}", got)
	}
}

func testListByEntry(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	mustCreate(t, r, sampleReleaseWithFields("r1", "group1", "entryA", "mbid-a1", "barcode-a1"))
	mustCreate(t, r, sampleReleaseWithFields("r2", "group2", "entryA", "mbid-a2", "barcode-a2"))
	mustCreate(t, r, sampleReleaseWithFields("r3", "group3", "entryB", "mbid-b1", "barcode-b1"))

	releases, _, err := r.ListByEntry(ctx, "entryA", 10, "")
	if err != nil {
		t.Fatalf("ListByEntry returned error: %v", err)
	}
	got := map[string]bool{}
	for _, rel := range releases {
		got[rel.ID] = true
	}
	if len(got) != 2 || !got["r1"] || !got["r2"] {
		t.Fatalf("ListByEntry(entryA) returned %v, want exactly {r1, r2}", got)
	}
}
