// Package externalidtest is the shared contract test suite for the
// ports.ExternalIDRepository port. See internal/ports/entrypersontest for
// the composite-key convention this follows.
package externalidtest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty ExternalIDRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.ExternalIDRepository

// TestExternalIDRepository runs the shared ExternalIDRepository contract
// against newRepo.
func TestExternalIDRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the external id", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate composite key returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces the value", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing external id returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes an external id", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing external id returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by entity type and entity id", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list paginates across an unfiltered set", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.ExternalIDRepository, e *domain.ExternalID) {
	t.Helper()
	if err := r.Create(context.Background(), e); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	e := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, e)

	got, err := r.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != e.Value {
		t.Fatalf("Get returned Value %q, want %q", got.Value, e.Value)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleExternalID("p1", "stashdb"))

	err := r.Create(context.Background(), sampleExternalID("p1", "stashdb"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate composite key returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	e := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, e)

	e.Value = "updated-value"
	if err := r.Update(context.Background(), e); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != "updated-value" {
		t.Fatalf("Get after Update returned Value %q, want %q", got.Value, "updated-value")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleExternalID("missing", "stashdb"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing external id returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleExternalID("p1", "stashdb"))

	if err := r.Delete(context.Background(), domain.EntityTypePerson, "p1", "stashdb"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), domain.EntityTypePerson, "missing", "stashdb")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing external id returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleExternalID("p1", "stashdb"))
	mustCreate(t, r, sampleExternalID("p1", "tpdb"))
	mustCreate(t, r, sampleExternalID("p2", "stashdb"))

	byEntity, _, err := r.List(ctx, "", "p1", 10, "")
	if err != nil {
		t.Fatalf("List(by entity) returned error: %v", err)
	}
	if len(byEntity) != 2 {
		t.Fatalf("List(by entity p1) returned %d rows, want 2", len(byEntity))
	}

	byType, _, err := r.List(ctx, domain.EntityTypePerson, "", 10, "")
	if err != nil {
		t.Fatalf("List(by type) returned error: %v", err)
	}
	if len(byType) != 3 {
		t.Fatalf("List(by type person) returned %d rows, want 3", len(byType))
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := 5
	for i := range want {
		mustCreate(t, r, sampleExternalID(fmt.Sprintf("p%d", i), "stashdb"))
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		rows, next, err := r.List(ctx, "", "", 2, pageToken)
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

func sampleExternalID(entityID, source string) *domain.ExternalID {
	return &domain.ExternalID{
		EntityType: domain.EntityTypePerson,
		EntityID:   entityID,
		Source:     domain.ExternalIDSource(source),
		Value:      "external-value",
	}
}
