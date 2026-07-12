// Package performerprofiletest is the shared contract test suite for the
// ports.PerformerProfileRepository port. See internal/ports/persontest
// for the convention this follows.
package performerprofiletest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty PerformerProfileRepository for
// the duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.PerformerProfileRepository

// TestPerformerProfileRepository runs the shared
// PerformerProfileRepository contract against newRepo.
func TestPerformerProfileRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the profile", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate person id returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing profile", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing profile returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a profile", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing profile returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created profile across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.PerformerProfileRepository, p *afterdark.PerformerProfile) {
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
	p := sampleProfile("p1")
	mustCreate(t, r, p)

	got, err := r.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CupSize != p.CupSize {
		t.Fatalf("Get returned CupSize %q, want %q", got.CupSize, p.CupSize)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleProfile("p1"))

	err := r.Create(context.Background(), sampleProfile("p1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate person id returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	p := sampleProfile("p1")
	mustCreate(t, r, p)

	p.CupSize = "36"
	if err := r.Update(context.Background(), p); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CupSize != "36" {
		t.Fatalf("Get after Update returned CupSize %q, want %q", got.CupSize, "36")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleProfile("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing profile returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleProfile("p1"))

	if err := r.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing profile returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("p%d", i)
		mustCreate(t, r, sampleProfile(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		profiles, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, p := range profiles {
			got[p.PersonID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d profiles, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing profile %q", id)
		}
	}
}

func sampleProfile(personID string) *afterdark.PerformerProfile {
	return &afterdark.PerformerProfile{PersonID: personID, CupSize: "34"}
}
