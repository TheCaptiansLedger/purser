// Package unmatchedfiletest is the shared contract test suite for the
// ports.UnmatchedFileRepository port. See internal/ports/mediafiletest for
// the convention this follows.
package unmatchedfiletest

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// NewRepositoryFunc returns a fresh, empty UnmatchedFileRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.UnmatchedFileRepository

// TestUnmatchedFileRepository runs the shared UnmatchedFileRepository
// contract against newRepo.
func TestUnmatchedFileRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the unmatched file", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("list with no filter returns every record", func(t *testing.T) { testListNoFilter(t, newRepo) })
	t.Run("list filtered by status returns only matching records", func(t *testing.T) { testListStatusFilter(t, newRepo) })
	t.Run("list paginates across two calls", func(t *testing.T) { testListPagination(t, newRepo) })
	t.Run("update replaces an existing unmatched file", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing unmatched file returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes the unmatched file", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing unmatched file returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("get by hash matches on oshash", func(t *testing.T) { testGetByHashOSHash(t, newRepo) })
	t.Run("get by hash matches on sha1", func(t *testing.T) { testGetByHashSHA1(t, newRepo) })
	t.Run("get by hash returns ErrNotFound when nothing matches", func(t *testing.T) { testGetByHashNoMatch(t, newRepo) })
	t.Run("get by hash skips empty inputs", func(t *testing.T) { testGetByHashSkipsEmpty(t, newRepo) })
	t.Run("get by hash matches a dismissed record", func(t *testing.T) { testGetByHashMatchesDismissed(t, newRepo) })
	t.Run("list by group key returns every row sharing a group key", func(t *testing.T) { testListByGroupKey(t, newRepo) })
	t.Run("list by group key ignores other groups", func(t *testing.T) { testListByGroupKeyIgnoresOtherGroups(t, newRepo) })
	t.Run("list by group key on an unknown key returns empty, not an error", func(t *testing.T) { testListByGroupKeyUnknown(t, newRepo) })
	t.Run("update batch replaces every record in one call", func(t *testing.T) { testUpdateBatch(t, newRepo) })
	t.Run("update batch is all-or-nothing on a missing id", func(t *testing.T) { testUpdateBatchMissing(t, newRepo) })
	t.Run("track number round-trips a non-numeric value with no coercion", func(t *testing.T) { testTrackNumberNonNumeric(t, newRepo) })
	t.Run("delete batch removes every unmatched file atomically", func(t *testing.T) { testDeleteBatch(t, newRepo) })
	t.Run("delete batch with one missing id rolls back the whole batch", func(t *testing.T) { testDeleteBatchRollsBackOnMissing(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.UnmatchedFileRepository, u *domain.UnmatchedFile) {
	t.Helper()
	if err := r.Create(context.Background(), u); err != nil {
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
	u := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u)

	got, err := r.Get(context.Background(), "uf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Path != u.Path {
		t.Fatalf("Get returned Path %q, want %q", got.Path, u.Path)
	}
	if got.OSHash != u.OSHash {
		t.Fatalf("Get returned OSHash %q, want %q", got.OSHash, u.OSHash)
	}
	if got.Status != u.Status {
		t.Fatalf("Get returned Status %q, want %q", got.Status, u.Status)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	err := r.Create(context.Background(), sampleUnmatchedFile("uf1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testListNoFilter(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	want := map[string]bool{"uf1": false, "uf2": false, "uf3": false}
	for id := range want {
		mustCreate(t, r, sampleUnmatchedFile(id))
	}

	got, next, err := r.List(context.Background(), "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if next != "" {
		t.Fatalf("List with page_size >= total returned next_page_token %q, want empty", next)
	}
	if len(got) != len(want) {
		t.Fatalf("List returned %d records, want %d", len(got), len(want))
	}
	for _, u := range got {
		want[u.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Fatalf("List did not include %q", id)
		}
	}
}

func testListStatusFilter(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFileWithStatus("uf-pending", domain.UnmatchedFileStatusPending))
	mustCreate(t, r, sampleUnmatchedFileWithStatus("uf-matched", domain.UnmatchedFileStatusMatched))
	mustCreate(t, r, sampleUnmatchedFileWithStatus("uf-dismissed", domain.UnmatchedFileStatusDismissed))

	got, _, err := r.List(context.Background(), domain.UnmatchedFileStatusMatched, 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "uf-matched" {
		t.Fatalf("List(status=matched) returned %v, want exactly [uf-matched]", got)
	}
}

func testListPagination(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	want := map[string]bool{"uf1": false, "uf2": false, "uf3": false, "uf4": false, "uf5": false}
	for id := range want {
		mustCreate(t, r, sampleUnmatchedFile(id))
	}

	var got []*domain.UnmatchedFile
	token := ""
	for range len(want) {
		page, next, err := r.List(context.Background(), "", 2, token)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		got = append(got, page...)
		if next == "" {
			break
		}
		token = next
	}
	if len(got) != len(want) {
		t.Fatalf("List paginated to %d records, want %d", len(got), len(want))
	}
	for _, u := range got {
		if want[u.ID] {
			t.Fatalf("List returned %q more than once across pages", u.ID)
		}
		want[u.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Fatalf("List paginated result did not include %q", id)
		}
	}
}

func sampleUnmatchedFile(id string) *domain.UnmatchedFile {
	return sampleUnmatchedFileWithStatus(id, domain.UnmatchedFileStatusPending)
}

func sampleUnmatchedFileWithStatus(id string, status domain.UnmatchedFileStatus) *domain.UnmatchedFile {
	path := "/media/incoming/" + id + ".flac"
	return &domain.UnmatchedFile{
		ID:           id,
		Path:         path,
		ContentType:  domain.ContentTypeMusic,
		GroupKey:     path,
		Size:         123456,
		OSHash:       "0123456789abcdef",
		SHA1:         "a9993e364706816aba3e25717850c26c9cd0d89d",
		DiscoveredAt: time.Unix(1700000000, 0).UTC(),
		Status:       status,
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u)

	u.Path = "/media/incoming/moved.flac"
	if err := r.Update(context.Background(), u); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "uf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Path != "/media/incoming/moved.flac" {
		t.Fatalf("Get after Update returned Path %q, want %q", got.Path, "/media/incoming/moved.flac")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleUnmatchedFile("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing unmatched file returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	if err := r.Delete(context.Background(), "uf1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "uf1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing unmatched file returned %v, want ErrNotFound", err)
	}
}

func testGetByHashOSHash(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u)

	got, err := r.GetByHash(context.Background(), u.OSHash, "", "", "")
	if err != nil {
		t.Fatalf("GetByHash(oshash) returned error: %v", err)
	}
	if got.ID != "uf1" {
		t.Fatalf("GetByHash(oshash) returned ID %q, want %q", got.ID, "uf1")
	}
}

func testGetByHashSHA1(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u)

	got, err := r.GetByHash(context.Background(), "", u.SHA1, "", "")
	if err != nil {
		t.Fatalf("GetByHash(sha1) returned error: %v", err)
	}
	if got.ID != "uf1" {
		t.Fatalf("GetByHash(sha1) returned ID %q, want %q", got.ID, "uf1")
	}
}

func testGetByHashNoMatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	_, err := r.GetByHash(context.Background(), "no-such-hash", "no-such-hash", "no-such-hash", "no-such-hash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByHash with no match returned %v, want ErrNotFound", err)
	}
}

func testGetByHashSkipsEmpty(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	// A record that has never computed MD5/SHA512 stores them as "".
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	_, err := r.GetByHash(context.Background(), "", "", "", "")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByHash with all-empty inputs returned %v, want ErrNotFound (must not match empty-hash records)", err)
	}
}

