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
	t.Run("create with an identical composite key and value is an idempotent get-or-create hit", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("create with a same composite key but different value returns ErrConflict", func(t *testing.T) { testCreateConflictingValue(t, newRepo) })
	t.Run("create with a duplicate (entity type, source, value) but different entity id is a get-or-create hit", func(t *testing.T) { testCreateGetOrCreateHit(t, newRepo) })
	t.Run("update replaces the value", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing external id returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("update changing value moves the reservation", func(t *testing.T) { testUpdateChangingValueMovesReservation(t, newRepo) })
	t.Run("update to a value already owned by a different row returns ErrConflict", func(t *testing.T) { testUpdateConflictingValue(t, newRepo) })
	t.Run("delete removes an external id", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing external id returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by entity type and entity id", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list paginates across an unfiltered set", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("GetByValue returns the row linking that source/value", func(t *testing.T) { testGetByValue(t, newRepo) })
	t.Run("GetByValue on an unknown value returns ErrNotFound", func(t *testing.T) { testGetByValueNotFound(t, newRepo) })
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

	dup := sampleExternalID("p1", "stashdb")
	if err := r.Create(context.Background(), dup); err != nil {
		t.Fatalf("Create with an identical (entity type, entity id, source, value) returned %v, want nil (idempotent get-or-create hit)", err)
	}
	if dup.EntityID != "p1" {
		t.Fatalf("Create idempotent hit mutated EntityID to %q, want unchanged %q", dup.EntityID, "p1")
	}
}

func testCreateConflictingValue(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleExternalID("p1", "stashdb"))

	conflicting := sampleExternalID("p1", "stashdb")
	conflicting.Value = "a-different-value"
	err := r.Create(context.Background(), conflicting)
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with the same composite key but a different value returned %v, want ErrConflict", err)
	}
}

func testCreateGetOrCreateHit(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	first := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, first)

	second := sampleExternalID("p2", "stashdb")
	second.Value = first.Value
	if err := r.Create(ctx, second); err != nil {
		t.Fatalf("Create with a duplicate (entity type, source, value) but different entity id returned %v, want nil", err)
	}
	if second.EntityID != "p1" {
		t.Fatalf("Create get-or-create hit returned EntityID %q, want the original owner %q", second.EntityID, "p1")
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

func testUpdateChangingValueMovesReservation(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	e := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, e)
	oldValue := e.Value

	e.Value = "new-value"
	if err := r.Update(ctx, e); err != nil {
		t.Fatalf("Update changing Value returned error: %v", err)
	}

	got, err := r.GetByValue(ctx, domain.EntityTypePerson, domain.ExternalIDSource("stashdb"), "new-value")
	if err != nil {
		t.Fatalf("GetByValue on the new value returned error: %v, want the updated row", err)
	}
	if got.EntityID != "p1" {
		t.Fatalf("GetByValue on the new value returned EntityID %q, want %q", got.EntityID, "p1")
	}

	if _, err := r.GetByValue(ctx, domain.EntityTypePerson, domain.ExternalIDSource("stashdb"), oldValue); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByValue on the old (pre-Update) value returned %v, want ErrNotFound", err)
	}
}

func testUpdateConflictingValue(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	first := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, first)
	second := sampleExternalID("p2", "stashdb")
	second.Value = "taken-value"
	mustCreate(t, r, second)

	first.Value = "taken-value"
	err := r.Update(ctx, first)
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Update to a value already owned by a different (entity type, source) row returned %v, want ErrConflict", err)
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

func testGetByValue(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	e := sampleExternalID("p1", "stashdb")
	mustCreate(t, r, e)

	got, err := r.GetByValue(ctx, domain.EntityTypePerson, domain.ExternalIDSource("stashdb"), e.Value)
	if err != nil {
		t.Fatalf("GetByValue returned error: %v", err)
	}
	if got.EntityID != "p1" {
		t.Fatalf("GetByValue returned EntityID %q, want %q", got.EntityID, "p1")
	}
}

func testGetByValueNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.GetByValue(context.Background(), domain.EntityTypePerson, domain.ExternalIDSource("stashdb"), "unknown-value")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByValue on an unknown value returned %v, want ErrNotFound", err)
	}
}

// sampleExternalID's Value defaults to a per-entityID literal — not a
// shared constant — because Create's get-or-create identity is now
// (EntityType, Source, Value), not just (EntityType, EntityID, Source); a
// shared literal Value across distinct entities would make every List/
// pagination fixture in this suite an unintended get-or-create hit on the
// first one created. See docs/adr/0026-external-id-get-or-create.md and the
// analogous note in docs/adr/0019-tag-identity-and-get-or-create.md's
// Consequences about per-VU-suffixed Tag fixture values. Tests that
// specifically exercise the get-or-create hit path set matching Values
// explicitly.
func sampleExternalID(entityID, source string) *domain.ExternalID {
	return &domain.ExternalID{
		EntityType: domain.EntityTypePerson,
		EntityID:   entityID,
		Source:     domain.ExternalIDSource(source),
		Value:      entityID + "-value",
	}
}
