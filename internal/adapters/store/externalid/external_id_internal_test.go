package externalid

import (
	"context"
	"encoding/json"
	"errors"
	"purser/internal/adapters/datastore"
	dsbadger "purser/internal/adapters/datastore/badger"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// These tests exercise the reservation self-healing logic described in
// docs/adr/0026-external-id-get-or-create.md directly — an implementation
// detail of this adapter, not something the shared
// ports.ExternalIDRepository contract (internal/ports/externalidtest) can
// observe without simulating a crash mid-operation, which is what these
// tests do by hand-writing a stale reservation document. See
// internal/adapters/store/tag/tag_internal_test.go for the convention this
// follows.

func newTestDatastore(t *testing.T) datastore.Datastore {
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
	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}
	return ds
}

func putReservation(t *testing.T, ds datastore.Datastore, entityType domain.EntityType, source domain.ExternalIDSource, value, entityID string) {
	t.Helper()
	data, err := json.Marshal(reservation{EntityID: entityID})
	if err != nil {
		t.Fatalf("marshal reservation returned error: %v", err)
	}
	id := reservationID(entityType, source, value)
	if err := ds.Create(context.Background(), datastore.Document{Collection: reservationCollection, ID: id, Data: data}); err != nil {
		t.Fatalf("Create reservation document returned error: %v", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtDeletedEntity simulates a crash
// between Delete removing the ExternalID document and cleaning up its
// reservation: a reservation exists whose entity_id no longer resolves to
// anything. Create against that identity must self-heal (clean up the
// stale reservation) and succeed as a genuinely new row, not error.
func TestCreate_SelfHealsReservationPointingAtDeletedEntity(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	putReservation(t, ds, domain.EntityTypePerson, domain.ExternalIDSourceStashDB, "value-1", "ghost-entity-id")

	e := &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: domain.ExternalIDSourceStashDB, Value: "value-1"}
	if err := r.Create(ctx, e); err != nil {
		t.Fatalf("Create against an identity with a stale (deleted-target) reservation returned %v, want nil", err)
	}
	if e.EntityID != "p1" {
		t.Fatalf("Create against a stale reservation returned EntityID %q, want the requested %q (should self-heal, not treat the stale reservation as a hit)", e.EntityID, "p1")
	}

	got, err := r.Get(ctx, domain.EntityTypePerson, "p1", string(domain.ExternalIDSourceStashDB))
	if err != nil {
		t.Fatalf("Get after self-healed Create returned error: %v", err)
	}
	if got.Value != "value-1" {
		t.Fatalf("Get after self-healed Create returned Value %q, want %q", got.Value, "value-1")
	}

	if _, err := ds.Get(ctx, reservationCollection, reservationID(domain.EntityTypePerson, domain.ExternalIDSourceStashDB, "value-1")); err != nil {
		t.Fatalf("reservation after self-healed Create returned error %v, want a fresh reservation pointing at p1", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtMismatchedValue simulates a
// crash mid-Update: a reservation exists for the old (entityType, source,
// value) identity, but the live row it points at now has a different Value
// (the rename's Update succeeded before the old reservation could be
// cleaned up). Create against that stale identity must self-heal and
// succeed as a new row, never silently returning the live-but-mismatched
// row.
func TestCreate_SelfHealsReservationPointingAtMismatchedValue(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	live := &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: domain.ExternalIDSourceStashDB, Value: "current-value"}
	if err := r.Create(ctx, live); err != nil {
		t.Fatalf("Create for the live row returned error: %v", err)
	}

	// Simulate the old identity's reservation surviving an Update that
	// already moved p1 to "current-value".
	putReservation(t, ds, domain.EntityTypePerson, domain.ExternalIDSourceStashDB, "old-value", "p1")

	newRow := &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p2", Source: domain.ExternalIDSourceStashDB, Value: "old-value"}
	if err := r.Create(ctx, newRow); err != nil {
		t.Fatalf("Create against a mismatched stale reservation returned %v, want nil", err)
	}
	if newRow.EntityID != "p2" {
		t.Fatalf("Create against a mismatched stale reservation returned EntityID %q, want the requested %q (must not return the mismatched live row p1)", newRow.EntityID, "p2")
	}

	gotLive, err := r.Get(ctx, domain.EntityTypePerson, "p1", string(domain.ExternalIDSourceStashDB))
	if err != nil {
		t.Fatalf("Get p1 after self-healed Create returned error: %v", err)
	}
	if gotLive.Value != "current-value" {
		t.Fatalf("Get p1 after self-healed Create returned Value %q, want untouched %q", gotLive.Value, "current-value")
	}
}

// TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive covers the
// inverse of self-healing: a reservation pointing at a live row whose Value
// genuinely still matches must not be treated as stale.
func TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	if err := r.Create(ctx, &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: domain.ExternalIDSourceStashDB, Value: "value-1"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	id := reservationID(domain.EntityTypePerson, domain.ExternalIDSourceStashDB, "value-1")
	err = r.reserve(ctx, id, &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p2", Source: domain.ExternalIDSourceStashDB, Value: "value-1"})
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("reserve against a genuinely live identity returned %v, want ErrConflict", err)
	}

	got, err := ds.Get(ctx, reservationCollection, id)
	if err != nil {
		t.Fatalf("Get reservation after rejected reserve returned error: %v", err)
	}
	var res reservation
	if err := json.Unmarshal(got.Data, &res); err != nil {
		t.Fatalf("unmarshal reservation returned error: %v", err)
	}
	if res.EntityID != "p1" {
		t.Fatalf("reservation after rejected reserve points at %q, want the original owner %q", res.EntityID, "p1")
	}
}