// testGetByHashMatchesDismissed proves docs/adr/0024-pipeline-core.md's
// "already known" short-circuit stays correct for a dismissed record:
// GetByHash is a pure hash lookup with no implicit status filter, so a
// dismissed file that moved on disk is still found (and just has its Path
// updated) rather than being silently re-queued by the next rescan.
func testGetByHashMatchesDismissed(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u := sampleUnmatchedFileWithStatus("uf1", domain.UnmatchedFileStatusDismissed)
	mustCreate(t, r, u)

	got, err := r.GetByHash(context.Background(), u.OSHash, "", "", "")
	if err != nil {
		t.Fatalf("GetByHash(oshash) on a dismissed record returned error: %v", err)
	}
	if got.ID != "uf1" || got.Status != domain.UnmatchedFileStatusDismissed {
		t.Fatalf("GetByHash(oshash) returned %+v, want dismissed record uf1", got)
	}
}

// sampleGroupedUnmatchedFile builds a record that shares groupKey with its
// siblings — unlike sampleUnmatchedFile, whose GroupKey always equals its
// own Path (a group of one).
func sampleGroupedUnmatchedFile(id, groupKey string) *domain.UnmatchedFile {
	u := sampleUnmatchedFile(id)
	u.GroupKey = groupKey
	return u
}

