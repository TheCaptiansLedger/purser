// Package tagtest is the shared contract test suite for the
// ports.TagRepository port. See internal/ports/persontest for the
// convention this follows.
package tagtest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty TagRepository for the duration
// of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.TagRepository

// TestTagRepository runs the shared TagRepository contract against
// newRepo.
func TestTagRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the tag", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing tag", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing tag returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a tag", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing tag returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created tag across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.TagRepository, tag *domain.Tag) {
	t.Helper()
	if err := r.Create(context.Background(), tag); err != nil {
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
	tag := sampleTag("t1")
	mustCreate(t, r, tag)

	got, err := r.Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != tag.Value {
		t.Fatalf("Get returned Value %q, want %q", got.Value, tag.Value)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	err := r.Create(context.Background(), sampleTag("t1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	tag := sampleTag("t1")
	mustCreate(t, r, tag)

	tag.Value = "gonzo"
	if err := r.Update(context.Background(), tag); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != "gonzo" {
		t.Fatalf("Get after Update returned Value %q, want %q", got.Value, "gonzo")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleTag("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing tag returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	if err := r.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "t1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing tag returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("t%d", i)
		mustCreate(t, r, sampleTag(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		tags, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, tag := range tags {
			got[tag.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d tags, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing tag %q", id)
		}
	}
}

func sampleTag(id string) *domain.Tag {
	return &domain.Tag{ID: id, Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
}
