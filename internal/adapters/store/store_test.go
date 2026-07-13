package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"purser/internal/adapters/datastore"
	dsbadger "purser/internal/adapters/datastore/badger"
	"purser/internal/adapters/store"
	"purser/internal/ports"
	"testing"
)

// widget is a trivial local type standing in for a real domain entity —
// this package must not know about purser/internal/domain, so it can't
// reuse a real one.
type widget struct {
	ID   string
	Name string
}

func widgetID(w *widget) string { return w.ID }

func TestNew_RejectsEmptyName(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.New("", "widget", ds, widgetID)
	if err == nil {
		t.Fatal("New with an empty name did not return an error")
	}
}

func TestNew_RejectsEmptyCollection(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.New("test", "", ds, widgetID)
	if err == nil {
		t.Fatal("New with an empty collection did not return an error")
	}
}

func TestNew_RejectsNilDatastore(t *testing.T) {
	_, err := store.New[widget]("test", "widget", nil, widgetID)
	if err == nil {
		t.Fatal("New with a nil datastore did not return an error")
	}
}

func TestNew_RejectsNilIDFunc(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.New[widget]("test", "widget", ds, nil)
	if err == nil {
		t.Fatal("New with a nil idOf did not return an error")
	}
}

func TestRepository_CRUDRoundTrip(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.New("test", "widget", ds, widgetID)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx := context.Background()

	w := &widget{ID: "w1", Name: "Widget One"}
	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := repo.Get(ctx, "w1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != "Widget One" {
		t.Fatalf("Get returned Name %q, want %q", got.Name, "Widget One")
	}

	if err := repo.Create(ctx, w); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with a duplicate ID returned %v, want ErrConflict", err)
	}

	w.Name = "Widget One Updated"
	if err := repo.Update(ctx, w); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got, err = repo.Get(ctx, "w1")
	if err != nil {
		t.Fatalf("Get after Update returned error: %v", err)
	}
	if got.Name != "Widget One Updated" {
		t.Fatalf("Get after Update returned Name %q, want %q", got.Name, "Widget One Updated")
	}

	widgets, _, err := repo.List(ctx, 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(widgets) != 1 {
		t.Fatalf("List returned %d widgets, want 1", len(widgets))
	}

	if err := repo.Delete(ctx, "w1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repo.Get(ctx, "w1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func newTestDatastore(t *testing.T) datastore.Datastore {
	t.Helper()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: filepath.Join(t.TempDir())})
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
