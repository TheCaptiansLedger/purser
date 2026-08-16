// Package databaseadmintest is the shared contract test suite for the
// ports.DatabaseAdmin port (see internal/ports/persontest for the
// convention this follows, and
// internal/adapters/datastore/datastoretest for the closer sibling this
// mirrors — placed under internal/adapters/database rather than
// internal/ports because the contract needs datastore.Datastore to
// seed/verify documents, and internal/ports must never import an
// adapters package per docs/adr/0001-hexagonal-architecture.md).
package databaseadmintest

import (
	"bytes"
	"context"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"
	"testing"
)

// Fixture bundles the ports.DatabaseAdmin under test together with the
// datastore.Datastore it wraps. DatabaseAdmin itself has no Create
// method, so the contract seeds/verifies documents through DS directly
// and exercises Info/Backup/Restore through Admin.
type Fixture struct {
	Admin ports.DatabaseAdmin
	DS    datastore.Datastore
}

// NewFixtureFunc returns a fresh, empty Fixture for the duration of a
// single subtest.
type NewFixtureFunc func(t *testing.T) Fixture

// TestDatabaseAdmin runs the shared DatabaseAdmin contract against
// newFixture. Each check is its own top-level subtest so a single
// failure identifies exactly which part of the contract broke.
func TestDatabaseAdmin(t *testing.T, newFixture NewFixtureFunc) {
	t.Helper()

	t.Run("info reports driver and per-collection counts", func(t *testing.T) { testInfo(t, newFixture) })
	t.Run("backup then restore round-trips every document", func(t *testing.T) { testBackupRestoreRoundTrip(t, newFixture) })
	t.Run("restore rejects a stream with a wrong version header, leaving the datastore untouched", func(t *testing.T) {
		testRestoreRejectsBadVersion(t, newFixture)
	})
	t.Run("restore rejects a stream with a malformed document line, leaving the datastore untouched", func(t *testing.T) {
		testRestoreRejectsMalformedLine(t, newFixture)
	})
}

func seed(t *testing.T, ds datastore.Datastore) {
	t.Helper()
	docs := []datastore.Document{
		{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1","name":"Widget One"}`)},
		{Collection: "widget", ID: "w2", Data: []byte(`{"id":"w2","name":"Widget Two"}`)},
		{Collection: "gadget", ID: "g1", Data: []byte(`{"id":"g1","name":"Gadget One"}`), Index: map[string]string{"owner": "alice"}},
	}
	for _, d := range docs {
		if err := ds.Create(context.Background(), d); err != nil {
			t.Fatalf("seeding: Create returned error: %v", err)
		}
	}
}

func testInfo(t *testing.T, newFixture NewFixtureFunc) {
	f := newFixture(t)
	seed(t, f.DS)

	info, err := f.Admin.Info(context.Background())
	if err != nil {
		t.Fatalf("Info returned error: %v", err)
	}
	if info.Driver == "" {
		t.Fatalf("Info returned an empty Driver")
	}
	if info.CollectionCounts["widget"] != 2 {
		t.Fatalf("Info CollectionCounts[widget] = %d, want 2", info.CollectionCounts["widget"])
	}
	if info.CollectionCounts["gadget"] != 1 {
		t.Fatalf("Info CollectionCounts[gadget] = %d, want 1", info.CollectionCounts["gadget"])
	}
}

func testBackupRestoreRoundTrip(t *testing.T, newFixture NewFixtureFunc) {
	f := newFixture(t)
	seed(t, f.DS)

	var buf bytes.Buffer
	if err := f.Admin.Backup(context.Background(), &buf); err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("Backup wrote no data")
	}

	if err := f.Admin.Restore(context.Background(), bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	got, err := f.DS.Get(context.Background(), "widget", "w1")
	if err != nil {
		t.Fatalf("Get after restore returned error: %v", err)
	}
	if string(got.Data) != `{"id":"w1","name":"Widget One"}` {
		t.Fatalf("Get after restore returned Data %q", got.Data)
	}

	got, err = f.DS.Get(context.Background(), "gadget", "g1")
	if err != nil {
		t.Fatalf("Get after restore returned error: %v", err)
	}
	if got.Index["owner"] != "alice" {
		t.Fatalf("Get after restore returned Index %v, want owner=alice", got.Index)
	}

	docs, _, err := f.DS.List(context.Background(), "widget", nil, 50, "")
	if err != nil {
		t.Fatalf("List after restore returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("List after restore returned %d widgets, want 2", len(docs))
	}
}

func testRestoreRejectsBadVersion(t *testing.T, newFixture NewFixtureFunc) {
	f := newFixture(t)
	seed(t, f.DS)

	bad := bytes.NewReader([]byte("{\"purser_backup_version\":999}\n"))
	if err := f.Admin.Restore(context.Background(), bad); err == nil {
		t.Fatalf("Restore with a wrong version header returned a nil error")
	}

	if _, err := f.DS.Get(context.Background(), "widget", "w1"); err != nil {
		t.Fatalf("Get after a rejected restore returned error: %v, want the original document untouched", err)
	}
}

func testRestoreRejectsMalformedLine(t *testing.T, newFixture NewFixtureFunc) {
	f := newFixture(t)
	seed(t, f.DS)

	bad := bytes.NewReader([]byte("{\"purser_backup_version\":1}\nnot json\n"))
	if err := f.Admin.Restore(context.Background(), bad); err == nil {
		t.Fatalf("Restore with a malformed document line returned a nil error")
	}

	if _, err := f.DS.Get(context.Background(), "widget", "w1"); err != nil {
		t.Fatalf("Get after a rejected restore returned error: %v, want the original document untouched", err)
	}
}
