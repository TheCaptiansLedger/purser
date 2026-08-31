package store_test

import (
	"context"
	"errors"
	"purser/internal/adapters/store"
	"purser/internal/ports"
	"testing"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

// filteredWidget is a trivial local type standing in for a real
// FilteredRepository[T] domain entity (Image, Item, LibraryEntry, ...) —
// this package must not know about purser/internal/domain, so it can't
// reuse a real one.
type filteredWidget struct {
	ID       string
	Name     string
	Category string
}

func filteredWidgetID(w *filteredWidget) string { return w.ID }

func filteredWidgetIndex(w *filteredWidget) map[string]string {
	return map[string]string{"category": w.Category}
}

func TestNewFiltered_RejectsEmptyName(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewFiltered("", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err == nil {
		t.Fatal("NewFiltered with an empty name did not return an error")
	}
}

func TestNewFiltered_RejectsEmptyCollection(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewFiltered("test", "", ds, filteredWidgetID, filteredWidgetIndex)
	if err == nil {
		t.Fatal("NewFiltered with an empty collection did not return an error")
	}
}

func TestNewFiltered_RejectsNilDatastore(t *testing.T) {
	_, err := store.NewFiltered[filteredWidget]("test", "filtered_widget", nil, filteredWidgetID, filteredWidgetIndex)
	if err == nil {
		t.Fatal("NewFiltered with a nil datastore did not return an error")
	}
}

func TestNewFiltered_RejectsNilIDFunc(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewFiltered[filteredWidget]("test", "filtered_widget", ds, nil, filteredWidgetIndex)
	if err == nil {
		t.Fatal("NewFiltered with a nil idOf did not return an error")
	}
}

func TestNewFiltered_RejectsNilIndexFunc(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewFiltered[filteredWidget]("test", "filtered_widget", ds, filteredWidgetID, nil)
	if err == nil {
		t.Fatal("NewFiltered with a nil indexOf did not return an error")
	}
}

func TestFilteredRepository_CRUDRoundTrip(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}
	ctx := context.Background()

	w := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
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

	widgets, _, err := repo.List(ctx, map[string]string{"category": "gadget"}, 10, "")
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

func TestFilteredRepository_UpdateBatch(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}
	ctx := context.Background()

	w1 := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
	w2 := &filteredWidget{ID: "w2", Name: "Widget Two", Category: "gadget"}
	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1 returned error: %v", err)
	}
	if err := repo.Create(ctx, w2); err != nil {
		t.Fatalf("Create w2 returned error: %v", err)
	}

	updated1 := &filteredWidget{ID: "w1", Name: "Widget One Updated", Category: "widget"}
	updated2 := &filteredWidget{ID: "w2", Name: "Widget Two Updated", Category: "widget"}
	if err := repo.UpdateBatch(ctx, []*filteredWidget{updated1, updated2}); err != nil {
		t.Fatalf("UpdateBatch returned error: %v", err)
	}

	got1, err := repo.Get(ctx, "w1")
	if err != nil {
		t.Fatalf("Get after UpdateBatch for w1 returned error: %v", err)
	}
	if got1.Name != "Widget One Updated" || got1.Category != "widget" {
		t.Fatalf("Get after UpdateBatch for w1 returned %+v, want Name/Category updated", got1)
	}

	got2, err := repo.Get(ctx, "w2")
	if err != nil {
		t.Fatalf("Get after UpdateBatch for w2 returned error: %v", err)
	}
	if got2.Name != "Widget Two Updated" || got2.Category != "widget" {
		t.Fatalf("Get after UpdateBatch for w2 returned %+v, want Name/Category updated", got2)
	}

	// The stale "gadget" index must have been replaced, not left dangling.
	byOldCategory, _, err := repo.List(ctx, map[string]string{"category": "gadget"}, 10, "")
	if err != nil {
		t.Fatalf("List(category=gadget) returned error: %v", err)
	}
	if len(byOldCategory) != 0 {
		t.Fatalf("List(category=gadget) after UpdateBatch returned %d widgets, want 0 (stale index)", len(byOldCategory))
	}
}

func TestFilteredRepository_DeleteBatch(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}
	ctx := context.Background()

	w1 := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
	w2 := &filteredWidget{ID: "w2", Name: "Widget Two", Category: "gadget"}
	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1 returned error: %v", err)
	}
	if err := repo.Create(ctx, w2); err != nil {
		t.Fatalf("Create w2 returned error: %v", err)
	}

	if err := repo.DeleteBatch(ctx, []string{"w1", "w2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := repo.Get(ctx, "w1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get w1 after DeleteBatch returned %v, want ErrNotFound", err)
	}
	if _, err := repo.Get(ctx, "w2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get w2 after DeleteBatch returned %v, want ErrNotFound", err)
	}
}

func TestFilteredRepository_DeleteBatch_RollsBackOnMissing(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}
	ctx := context.Background()

	w1 := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1 returned error: %v", err)
	}

	err = repo.DeleteBatch(ctx, []string{"w1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := repo.Get(ctx, "w1"); err != nil {
		t.Fatalf("Get w1 after a rolled-back DeleteBatch returned error: %v, want it untouched", err)
	}
}

func TestFilteredRepository_DocumentIDAndIndexOf(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}

	w := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
	if err := repo.Create(context.Background(), w); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if got := repo.DocumentID(w); got != "w1" {
		t.Fatalf("DocumentID(w) = %q, want %q", got, "w1")
	}
	if index := repo.IndexOf(w); index["category"] != "gadget" {
		t.Fatalf("IndexOf(w) = %v, want category=gadget", index)
	}
}

func TestFilteredRepository_LoggerTracerMeterProviderReturnWhatNewFilteredWasGivenViaOptions(t *testing.T) {
	ds := newTestDatastore(t)
	mp := metricnoop.NewMeterProvider()

	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex, store.WithMeterProvider(mp))
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}

	if repo.Logger() == nil {
		t.Fatalf("Logger() returned nil")
	}
	if repo.Tracer() == nil {
		t.Fatalf("Tracer() returned nil")
	}
	if repo.MeterProvider() != mp {
		t.Fatalf("MeterProvider() = %v, want the mp passed to WithMeterProvider", repo.MeterProvider())
	}
}

func TestFilteredRepository_UpdateBatch_RollsBackOnMissing(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewFiltered("test", "filtered_widget", ds, filteredWidgetID, filteredWidgetIndex)
	if err != nil {
		t.Fatalf("NewFiltered returned error: %v", err)
	}
	ctx := context.Background()

	w1 := &filteredWidget{ID: "w1", Name: "Widget One", Category: "gadget"}
	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1 returned error: %v", err)
	}

	updated1 := &filteredWidget{ID: "w1", Name: "Widget One Updated", Category: "widget"}
	missing := &filteredWidget{ID: "missing", Name: "Missing Widget", Category: "widget"}
	err = repo.UpdateBatch(ctx, []*filteredWidget{updated1, missing})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("UpdateBatch with a missing id returned %v, want ErrNotFound", err)
	}

	// Rolled back means w1 must still have its original data, not the
	// update the failed batch attempted.
	got, err := repo.Get(ctx, "w1")
	if err != nil {
		t.Fatalf("Get for w1 returned error: %v", err)
	}
	if got.Name != "Widget One" || got.Category != "gadget" {
		t.Fatalf("UpdateBatch left w1 partially updated despite rolling back: got %+v", got)
	}
}
