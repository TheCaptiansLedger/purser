// Package tagassignmenttest is the shared contract test suite for the
// ports.TagAssignmentRepository port. See internal/ports/externalidtest for
// the composite-key convention this follows.
package tagassignmenttest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty TagAssignmentRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.TagAssignmentRepository

// TestTagAssignmentRepository runs the shared TagAssignmentRepository
// contract against newRepo.
func TestTagAssignmentRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the assignment", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate composite key returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("delete removes an assignment", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing assignment returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by tag id alone", func(t *testing.T) { testListFiltersByTagID(t, newRepo) })
	t.Run("list filters by entity type and entity id alone", func(t *testing.T) { testListFiltersByEntity(t, newRepo) })
	t.Run("list filters by tag id and entity together", func(t *testing.T) { testListFiltersByBoth(t, newRepo) })
	t.Run("list paginates across an unfiltered set", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.TagAssignmentRepository, ta *domain.TagAssignment) {
	t.Helper()
	if err := r.Create(context.Background(), ta); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "t1", domain.EntityTypePerson, "p1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ta := sampleTagAssignment("t1", domain.EntityTypePerson, "p1")
	mustCreate(t, r, ta)

	got, err := r.Get(context.Background(), "t1", domain.EntityTypePerson, "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.TagID != ta.TagID || got.EntityType != ta.EntityType || got.EntityID != ta.EntityID {
		t.Fatalf("Get returned %+v, want %+v", got, ta)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypePerson, "p1"))

	err := r.Create(context.Background(), sampleTagAssignment("t1", domain.EntityTypePerson, "p1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate composite key returned %v, want ErrConflict", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypePerson, "p1"))

	if err := r.Delete(context.Background(), "t1", domain.EntityTypePerson, "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "t1", domain.EntityTypePerson, "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "t1", domain.EntityTypePerson, "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing assignment returned %v, want ErrNotFound", err)
	}
}

func testListFiltersByTagID(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypeLibraryEntry, "network1"))
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypeLibraryEntry, "studio1"))
	mustCreate(t, r, sampleTagAssignment("t2", domain.EntityTypeLibraryEntry, "network1"))

	byTag, _, err := r.List(ctx, "t1", "", "", 10, "")
	if err != nil {
		t.Fatalf("List(by tag) returned error: %v", err)
	}
	if len(byTag) != 2 {
		t.Fatalf("List(by tag t1) returned %d rows, want 2", len(byTag))
	}
}

func testListFiltersByEntity(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypeItem, "scene1"))
	mustCreate(t, r, sampleTagAssignment("t2", domain.EntityTypeItem, "scene1"))
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypeItem, "scene2"))

	byEntity, _, err := r.List(ctx, "", domain.EntityTypeItem, "scene1", 10, "")
	if err != nil {
		t.Fatalf("List(by entity) returned error: %v", err)
	}
	if len(byEntity) != 2 {
		t.Fatalf("List(by entity scene1) returned %d rows, want 2", len(byEntity))
	}
}

func testListFiltersByBoth(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypePerson, "p1"))
	mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypePerson, "p2"))
	mustCreate(t, r, sampleTagAssignment("t2", domain.EntityTypePerson, "p1"))

	byBoth, _, err := r.List(ctx, "t1", domain.EntityTypePerson, "p1", 10, "")
	if err != nil {
		t.Fatalf("List(by tag and entity) returned error: %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].EntityID != "p1" {
		t.Fatalf("List(by tag and entity) returned %v, want a single row for p1", byBoth)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := 5
	for i := range want {
		mustCreate(t, r, sampleTagAssignment("t1", domain.EntityTypePerson, fmt.Sprintf("p%d", i)))
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		rows, next, err := r.List(ctx, "", "", "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, row := range rows {
			got[row.EntityID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != want {
		t.Fatalf("List across pages returned %d rows, want %d", len(got), want)
	}
}

func sampleTagAssignment(tagID string, entityType domain.EntityType, entityID string) *domain.TagAssignment {
	return &domain.TagAssignment{TagID: tagID, EntityType: entityType, EntityID: entityID}
}
