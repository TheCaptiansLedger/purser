// Package imageselectiontest is the shared contract test suite for the
// ports.ImageSelectionRepository port. See internal/ports/imagetest for
// the convention this follows.
package imageselectiontest

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty ImageSelectionRepository for
// the duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.ImageSelectionRepository

// TestImageSelectionRepository runs the shared ImageSelectionRepository
// contract against newRepo.
func TestImageSelectionRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the selection", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate slot returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces which image the slot points at", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing slot returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a selection", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing slot returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("different owners/image types are independent slots", func(t *testing.T) { testIndependentSlots(t, newRepo) })
}

func sampleSelection(ownerType, ownerID string, imageType domain.ImageType, imageID string) *domain.ImageSelection {
	return &domain.ImageSelection{OwnerType: ownerType, OwnerID: ownerID, ImageType: imageType, ImageID: imageID}
}

func mustCreate(t *testing.T, r ports.ImageSelectionRepository, sel *domain.ImageSelection) {
	t.Helper()
	if err := r.Create(context.Background(), sel); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))

	got, err := r.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ImageID != "img1" {
		t.Fatalf("Get returned ImageID %q, want %q", got.ImageID, "img1")
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))

	err := r.Create(context.Background(), sampleSelection("person", "p1", domain.ImageTypePoster, "img2"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create for an already-occupied slot returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))

	if err := r.Update(context.Background(), sampleSelection("person", "p1", domain.ImageTypePoster, "img2")); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ImageID != "img2" {
		t.Fatalf("Get after Update returned ImageID %q, want %q", got.ImageID, "img2")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on a missing slot returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))

	if err := r.Delete(context.Background(), "person", "p1", domain.ImageTypePoster); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "person", "p1", domain.ImageTypePoster); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "person", "p1", domain.ImageTypePoster)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on a missing slot returned %v, want ErrNotFound", err)
	}
}

func testIndependentSlots(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypePoster, "img1"))
	mustCreate(t, r, sampleSelection("person", "p2", domain.ImageTypePoster, "img2"))
	mustCreate(t, r, sampleSelection("person", "p1", domain.ImageTypeHero, "img3"))

	got, err := r.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ImageID != "img1" {
		t.Fatalf("Get(person/p1/poster) returned ImageID %q, want %q — a sibling slot leaked in", got.ImageID, "img1")
	}
}
