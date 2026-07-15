// Package musicreleasetest is the shared contract test suite for the
// ports.MusicReleaseRepository port. See internal/ports/grouptest for the
// convention this follows, scoped to only the Create/Get methods this
// walking-skeleton pass defines.
package musicreleasetest

import (
	"context"
	"errors"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty MusicReleaseRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.MusicReleaseRepository

// TestMusicReleaseRepository runs the shared MusicReleaseRepository
// contract against newRepo.
func TestMusicReleaseRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the release", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.MusicReleaseRepository, rel *music.Release) {
	t.Helper()
	if err := r.Create(context.Background(), rel); err != nil {
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
	rel := sampleRelease("r1")
	mustCreate(t, r, rel)

	got, err := r.Get(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Title != rel.Title {
		t.Fatalf("Get returned Title %q, want %q", got.Title, rel.Title)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleRelease("r1"))

	err := r.Create(context.Background(), sampleRelease("r1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func sampleRelease(id string) *music.Release {
	return &music.Release{
		ID:             id,
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Test Release",
		Status:         music.ReleaseStatusStub,
	}
}
