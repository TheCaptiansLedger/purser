// Package datastoretest is the shared contract test suite for the
// datastore.Datastore interface. See internal/ports/persontest for the
// convention this follows.
package datastoretest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"
	"testing"
)

// NewDatastoreFunc returns a fresh, empty datastore.Datastore for the
// duration of a single subtest.
type NewDatastoreFunc func(t *testing.T) datastore.Datastore

// TestDatastore runs the shared Datastore contract against newDS.
func TestDatastore(t *testing.T, newDS NewDatastoreFunc) {
	t.Helper()

	t.Run("get on empty datastore returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newDS) })
	t.Run("create then get round-trips the document", func(t *testing.T) { testCreateThenGet(t, newDS) })
	t.Run("create with a duplicate collection+id returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newDS) })
	t.Run("create with the same id in a different collection succeeds", func(t *testing.T) { testCreateSameIDDifferentCollection(t, newDS) })
	t.Run("update replaces an existing document", func(t *testing.T) { testUpdate(t, newDS) })
	t.Run("update on a missing document returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newDS) })
	t.Run("delete removes a document", func(t *testing.T) { testDelete(t, newDS) })
	t.Run("delete on a missing document returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newDS) })
	t.Run("list paginates across an unfiltered collection", func(t *testing.T) { testListPaginates(t, newDS) })
	t.Run("list filters by a single index key", func(t *testing.T) { testListFilterSingleKey(t, newDS) })
	t.Run("list filters by multiple index keys with AND semantics", func(t *testing.T) { testListFilterMultiKey(t, newDS) })
	t.Run("list filter matching nothing returns an empty page", func(t *testing.T) { testListFilterNoMatch(t, newDS) })
	t.Run("update replaces stale index entries", func(t *testing.T) { testUpdateReplacesIndex(t, newDS) })
	t.Run("create batch stores every document atomically", func(t *testing.T) { testCreateBatch(t, newDS) })
	t.Run("create batch with one conflicting id rolls back the whole batch", func(t *testing.T) { testCreateBatchRollsBackOnConflict(t, newDS) })
	t.Run("delete batch removes every document atomically", func(t *testing.T) { testDeleteBatch(t, newDS) })
	t.Run("delete batch with one missing id rolls back the whole batch", func(t *testing.T) { testDeleteBatchRollsBackOnMissing(t, newDS) })
	t.Run("update batch replaces every document atomically", func(t *testing.T) { testUpdateBatch(t, newDS) })
	t.Run("update batch with one missing id rolls back the whole batch", func(t *testing.T) { testUpdateBatchRollsBackOnMissing(t, newDS) })
}

func mustCreate(t *testing.T, ds datastore.Datastore, doc datastore.Document) {
	t.Helper()
	if err := ds.Create(context.Background(), doc); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func doc(collection, id string, index map[string]string) datastore.Document {
	return datastore.Document{
		Collection: collection,
		ID:         id,
		Data:       []byte(fmt.Sprintf(`{"id":%q}`, id)),
		Index:      index,
	}
}

func testGetOnEmptyNotFound(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	_, err := ds.Get(context.Background(), "widget", "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty datastore returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	d := doc("widget", "w1", nil)
	mustCreate(t, ds, d)

	got, err := ds.Get(context.Background(), "widget", "w1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got.Data) != string(d.Data) {
		t.Fatalf("Get returned Data %q, want %q", got.Data, d.Data)
	}
}

func testCreateDuplicate(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	mustCreate(t, ds, doc("widget", "w1", nil))

	err := ds.Create(context.Background(), doc("widget", "w1", nil))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate collection+id returned %v, want ErrConflict", err)
	}
}

func testCreateSameIDDifferentCollection(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	mustCreate(t, ds, doc("widget", "shared", nil))

	if err := ds.Create(context.Background(), doc("gadget", "shared", nil)); err != nil {
		t.Fatalf("Create with same id in a different collection returned error: %v", err)
	}
}

func testUpdate(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	mustCreate(t, ds, doc("widget", "w1", nil))

	updated := datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1","updated":true}`)}
	if err := ds.Update(context.Background(), updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := ds.Get(context.Background(), "widget", "w1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got.Data) != string(updated.Data) {
		t.Fatalf("Get after Update returned Data %q, want %q", got.Data, updated.Data)
	}
}

func testUpdateMissing(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	err := ds.Update(context.Background(), doc("widget", "missing", nil))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing document returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	mustCreate(t, ds, doc("widget", "w1", nil))

	if err := ds.Delete(context.Background(), "widget", "w1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := ds.Get(context.Background(), "widget", "w1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	err := ds.Delete(context.Background(), "widget", "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing document returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()

	want := 5
	for i := range want {
		mustCreate(t, ds, doc("widget", fmt.Sprintf("w%d", i), nil))
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		docs, next, err := ds.List(ctx, "widget", nil, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, d := range docs {
			got[d.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != want {
		t.Fatalf("List across pages returned %d documents, want %d", len(got), want)
	}
}

func testListFilterSingleKey(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("image", "i1", map[string]string{"owner_type": "person"}))
	mustCreate(t, ds, doc("image", "i2", map[string]string{"owner_type": "person"}))
	mustCreate(t, ds, doc("image", "i3", map[string]string{"owner_type": "group"}))

	docs, _, err := ds.List(ctx, "image", map[string]string{"owner_type": "person"}, 10, "")
	if err != nil {
		t.Fatalf("List(filter) returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("List(owner_type=person) returned %d documents, want 2", len(docs))
	}
}

func testListFilterMultiKey(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("image", "i1", map[string]string{"owner_type": "person", "owner_id": "p1"}))
	mustCreate(t, ds, doc("image", "i2", map[string]string{"owner_type": "person", "owner_id": "p2"}))
	mustCreate(t, ds, doc("image", "i3", map[string]string{"owner_type": "person", "owner_id": "p1"}))

	docs, _, err := ds.List(ctx, "image", map[string]string{"owner_type": "person", "owner_id": "p1"}, 10, "")
	if err != nil {
		t.Fatalf("List(filter) returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("List(owner_type=person,owner_id=p1) returned %d documents, want 2", len(docs))
	}
	for _, d := range docs {
		if d.ID != "i1" && d.ID != "i3" {
			t.Errorf("List(owner_type=person,owner_id=p1) unexpectedly returned %q", d.ID)
		}
	}
}

func testListFilterNoMatch(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("image", "i1", map[string]string{"owner_type": "person"}))

	docs, next, err := ds.List(ctx, "image", map[string]string{"owner_type": "group"}, 10, "")
	if err != nil {
		t.Fatalf("List(filter) returned error: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("List(owner_type=group) returned %d documents, want 0", len(docs))
	}
	if next != "" {
		t.Fatalf("List(owner_type=group) returned next page token %q, want empty", next)
	}
}

func testUpdateReplacesIndex(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("image", "i1", map[string]string{"owner_type": "person"}))

	updated := doc("image", "i1", map[string]string{"owner_type": "group"})
	if err := ds.Update(ctx, updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	byOldIndex, _, err := ds.List(ctx, "image", map[string]string{"owner_type": "person"}, 10, "")
	if err != nil {
		t.Fatalf("List(old index) returned error: %v", err)
	}
	if len(byOldIndex) != 0 {
		t.Fatalf("List(owner_type=person) after Update returned %d documents, want 0 (stale index)", len(byOldIndex))
	}

	byNewIndex, _, err := ds.List(ctx, "image", map[string]string{"owner_type": "group"}, 10, "")
	if err != nil {
		t.Fatalf("List(new index) returned error: %v", err)
	}
	if len(byNewIndex) != 1 {
		t.Fatalf("List(owner_type=group) after Update returned %d documents, want 1", len(byNewIndex))
	}
}

func testCreateBatch(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()

	docs := []datastore.Document{
		doc("widget", "w1", nil),
		doc("widget", "w2", nil),
		doc("widget", "w3", nil),
	}
	if err := ds.CreateBatch(ctx, docs); err != nil {
		t.Fatalf("CreateBatch returned error: %v", err)
	}

	for _, d := range docs {
		if _, err := ds.Get(ctx, "widget", d.ID); err != nil {
			t.Fatalf("Get after CreateBatch for %q returned error: %v", d.ID, err)
		}
	}
}

func testCreateBatchRollsBackOnConflict(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("widget", "w2", nil))

	err := ds.CreateBatch(ctx, []datastore.Document{
		doc("widget", "w1", nil),
		doc("widget", "w2", nil), // already exists — the whole batch must fail
		doc("widget", "w3", nil),
	})
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("CreateBatch with a conflicting id returned %v, want ErrConflict", err)
	}

	// Rolled back means w1/w3 must not have been created either.
	if _, err := ds.Get(ctx, "widget", "w1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("CreateBatch left w1 created despite rolling back: %v", err)
	}
	if _, err := ds.Get(ctx, "widget", "w3"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("CreateBatch left w3 created despite rolling back: %v", err)
	}
}

func testDeleteBatch(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("widget", "w1", nil))
	mustCreate(t, ds, doc("widget", "w2", nil))
	mustCreate(t, ds, doc("widget", "w3", nil))

	if err := ds.DeleteBatch(ctx, "widget", []string{"w1", "w2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := ds.Get(ctx, "widget", "w1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for w1 returned %v, want ErrNotFound", err)
	}
	if _, err := ds.Get(ctx, "widget", "w2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for w2 returned %v, want ErrNotFound", err)
	}
	if _, err := ds.Get(ctx, "widget", "w3"); err != nil {
		t.Fatalf("DeleteBatch removed w3, which wasn't in the batch: %v", err)
	}
}

func testDeleteBatchRollsBackOnMissing(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("widget", "w1", nil))

	err := ds.DeleteBatch(ctx, "widget", []string{"w1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	// Rolled back means w1 must still exist.
	if _, err := ds.Get(ctx, "widget", "w1"); err != nil {
		t.Fatalf("DeleteBatch removed w1 despite rolling back: %v", err)
	}
}

func testUpdateBatch(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	mustCreate(t, ds, doc("widget", "w1", nil))
	mustCreate(t, ds, doc("widget", "w2", nil))
	mustCreate(t, ds, doc("widget", "w3", nil))

	updated := []datastore.Document{
		{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1","updated":true}`)},
		{Collection: "widget", ID: "w2", Data: []byte(`{"id":"w2","updated":true}`)},
	}
	if err := ds.UpdateBatch(ctx, updated); err != nil {
		t.Fatalf("UpdateBatch returned error: %v", err)
	}

	for _, want := range updated {
		got, err := ds.Get(ctx, "widget", want.ID)
		if err != nil {
			t.Fatalf("Get after UpdateBatch for %q returned error: %v", want.ID, err)
		}
		if string(got.Data) != string(want.Data) {
			t.Fatalf("Get after UpdateBatch for %q returned Data %q, want %q", want.ID, got.Data, want.Data)
		}
	}

	// w3 wasn't in the batch — must be untouched.
	got, err := ds.Get(ctx, "widget", "w3")
	if err != nil {
		t.Fatalf("Get for w3 returned error: %v", err)
	}
	if string(got.Data) != string(doc("widget", "w3", nil).Data) {
		t.Fatalf("UpdateBatch modified w3, which wasn't in the batch: %q", got.Data)
	}
}

func testUpdateBatchRollsBackOnMissing(t *testing.T, newDS NewDatastoreFunc) {
	ds := newDS(t)
	ctx := context.Background()
	original := doc("widget", "w1", nil)
	mustCreate(t, ds, original)

	err := ds.UpdateBatch(ctx, []datastore.Document{
		{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1","updated":true}`)},
		{Collection: "widget", ID: "missing", Data: []byte(`{"id":"missing","updated":true}`)},
	})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("UpdateBatch with a missing id returned %v, want ErrNotFound", err)
	}

	// Rolled back means w1 must still have its original data, not the
	// update the failed batch attempted — proves no partial write
	// committed, not just that the call returned an error.
	got, err := ds.Get(ctx, "widget", "w1")
	if err != nil {
		t.Fatalf("Get for w1 returned error: %v", err)
	}
	if string(got.Data) != string(original.Data) {
		t.Fatalf("UpdateBatch left w1 partially updated despite rolling back: got %q, want original %q", got.Data, original.Data)
	}
}
