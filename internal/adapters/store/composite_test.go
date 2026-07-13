package store_test

import (
	"context"
	"errors"
	"purser/internal/adapters/store"
	"purser/internal/ports"
	"testing"
)

// link is a trivial local type standing in for a real composite-key
// domain entity (EntryPerson, ItemPerson, ...) — this package must not
// know about purser/internal/domain, so it can't reuse a real one.
type link struct {
	ParentID string
	ChildID  string
	Role     string
	Note     string
}

func linkKey(l *link) (string, string, string) { return l.ParentID, l.ChildID, l.Role }

func linkIndex(l *link) map[string]string {
	return map[string]string{"parent_id": l.ParentID, "child_id": l.ChildID}
}

func TestNewComposite_RejectsEmptyName(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewComposite("", "link", ds, linkKey, linkIndex)
	if err == nil {
		t.Fatal("NewComposite with an empty name did not return an error")
	}
}

func TestNewComposite_RejectsNilKeyFunc(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewComposite[link]("test", "link", ds, nil, linkIndex)
	if err == nil {
		t.Fatal("NewComposite with a nil keyOf did not return an error")
	}
}

func TestNewComposite_RejectsNilIndexFunc(t *testing.T) {
	ds := newTestDatastore(t)
	_, err := store.NewComposite("test", "link", ds, linkKey, nil)
	if err == nil {
		t.Fatal("NewComposite with a nil indexOf did not return an error")
	}
}

func TestCompositeRepository_CRUDRoundTrip(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewComposite("test", "link", ds, linkKey, linkIndex)
	if err != nil {
		t.Fatalf("NewComposite returned error: %v", err)
	}
	ctx := context.Background()

	l := &link{ParentID: "p1", ChildID: "c1", Role: "owner"}
	if err := repo.Create(ctx, l); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := repo.Get(ctx, "p1", "c1", "owner")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Role != "owner" {
		t.Fatalf("Get returned Role %q, want %q", got.Role, "owner")
	}

	if err := repo.Create(ctx, l); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with a duplicate key returned %v, want ErrConflict", err)
	}

	l.Note = "updated"
	if err := repo.Update(ctx, l); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	got, err = repo.Get(ctx, "p1", "c1", "owner")
	if err != nil {
		t.Fatalf("Get after Update returned error: %v", err)
	}
	if got.Note != "updated" {
		t.Fatalf("Get after Update returned Note %q, want %q", got.Note, "updated")
	}

	if err := repo.Delete(ctx, "p1", "c1", "owner"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repo.Get(ctx, "p1", "c1", "owner"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestCompositeRepository_ListFiltersIndependently(t *testing.T) {
	ds := newTestDatastore(t)
	repo, err := store.NewComposite("test", "link", ds, linkKey, linkIndex)
	if err != nil {
		t.Fatalf("NewComposite returned error: %v", err)
	}
	ctx := context.Background()

	mustCreateLink(t, repo, "p1", "c1", "owner")
	mustCreateLink(t, repo, "p1", "c2", "editor")
	mustCreateLink(t, repo, "p2", "c3", "owner")

	byParent, _, err := repo.List(ctx, map[string]string{"parent_id": "p1"}, 10, "")
	if err != nil {
		t.Fatalf("List(parent_id=p1) returned error: %v", err)
	}
	if len(byParent) != 2 {
		t.Fatalf("List(parent_id=p1) returned %d links, want 2", len(byParent))
	}

	unfiltered, _, err := repo.List(ctx, nil, 10, "")
	if err != nil {
		t.Fatalf("List(nil) returned error: %v", err)
	}
	if len(unfiltered) != 3 {
		t.Fatalf("List(nil) returned %d links, want 3", len(unfiltered))
	}
}

func mustCreateLink(t *testing.T, repo *store.CompositeRepository[link], parentID, childID, role string) {
	t.Helper()
	if err := repo.Create(context.Background(), &link{ParentID: parentID, ChildID: childID, Role: role}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}
