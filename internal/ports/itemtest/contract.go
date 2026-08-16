// Package itemtest is the shared contract test suite for the
// ports.ItemRepository port. See internal/ports/persontest for the
// convention this follows.
package itemtest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty ItemRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.ItemRepository

// TestItemRepository runs the shared ItemRepository contract against
// newRepo.
func TestItemRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the item", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing item", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing item returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes an item", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing item returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by library entry, content type, and group independently", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list filters by status", func(t *testing.T) { testListFiltersByStatus(t, newRepo) })
	t.Run("list narrows by content type and status together", func(t *testing.T) { testListFiltersByContentTypeAndStatus(t, newRepo) })
	t.Run("list returns every created item across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("delete batch removes every item atomically", func(t *testing.T) { testDeleteBatch(t, newRepo) })
	t.Run("delete batch with one missing id rolls back the whole batch", func(t *testing.T) { testDeleteBatchRollsBackOnMissing(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.ItemRepository, i *domain.Item) {
	t.Helper()
	if err := r.Create(context.Background(), i); err != nil {
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
	i := sampleItem("i1")
	mustCreate(t, r, i)

	got, err := r.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != i.Title {
		t.Fatalf("Get returned Title %q, want %q", got.Title, i.Title)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleItem("i1"))

	err := r.Create(context.Background(), sampleItem("i1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	i := sampleItem("i1")
	mustCreate(t, r, i)

	i.Title = "Updated Title"
	if err := r.Update(context.Background(), i); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated Title")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleItem("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing item returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleItem("i1"))

	if err := r.Delete(context.Background(), "i1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "i1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing item returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleItemWithGroup("i1", "studio1", "group1"))
	mustCreate(t, r, sampleItemWithGroup("i2", "studio1", "group2"))
	mustCreate(t, r, sampleItemWithGroup("i3", "studio2", "group1"))

	byLibraryEntry, _, err := r.List(ctx, "studio1", "", "", "", 10, "")
	if err != nil {
		t.Fatalf("List(by library entry) returned error: %v", err)
	}
	if len(byLibraryEntry) != 2 {
		t.Fatalf("List(by library entry studio1) returned %d rows, want 2", len(byLibraryEntry))
	}

	byContentType, _, err := r.List(ctx, "", "adult", "", "", 10, "")
	if err != nil {
		t.Fatalf("List(by content type) returned error: %v", err)
	}
	if len(byContentType) != 3 {
		t.Fatalf("List(by content type adult) returned %d rows, want 3", len(byContentType))
	}

	byGroup, _, err := r.List(ctx, "", "", "group1", "", 10, "")
	if err != nil {
		t.Fatalf("List(by group) returned error: %v", err)
	}
	if len(byGroup) != 2 {
		t.Fatalf("List(by group group1) returned %d rows, want 2", len(byGroup))
	}

	byBoth, _, err := r.List(ctx, "studio1", "", "group2", "", 10, "")
	if err != nil {
		t.Fatalf("List(by library entry and group) returned error: %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].ID != "i2" {
		t.Fatalf("List(by library entry and group) returned %v, want [i2]", byBoth)
	}
}

func testListFiltersByStatus(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	statuses := []domain.ItemStatus{
		domain.ItemStatusWanted,
		domain.ItemStatusGrabbed,
		domain.ItemStatusDownloading,
		domain.ItemStatusImported,
		domain.ItemStatusMissing,
		domain.ItemStatusSkipped,
	}
	for i, status := range statuses {
		mustCreate(t, r, sampleItemWithStatus(fmt.Sprintf("i%d", i), status))
	}

	for _, status := range statuses {
		got, _, err := r.List(ctx, "", "", "", status, 10, "")
		if err != nil {
			t.Fatalf("List(by status %s) returned error: %v", status, err)
		}
		if len(got) != 1 || got[0].Status != status {
			t.Fatalf("List(by status %s) returned %v, want exactly one item with that status", status, got)
		}
	}

	unfiltered, _, err := r.List(ctx, "", "", "", "", 10, "")
	if err != nil {
		t.Fatalf("List(unfiltered status) returned error: %v", err)
	}
	if len(unfiltered) != len(statuses) {
		t.Fatalf("List(unfiltered status) returned %d rows, want %d", len(unfiltered), len(statuses))
	}
}

func testListFiltersByContentTypeAndStatus(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleItemWithStatus("i1", domain.ItemStatusWanted))
	mustCreate(t, r, sampleItemWithStatus("i2", domain.ItemStatusMissing))
	other := sampleItemWithStatus("i3", domain.ItemStatusWanted)
	other.ContentType = domain.ContentTypeMusic
	mustCreate(t, r, other)

	got, _, err := r.List(ctx, "", string(domain.ContentTypeAdult), "", domain.ItemStatusWanted, 10, "")
	if err != nil {
		t.Fatalf("List(by content type and status) returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "i1" {
		t.Fatalf("List(by content type and status) returned %v, want [i1]", got)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("i%d", i)
		mustCreate(t, r, sampleItem(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		items, next, err := r.List(ctx, "", "", "", "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, i := range items {
			got[i.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d items, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing item %q", id)
		}
	}
}

func testDeleteBatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleItem("i1"))
	mustCreate(t, r, sampleItem("i2"))
	mustCreate(t, r, sampleItem("i3"))

	if err := r.DeleteBatch(context.Background(), []string{"i1", "i2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := r.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for i1 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "i2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for i2 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "i3"); err != nil {
		t.Fatalf("DeleteBatch removed i3, which wasn't in the batch: %v", err)
	}
}

func testDeleteBatchRollsBackOnMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleItem("i1"))

	err := r.DeleteBatch(context.Background(), []string{"i1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := r.Get(context.Background(), "i1"); err != nil {
		t.Fatalf("DeleteBatch removed i1 despite rolling back: %v", err)
	}
}

func sampleItem(id string) *domain.Item {
	return &domain.Item{
		ID:             id,
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: "entry1",
		Title:          "Test Item",
		Status:         domain.ItemStatusWanted,
	}
}

func sampleItemWithStatus(id string, status domain.ItemStatus) *domain.Item {
	i := sampleItem(id)
	i.Status = status
	return i
}

func sampleItemWithGroup(id, libraryEntryID, groupID string) *domain.Item {
	i := sampleItem(id)
	i.LibraryEntryID = libraryEntryID
	i.GroupID = groupID
	return i
}
