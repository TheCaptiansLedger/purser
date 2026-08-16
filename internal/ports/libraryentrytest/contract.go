// Package libraryentrytest is the shared contract test suite for the
// ports.LibraryEntryRepository port. See internal/ports/persontest for the
// convention this follows.
package libraryentrytest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty LibraryEntryRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.LibraryEntryRepository

// TestLibraryEntryRepository runs the shared LibraryEntryRepository
// contract against newRepo.
func TestLibraryEntryRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the entry", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing entry", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing entry returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes an entry", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing entry returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by kind and parent independently", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list returns every created entry across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("delete batch removes every entry atomically", func(t *testing.T) { testDeleteBatch(t, newRepo) })
	t.Run("delete batch with one missing id rolls back the whole batch", func(t *testing.T) { testDeleteBatchRollsBackOnMissing(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.LibraryEntryRepository, e *domain.LibraryEntry) {
	t.Helper()
	if err := r.Create(context.Background(), e); err != nil {
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
	e := sampleLibraryEntry("e1")
	mustCreate(t, r, e)

	got, err := r.Get(context.Background(), "e1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != e.Name {
		t.Fatalf("Get returned Name %q, want %q", got.Name, e.Name)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleLibraryEntry("e1"))

	err := r.Create(context.Background(), sampleLibraryEntry("e1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	e := sampleLibraryEntry("e1")
	mustCreate(t, r, e)

	e.Name = "Updated Name"
	if err := r.Update(context.Background(), e); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "e1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Fatalf("Get after Update returned Name %q, want %q", got.Name, "Updated Name")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleLibraryEntry("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing entry returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleLibraryEntry("e1"))

	if err := r.Delete(context.Background(), "e1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "e1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing entry returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleLibraryEntryWithParent("n1", domain.KindNetwork, ""))
	mustCreate(t, r, sampleLibraryEntryWithParent("s1", domain.KindStudio, "n1"))
	mustCreate(t, r, sampleLibraryEntryWithParent("s2", domain.KindStudio, "n1"))

	byKind, _, err := r.List(ctx, domain.KindStudio, "", 10, "")
	if err != nil {
		t.Fatalf("List(by kind) returned error: %v", err)
	}
	if len(byKind) != 2 {
		t.Fatalf("List(by kind studio) returned %d rows, want 2", len(byKind))
	}

	byParent, _, err := r.List(ctx, "", "n1", 10, "")
	if err != nil {
		t.Fatalf("List(by parent) returned error: %v", err)
	}
	if len(byParent) != 2 {
		t.Fatalf("List(by parent n1) returned %d rows, want 2", len(byParent))
	}

	byBoth, _, err := r.List(ctx, domain.KindNetwork, "", 10, "")
	if err != nil {
		t.Fatalf("List(by kind network) returned error: %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].ID != "n1" {
		t.Fatalf("List(by kind network) returned %v, want [n1]", byBoth)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("e%d", i)
		mustCreate(t, r, sampleLibraryEntry(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		entries, next, err := r.List(ctx, "", "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, e := range entries {
			got[e.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d entries, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing entry %q", id)
		}
	}
}

func testDeleteBatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleLibraryEntry("e1"))
	mustCreate(t, r, sampleLibraryEntry("e2"))
	mustCreate(t, r, sampleLibraryEntry("e3"))

	if err := r.DeleteBatch(context.Background(), []string{"e1", "e2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := r.Get(context.Background(), "e1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for e1 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "e2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for e2 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "e3"); err != nil {
		t.Fatalf("DeleteBatch removed e3, which wasn't in the batch: %v", err)
	}
}

func testDeleteBatchRollsBackOnMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleLibraryEntry("e1"))

	err := r.DeleteBatch(context.Background(), []string{"e1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := r.Get(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteBatch removed e1 despite rolling back: %v", err)
	}
}

func sampleLibraryEntry(id string) *domain.LibraryEntry {
	return &domain.LibraryEntry{
		ID:          id,
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Test Entry",
		MonitorMode: domain.MonitorModeNone,
	}
}

func sampleLibraryEntryWithParent(id string, kind domain.Kind, parentID string) *domain.LibraryEntry {
	e := sampleLibraryEntry(id)
	e.Kind = kind
	e.ParentID = parentID
	return e
}
