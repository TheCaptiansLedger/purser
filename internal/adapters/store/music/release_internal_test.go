package music

import (
	"context"
	"encoding/json"
	"errors"
	"purser/internal/adapters/datastore"
	dsbadger "purser/internal/adapters/datastore/badger"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"testing"
)

// These tests exercise the reservation self-healing logic described in
// docs/technical/pipeline-music-persist.md's "MusicRelease's own
// reservation-document fix" directly — an implementation detail of this
// adapter, not something the shared ports.MusicReleaseRepository contract
// (internal/ports/musicreleasetest) can observe without simulating a crash
// mid-operation, which is what these tests do by hand-writing a stale
// reservation document. See
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

func testRelease(id, mbid string) *music.Release {
	return &music.Release{
		ID:             id,
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Test Release",
		Status:         music.ReleaseStatusStub,
		MBID:           mbid,
	}
}

func putReservation(t *testing.T, ds datastore.Datastore, mbid, releaseID string) {
	t.Helper()
	data, err := json.Marshal(reservation{ReleaseID: releaseID})
	if err != nil {
		t.Fatalf("marshal reservation returned error: %v", err)
	}
	if err := ds.Create(context.Background(), datastore.Document{Collection: reservationCollection, ID: reservationID(mbid), Data: data}); err != nil {
		t.Fatalf("Create reservation document returned error: %v", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtDeletedRelease simulates a
// crash between Delete removing the release document and cleaning up its
// reservation: a reservation exists whose release_id no longer resolves to
// anything. Create against that MBID must self-heal (clean up the stale
// reservation) and succeed as a genuinely new release, not error.
func TestCreate_SelfHealsReservationPointingAtDeletedRelease(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	putReservation(t, ds, "mbid-1", "ghost-release-id")

	rel := testRelease("r1", "mbid-1")
	if err := r.Create(ctx, rel); err != nil {
		t.Fatalf("Create against an MBID with a stale (deleted-target) reservation returned %v, want nil", err)
	}
	if rel.ID != "r1" {
		t.Fatalf("Create against a stale reservation returned ID %q, want the requested %q (should self-heal, not treat the stale reservation as a hit)", rel.ID, "r1")
	}

	got, err := r.Get(ctx, "r1")
	if err != nil {
		t.Fatalf("Get after self-healed Create returned error: %v", err)
	}
	if got.MBID != "mbid-1" {
		t.Fatalf("Get after self-healed Create returned MBID %q, want %q", got.MBID, "mbid-1")
	}

	if _, err := ds.Get(ctx, reservationCollection, reservationID("mbid-1")); err != nil {
		t.Fatalf("reservation after self-healed Create returned error %v, want a fresh reservation pointing at r1", err)
	}
}

// TestCreate_SelfHealsReservationPointingAtMismatchedRelease simulates a
// crash mid-Update: a reservation exists for the old MBID, but the live
// release it points at now has a different MBID (the rename's Update
// succeeded before the old reservation could be cleaned up). Create
// against that stale MBID must self-heal and succeed as a new release,
// never silently returning the live-but-mismatched release.
func TestCreate_SelfHealsReservationPointingAtMismatchedRelease(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	live := testRelease("r1", "current-mbid")
	if err := r.Create(ctx, live); err != nil {
		t.Fatalf("Create for the live release returned error: %v", err)
	}

	// Simulate the old MBID's reservation surviving an Update that already
	// moved r1 to "current-mbid".
	putReservation(t, ds, "old-mbid", "r1")

	newRelease := testRelease("r2", "old-mbid")
	if err := r.Create(ctx, newRelease); err != nil {
		t.Fatalf("Create against a mismatched stale reservation returned %v, want nil", err)
	}
	if newRelease.ID != "r2" {
		t.Fatalf("Create against a mismatched stale reservation returned ID %q, want the requested %q (must not return the mismatched live release r1)", newRelease.ID, "r2")
	}

	gotLive, err := r.Get(ctx, "r1")
	if err != nil {
		t.Fatalf("Get r1 after self-healed Create returned error: %v", err)
	}
	if gotLive.MBID != "current-mbid" {
		t.Fatalf("Get r1 after self-healed Create returned MBID %q, want untouched %q", gotLive.MBID, "current-mbid")
	}
}

// TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive covers the
// inverse of self-healing: a reservation pointing at a live release whose
// MBID genuinely still matches must not be treated as stale.
func TestReserve_ReturnsErrConflictWhenIdentityIsGenuinelyLive(t *testing.T) {
	ds := newTestDatastore(t)
	r, err := New("test", ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	if err := r.Create(ctx, testRelease("r1", "mbid-1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	err = r.reserve(ctx, testRelease("r2", "mbid-1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("reserve against a genuinely live MBID returned %v, want ErrConflict", err)
	}

	got, err := ds.Get(ctx, reservationCollection, reservationID("mbid-1"))
	if err != nil {
		t.Fatalf("Get reservation after rejected reserve returned error: %v", err)
	}
	var res reservation
	if err := json.Unmarshal(got.Data, &res); err != nil {
		t.Fatalf("unmarshal reservation returned error: %v", err)
	}
	if res.ReleaseID != "r1" {
		t.Fatalf("reservation after rejected reserve points at %q, want the original owner %q", res.ReleaseID, "r1")
	}
}