func testListByGroupKey(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleGroupedUnmatchedFile("uf1", "album-1"))
	mustCreate(t, r, sampleGroupedUnmatchedFile("uf2", "album-1"))
	mustCreate(t, r, sampleGroupedUnmatchedFile("uf3", "album-1"))

	got, err := r.ListByGroupKey(context.Background(), "album-1")
	if err != nil {
		t.Fatalf("ListByGroupKey returned error: %v", err)
	}
	want := map[string]bool{"uf1": false, "uf2": false, "uf3": false}
	if len(got) != len(want) {
		t.Fatalf("ListByGroupKey returned %d records, want %d", len(got), len(want))
	}
	for _, u := range got {
		want[u.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Fatalf("ListByGroupKey did not include %q", id)
		}
	}
}

func testListByGroupKeyIgnoresOtherGroups(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleGroupedUnmatchedFile("uf1", "album-1"))
	mustCreate(t, r, sampleGroupedUnmatchedFile("uf2", "album-2"))

	got, err := r.ListByGroupKey(context.Background(), "album-1")
	if err != nil {
		t.Fatalf("ListByGroupKey returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "uf1" {
		t.Fatalf("ListByGroupKey(album-1) returned %v, want exactly [uf1]", got)
	}
}

func testListByGroupKeyUnknown(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	got, err := r.ListByGroupKey(context.Background(), "no-such-group")
	if err != nil {
		t.Fatalf("ListByGroupKey on an unknown key returned error: %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListByGroupKey on an unknown key returned %v, want empty", got)
	}
}

func testUpdateBatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u1 := sampleGroupedUnmatchedFile("uf1", "album-1")
	u2 := sampleGroupedUnmatchedFile("uf2", "album-1")
	mustCreate(t, r, u1)
	mustCreate(t, r, u2)

	u1.Status = domain.UnmatchedFileStatusDismissed
	u2.Status = domain.UnmatchedFileStatusDismissed
	if err := r.UpdateBatch(context.Background(), []*domain.UnmatchedFile{u1, u2}); err != nil {
		t.Fatalf("UpdateBatch returned error: %v", err)
	}

	for _, id := range []string{"uf1", "uf2"} {
		got, err := r.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get(%q) after UpdateBatch returned error: %v", id, err)
		}
		if got.Status != domain.UnmatchedFileStatusDismissed {
			t.Fatalf("Get(%q) after UpdateBatch returned Status %q, want dismissed", id, got.Status)
		}
	}
}

// testUpdateBatchMissing proves docs/adr/0016-bulk-operations.md's
// all-or-nothing default: one bad id in the batch fails the whole call,
// leaving the valid record's prior state untouched.
func testUpdateBatchMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u1 := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u1)

	u1.Status = domain.UnmatchedFileStatusDismissed
	missing := sampleUnmatchedFile("missing")
	err := r.UpdateBatch(context.Background(), []*domain.UnmatchedFile{u1, missing})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("UpdateBatch with a missing id returned %v, want ErrNotFound", err)
	}

	got, getErr := r.Get(context.Background(), "uf1")
	if getErr != nil {
		t.Fatalf("Get(uf1) after a failed UpdateBatch returned error: %v", getErr)
	}
	if got.Status == domain.UnmatchedFileStatusDismissed {
		t.Fatal("UpdateBatch applied a partial update despite one id failing — all-or-nothing was violated")
	}
}

// testTrackNumberNonNumeric proves TrackNumber round-trips a vinyl
// side-lettered value with no coercion or error — the whole reason it's a
// string, not an int (matching Item.Sequence's convention).
func testTrackNumberNonNumeric(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	u := sampleUnmatchedFile("uf1")
	u.DiscNumber = 2
	u.TrackNumber = "A1"
	mustCreate(t, r, u)

	got, err := r.Get(context.Background(), "uf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.TrackNumber != "A1" {
		t.Fatalf("Get returned TrackNumber %q, want %q (no coercion)", got.TrackNumber, "A1")
	}
	if got.DiscNumber != 2 {
		t.Fatalf("Get returned DiscNumber %d, want %d", got.DiscNumber, 2)
	}
}

func testDeleteBatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))
	mustCreate(t, r, sampleUnmatchedFile("uf2"))
	mustCreate(t, r, sampleUnmatchedFile("uf3"))

	if err := r.DeleteBatch(context.Background(), []string{"uf1", "uf2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := r.Get(context.Background(), "uf1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for uf1 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "uf2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for uf2 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "uf3"); err != nil {
		t.Fatalf("DeleteBatch removed uf3, which wasn't in the batch: %v", err)
	}
}

func testDeleteBatchRollsBackOnMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	err := r.DeleteBatch(context.Background(), []string{"uf1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := r.Get(context.Background(), "uf1"); err != nil {
		t.Fatalf("DeleteBatch removed uf1 despite rolling back: %v", err)
	}
}
