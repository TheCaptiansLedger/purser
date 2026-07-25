// Package unmatchedfiletest is the shared contract test suite for the
// ports.UnmatchedFileRepository port. See internal/ports/mediafiletest for
// the convention this follows, scoped down to the Create/Get methods this
// issue's slice of the port actually has.
package unmatchedfiletest

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// NewRepositoryFunc returns a fresh, empty UnmatchedFileRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.UnmatchedFileRepository

// TestUnmatchedFileRepository runs the shared UnmatchedFileRepository
// contract against newRepo.
func TestUnmatchedFileRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the unmatched file", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate ID returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.UnmatchedFileRepository, u *domain.UnmatchedFile) {
	t.Helper()
	if err := r.Create(context.Background(), u); err != nil {
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
	u := sampleUnmatchedFile("uf1")
	mustCreate(t, r, u)

	got, err := r.Get(context.Background(), "uf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Path != u.Path {
		t.Fatalf("Get returned Path %q, want %q", got.Path, u.Path)
	}
	if got.OSHash != u.OSHash {
		t.Fatalf("Get returned OSHash %q, want %q", got.OSHash, u.OSHash)
	}
	if got.Status != u.Status {
		t.Fatalf("Get returned Status %q, want %q", got.Status, u.Status)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleUnmatchedFile("uf1"))

	err := r.Create(context.Background(), sampleUnmatchedFile("uf1"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate ID returned %v, want ErrConflict", err)
	}
}

func sampleUnmatchedFile(id string) *domain.UnmatchedFile {
	return &domain.UnmatchedFile{
		ID:           id,
		Path:         "/media/incoming/" + id + ".flac",
		Size:         123456,
		OSHash:       "0123456789abcdef",
		SHA1:         "a9993e364706816aba3e25717850c26c9cd0d89d",
		DiscoveredAt: time.Unix(1700000000, 0).UTC(),
		Status:       domain.UnmatchedFileStatusPending,
	}
}
