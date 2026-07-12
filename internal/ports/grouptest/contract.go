// Package grouptest is the shared contract test suite for the
// ports.GroupRepository port. See internal/ports/persontest for the
// convention this follows.
package grouptest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty GroupRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.GroupRepository

// TestGroupRepository runs the shared GroupRepository contract against
// newRepo.
func TestGroupRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the group", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing group", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing group returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a group", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing group returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created group across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.GroupRepository, g *domain.Group) {
	t.Helper()
	if err := r.Create(context.Background(), g); err != nil {
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
	g := sampleGroup("g1")
	mustCreate(t, r, g)

	got, err := r.Get(context.Background(), "g1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != g.Title {
		t.Fatalf("Get returned Title %q, want %q", got.Title, g.Title)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleGroup("g1"))

	err := r.Create(context.Background(), sampleGroup("g1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	g := sampleGroup("g1")
	mustCreate(t, r, g)

	g.Title = "Updated Title"
	if err := r.Update(context.Background(), g); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "g1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated Title")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleGroup("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing group returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleGroup("g1"))

	if err := r.Delete(context.Background(), "g1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "g1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing group returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("g%d", i)
		mustCreate(t, r, sampleGroup(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		groups, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, g := range groups {
			got[g.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d groups, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing group %q", id)
		}
	}
}

func sampleGroup(id string) *domain.Group {
	return &domain.Group{
		ID:             id,
		LibraryEntryID: "entry1",
		Title:          "Test Group",
		MonitorMode:    domain.MonitorModeNone,
	}
}
