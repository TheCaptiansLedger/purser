// Package entrypersontest is the shared contract test suite for the
// ports.EntryPersonRepository port. See internal/ports/persontest for the
// convention this follows.
package entrypersontest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty EntryPersonRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.EntryPersonRepository

// TestEntryPersonRepository runs the shared EntryPersonRepository contract
// against newRepo.
func TestEntryPersonRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the credit", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate composite key returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("same person can hold two different roles on the same entry", func(t *testing.T) { testSamePersonTwoRoles(t, newRepo) })
	t.Run("update replaces an existing credit", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing credit returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a credit", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing credit returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by library entry and person independently", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list paginates across an unfiltered set", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.EntryPersonRepository, ep *domain.EntryPerson) {
	t.Helper()
	if err := r.Create(context.Background(), ep); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "e1", "p1", "director")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ep := sampleEntryPerson("e1", "p1", "director")
	mustCreate(t, r, ep)

	got, err := r.Get(context.Background(), "e1", "p1", "director")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CreditedAs != ep.CreditedAs {
		t.Fatalf("Get returned CreditedAs %q, want %q", got.CreditedAs, ep.CreditedAs)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleEntryPerson("e1", "p1", "director"))

	err := r.Create(context.Background(), sampleEntryPerson("e1", "p1", "director"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate composite key returned %v, want ErrConflict", err)
	}
}

func testSamePersonTwoRoles(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleEntryPerson("e1", "p1", "director"))
	mustCreate(t, r, sampleEntryPerson("e1", "p1", "writer"))

	if _, err := r.Get(context.Background(), "e1", "p1", "director"); err != nil {
		t.Fatalf("Get(director) returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "e1", "p1", "writer"); err != nil {
		t.Fatalf("Get(writer) returned error: %v", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ep := sampleEntryPerson("e1", "p1", "director")
	mustCreate(t, r, ep)

	ep.CreditedAs = "Updated Credit"
	if err := r.Update(context.Background(), ep); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "e1", "p1", "director")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CreditedAs != "Updated Credit" {
		t.Fatalf("Get after Update returned CreditedAs %q, want %q", got.CreditedAs, "Updated Credit")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleEntryPerson("e1", "missing", "director"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing credit returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleEntryPerson("e1", "p1", "director"))

	if err := r.Delete(context.Background(), "e1", "p1", "director"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "e1", "p1", "director"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "e1", "missing", "director")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing credit returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleEntryPerson("e1", "p1", "director"))
	mustCreate(t, r, sampleEntryPerson("e1", "p2", "writer"))
	mustCreate(t, r, sampleEntryPerson("e2", "p1", "producer"))

	byEntry, _, err := r.List(ctx, "e1", "", 10, "")
	if err != nil {
		t.Fatalf("List(by entry) returned error: %v", err)
	}
	if len(byEntry) != 2 {
		t.Fatalf("List(by entry e1) returned %d rows, want 2", len(byEntry))
	}

	byPerson, _, err := r.List(ctx, "", "p1", 10, "")
	if err != nil {
		t.Fatalf("List(by person) returned error: %v", err)
	}
	if len(byPerson) != 2 {
		t.Fatalf("List(by person p1) returned %d rows, want 2", len(byPerson))
	}

	byBoth, _, err := r.List(ctx, "e1", "p1", 10, "")
	if err != nil {
		t.Fatalf("List(by both) returned error: %v", err)
	}
	if len(byBoth) != 1 {
		t.Fatalf("List(by e1+p1) returned %d rows, want 1", len(byBoth))
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := 5
	for i := range want {
		mustCreate(t, r, sampleEntryPerson("e1", fmt.Sprintf("p%d", i), "cast"))
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		rows, next, err := r.List(ctx, "", "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, row := range rows {
			got[row.PersonID] = true
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

func sampleEntryPerson(libraryEntryID, personID, role string) *domain.EntryPerson {
	return &domain.EntryPerson{
		LibraryEntryID: libraryEntryID,
		PersonID:       personID,
		Role:           role,
		CreditedAs:     "Test Credit",
	}
}
