// Package imagetest is the shared contract test suite for the
// ports.ImageRepository port. See internal/ports/persontest for the
// convention this follows.
package imagetest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty ImageRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.ImageRepository

// TestImageRepository runs the shared ImageRepository contract against
// newRepo.
func TestImageRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the image", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing image", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing image returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes an image", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing image returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list filters by owner type and owner id", func(t *testing.T) { testListFilters(t, newRepo) })
	t.Run("list paginates across an unfiltered set", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.ImageRepository, img *domain.Image) {
	t.Helper()
	if err := r.Create(context.Background(), img); err != nil {
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
	img := sampleImage("i1", "person", "p1")
	mustCreate(t, r, img)

	got, err := r.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.URL != img.URL {
		t.Fatalf("Get returned URL %q, want %q", got.URL, img.URL)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleImage("i1", "person", "p1"))

	err := r.Create(context.Background(), sampleImage("i1", "person", "p1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	img := sampleImage("i1", "person", "p1")
	mustCreate(t, r, img)

	img.URL = "https://example.com/updated.jpg"
	if err := r.Update(context.Background(), img); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.URL != "https://example.com/updated.jpg" {
		t.Fatalf("Get after Update returned URL %q, want %q", got.URL, "https://example.com/updated.jpg")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleImage("missing", "person", "p1"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing image returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleImage("i1", "person", "p1"))

	if err := r.Delete(context.Background(), "i1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing image returned %v, want ErrNotFound", err)
	}
}

func testListFilters(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	mustCreate(t, r, sampleImage("i1", "person", "p1"))
	mustCreate(t, r, sampleImage("i2", "person", "p1"))
	mustCreate(t, r, sampleImage("i3", "afterdark.performer_profile", "p1"))

	byOwner, _, err := r.List(ctx, "person", "p1", 10, "")
	if err != nil {
		t.Fatalf("List(by owner) returned error: %v", err)
	}
	if len(byOwner) != 2 {
		t.Fatalf("List(by owner person/p1) returned %d rows, want 2", len(byOwner))
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := 5
	for i := range want {
		mustCreate(t, r, sampleImage(fmt.Sprintf("i%d", i), "person", "p1"))
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		rows, next, err := r.List(ctx, "", "", 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, row := range rows {
			got[row.ID] = true
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

func sampleImage(id, ownerType, ownerID string) *domain.Image {
	return &domain.Image{
		ID:        id,
		OwnerType: ownerType,
		OwnerID:   ownerID,
		ImageType: domain.ImageTypePoster,
		URL:       "https://example.com/image.jpg",
	}
}
