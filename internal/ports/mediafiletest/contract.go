// Package mediafiletest is the shared contract test suite for the
// ports.MediaFileRepository port. See internal/ports/persontest for the
// convention this follows.
package mediafiletest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty MediaFileRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.MediaFileRepository

// TestMediaFileRepository runs the shared MediaFileRepository contract
// against newRepo.
func TestMediaFileRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the media file", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing media file", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing media file returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a media file", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing media file returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by item independently", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list returns every created media file across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("get by hash matches on oshash", func(t *testing.T) { testGetByHashOSHash(t, newRepo) })
	t.Run("get by hash matches on sha1", func(t *testing.T) { testGetByHashSHA1(t, newRepo) })
	t.Run("get by hash matches on md5", func(t *testing.T) { testGetByHashMD5(t, newRepo) })
	t.Run("get by hash matches on sha512", func(t *testing.T) { testGetByHashSHA512(t, newRepo) })
	t.Run("get by hash returns ErrNotFound when nothing matches", func(t *testing.T) { testGetByHashNoMatch(t, newRepo) })
	t.Run("get by hash skips empty inputs", func(t *testing.T) { testGetByHashSkipsEmpty(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.MediaFileRepository, m *domain.MediaFile) {
	t.Helper()
	if err := r.Create(context.Background(), m); err != nil {
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
	m := sampleMediaFile("m1")
	mustCreate(t, r, m)

	got, err := r.Get(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Path != m.Path {
		t.Fatalf("Get returned Path %q, want %q", got.Path, m.Path)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFile("m1"))

	err := r.Create(context.Background(), sampleMediaFile("m1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	m := sampleMediaFile("m1")
	mustCreate(t, r, m)

	m.Path = "/media/updated.mkv"
	if err := r.Update(context.Background(), m); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Path != "/media/updated.mkv" {
		t.Fatalf("Get after Update returned Path %q, want %q", got.Path, "/media/updated.mkv")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleMediaFile("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing media file returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFile("m1"))

	if err := r.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "m1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing media file returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleMediaFileForItem("m1", "item1"))
	mustCreate(t, r, sampleMediaFileForItem("m2", "item1"))
	mustCreate(t, r, sampleMediaFileForItem("m3", "item2"))

	byItem, _, err := r.List(ctx, "item1", 10, "")
	if err != nil {
		t.Fatalf("List(by item) returned error: %v", err)
	}
	if len(byItem) != 2 {
		t.Fatalf("List(by item item1) returned %d rows, want 2", len(byItem))
	}

	unfiltered, _, err := r.List(ctx, "", 10, "")
	if err != nil {
		t.Fatalf("List(unfiltered) returned error: %v", err)
	}
	if len(unfiltered) != 3 {
		t.Fatalf("List(unfiltered) returned %d rows, want 3", len(unfiltered))
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("m%d", i)
		mustCreate(t, r, sampleMediaFile(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		files, next, err := r.List(ctx, "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, m := range files {
			got[m.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d media files, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing media file %q", id)
		}
	}
}

func sampleMediaFile(id string) *domain.MediaFile {
	return &domain.MediaFile{ID: id, ItemID: "item1", Path: "/media/test.mkv"}
}

func sampleMediaFileForItem(id, itemID string) *domain.MediaFile {
	m := sampleMediaFile(id)
	m.ItemID = itemID
	return m
}

func sampleMediaFileWithHashes(id string) *domain.MediaFile {
	m := sampleMediaFile(id)
	m.OSHash = "oshash-" + id
	m.SHA1 = "sha1-" + id
	m.MD5 = "md5-" + id
	m.SHA512 = "sha512-" + id
	return m
}

func testGetByHashOSHash(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFileWithHashes("m1"))

	got, err := r.GetByHash(context.Background(), "oshash-m1", "", "", "")
	if err != nil {
		t.Fatalf("GetByHash(oshash) returned error: %v", err)
	}
	if got.ID != "m1" {
		t.Fatalf("GetByHash(oshash) returned ID %q, want %q", got.ID, "m1")
	}
}

func testGetByHashSHA1(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFileWithHashes("m1"))

	got, err := r.GetByHash(context.Background(), "", "sha1-m1", "", "")
	if err != nil {
		t.Fatalf("GetByHash(sha1) returned error: %v", err)
	}
	if got.ID != "m1" {
		t.Fatalf("GetByHash(sha1) returned ID %q, want %q", got.ID, "m1")
	}
}

func testGetByHashMD5(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFileWithHashes("m1"))

	got, err := r.GetByHash(context.Background(), "", "", "md5-m1", "")
	if err != nil {
		t.Fatalf("GetByHash(md5) returned error: %v", err)
	}
	if got.ID != "m1" {
		t.Fatalf("GetByHash(md5) returned ID %q, want %q", got.ID, "m1")
	}
}

func testGetByHashSHA512(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFileWithHashes("m1"))

	got, err := r.GetByHash(context.Background(), "", "", "", "sha512-m1")
	if err != nil {
		t.Fatalf("GetByHash(sha512) returned error: %v", err)
	}
	if got.ID != "m1" {
		t.Fatalf("GetByHash(sha512) returned ID %q, want %q", got.ID, "m1")
	}
}

func testGetByHashNoMatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleMediaFileWithHashes("m1"))

	_, err := r.GetByHash(context.Background(), "no-such-hash", "no-such-hash", "no-such-hash", "no-such-hash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByHash with no match returned %v, want ErrNotFound", err)
	}
}

func testGetByHashSkipsEmpty(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	// A record that has never computed MD5/SHA512 stores them as "".
	mustCreate(t, r, sampleMediaFile("m1"))

	_, err := r.GetByHash(context.Background(), "", "", "", "")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByHash with all-empty inputs returned %v, want ErrNotFound (must not match empty-hash records)", err)
	}
}
