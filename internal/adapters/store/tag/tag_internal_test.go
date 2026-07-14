package tag

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
// docs/adr/0019-tag-identity-and-get-or-create.md directly — an
// implementation detail of this adapter, not something the shared
// ports.TagRepository contract (internal/ports/tagtest) can observe
// without simulating a crash mid-operation, which is what these tests do
// by hand-writing a stale reservation document.

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

func putReservation(t *testing.T, ds datastore.Datastore, scope domain.TagScope, key, value, tagID string) {
	t.Helper()
	data, err := json.Marshal(reservation{TagID: tagID})
	if err != nil {
		t.Fatalf("marshal reservation returned error: %v", err)
	}
	id := reservationID(scope, key, value)
	if err := ds.Create(context.Background(), datastore.Document{Collection: reservationCollection, ID: id, Data: data}); err != nil {
		t.Fatalf("Create reservation document returned error: %v", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtDeletedTag simulates a crash
// between Delete removing the Tag document and cleaning up its
// reservation: a reservation exists whose tag_id no longer resolves to
// anything. Create against that identity must self-heal (clean up the
// stale reservation) and succeed as a genuinely new tag, not error.
func TestCreate_SelfHealsReservationPointingAtDeletedTag(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	putReservation(t, ds, domain.TagScopeMetadata, "genre", "action", "ghost-tag-id")

	tag := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	if err := r.Create(ctx, tag); err != nil {
		t.Fatalf("Create against an identity with a stale (deleted-target) reservation returned %v, want nil", err)
	}
	if tag.ID != "t1" {
		t.Fatalf("Create against a stale reservation returned ID %q, want the requested %q (should self-heal, not treat the stale reservation as a hit)", tag.ID, "t1")
	}

	got, err := r.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get after self-healed Create returned error: %v", err)
	}
	if got.Value != "action" {
		t.Fatalf("Get after self-healed Create returned Value %q, want %q", got.Value, "action")
	}

	if _, err := ds.Get(ctx, reservationCollection, reservationID(domain.TagScopeMetadata, "genre", "action")); err != nil {
		t.Fatalf("reservation after self-healed Create returned error %v, want a fresh reservation pointing at t1", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtMismatchedTag simulates a
// crash mid-rename: a reservation exists for the old identity, but the
// live Tag it points at now has different fields (the rename's Update
// succeeded before the old reservation could be cleaned up). Create
// against that stale identity must self-heal and succeed as a new tag,
// never silently returning the live-but-mismatched tag.
func TestCreate_SelfHealsReservationPointingAtMismatchedTag(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	live := &domain.Tag{ID: "t1", Key: "mood", Value: "dark", Scope: domain.TagScopeMetadata}
	if err := r.Create(ctx, live); err != nil {
		t.Fatalf("Create for the live tag returned error: %v", err)
	}

	// Simulate the old identity's reservation surviving a rename that
	// already moved t1 to mood/dark.
	putReservation(t, ds, domain.TagScopeMetadata, "genre", "action", "t1")

	newTag := &domain.Tag{ID: "t2", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	if err := r.Create(ctx, newTag); err != nil {
		t.Fatalf("Create against a mismatched stale reservation returned %v, want nil", err)
	}
	if newTag.ID != "t2" {
		t.Fatalf("Create against a mismatched stale reservation returned ID %q, want the requested %q (must not return the mismatched live tag t1)", newTag.ID, "t2")
	}

	gotLive, err := r.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get t1 after self-healed Create returned error: %v", err)
	}
	if gotLive.Key != "mood" || gotLive.Value != "dark" {
		t.Fatalf("Get t1 after self-healed Create returned (%q,%q), want the untouched (%q,%q)", gotLive.Key, gotLive.Value, "mood", "dark")
	}
}

// TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive covers the
// inverse of self-healing: a reservation pointing at a live tag whose
// fields genuinely still match must not be treated as stale.
func TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	if err := r.Create(ctx, &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	err = r.reserve(ctx, reservationID(domain.TagScopeMetadata, "genre", "action"), "t2", domain.TagScopeMetadata, "genre", "action")
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("reserve against a genuinely live identity returned %v, want ErrConflict", err)
	}

	got, err := ds.Get(ctx, reservationCollection, reservationID(domain.TagScopeMetadata, "genre", "action"))
	if err != nil {
		t.Fatalf("Get reservation after rejected reserve returned error: %v", err)
	}
	var res reservation
	if err := json.Unmarshal(got.Data, &res); err != nil {
		t.Fatalf("unmarshal reservation returned error: %v", err)
	}
	if res.TagID != "t1" {
		t.Fatalf("reservation after rejected reserve points at %q, want the original owner %q", res.TagID, "t1")
	}
}
