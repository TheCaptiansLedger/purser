// Package persontest is the shared contract test suite for the
// ports.PersonRepository port (see ADR 0003's contract-test convention). It
// is a normal buildable package, not a _test.go file, because Go test files
// cannot be imported across packages — every adapter (internal/adapters/store/person
// today, others later) imports this from its own test file and runs it
// against its own constructor, proving Liskov substitutability without
// duplicating the assertions per adapter.
package persontest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// NewRepositoryFunc returns a fresh, empty PersonRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.PersonRepository

// TestPersonRepository runs the shared PersonRepository contract against
// newRepo. Each check is its own top-level subtest so a single failure
// identifies exactly which part of the contract broke.
func TestPersonRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the person", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing person", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing person returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a person", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing person returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created person across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("list with a name filter returns only matches", func(t *testing.T) { testListNameFilterMatches(t, newRepo) })
	t.Run("list with a name filter that matches nothing returns no results", func(t *testing.T) { testListNameFilterNoMatch(t, newRepo) })
	t.Run("list with an empty name filter is unfiltered", func(t *testing.T) { testListNameFilterEmptyIsUnfiltered(t, newRepo) })
	t.Run("list with a name filter is case-insensitive", func(t *testing.T) { testListNameFilterCaseInsensitive(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.PersonRepository, p *domain.Person) {
	t.Helper()
	if err := r.Create(context.Background(), p); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	p := samplePerson("p1")
	mustCreate(t, r, p)

	got, err := r.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != p.Name {
		t.Fatalf("Get returned Name %q, want %q", got.Name, p.Name)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, samplePerson("p1"))

	err := r.Create(context.Background(), samplePerson("p1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	p := samplePerson("p1")
	mustCreate(t, r, p)

	p.Name = "Updated Name"
	if err := r.Update(context.Background(), p); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Fatalf("Get after Update returned Name %q, want %q", got.Name, "Updated Name")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), samplePerson("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing person returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, samplePerson("p1"))

	if err := r.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "p1")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing person returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("p%d", i)
		mustCreate(t, r, samplePerson(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		people, next, err := r.List(ctx, "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, p := range people {
			got[p.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d people, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing person %q", id)
		}
	}
}

func testListNameFilterMatches(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	stevie := samplePerson("p1")
	stevie.Name = "Stevie Nicks"
	mustCreate(t, r, stevie)

	other := samplePerson("p2")
	other.Name = "Lindsey Buckingham"
	mustCreate(t, r, other)

	people, _, err := r.List(ctx, "Nicks", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(people) != 1 || people[0].ID != "p1" {
		t.Fatalf("List with name filter %q returned %v, want only p1", "Nicks", people)
	}
}

func testListNameFilterNoMatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	mustCreate(t, r, samplePerson("p1"))

	people, _, err := r.List(ctx, "no such person", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(people) != 0 {
		t.Fatalf("List with a non-matching name filter returned %d people, want 0", len(people))
	}
}

func testListNameFilterEmptyIsUnfiltered(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	mustCreate(t, r, samplePerson("p1"))
	mustCreate(t, r, samplePerson("p2"))

	people, _, err := r.List(ctx, "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("List with an empty name filter returned %d people, want 2", len(people))
	}
}

func testListNameFilterCaseInsensitive(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	stevie := samplePerson("p1")
	stevie.Name = "Stevie Nicks"
	mustCreate(t, r, stevie)

	people, _, err := r.List(ctx, "nicks", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(people) != 1 || people[0].ID != "p1" {
		t.Fatalf("List with name filter %q returned %v, want only p1", "nicks", people)
	}
}

func samplePerson(id string) *domain.Person {
	return &domain.Person{
		ID:          id,
		Name:        "Test Person",
		Gender:      domain.GenderUnknown,
		MonitorMode: domain.MonitorModeNone,
		AddedAt:     time.Now(),
		UpdatedAt:   time.Now(),
	}
}
