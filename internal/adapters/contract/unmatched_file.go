package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

func runUnmatchedFileContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveGetDelete", func(t *testing.T) {
		ctx := context.Background()
		uf := &domain.UnmatchedFile{
			ID:           "umf-contract-001",
			Path:         "/media/contract-unmatched-001.mp4",
			Size:         12345,
			ContentType:  domain.ContentTypeAdult,
			Status:       domain.UnmatchedPending,
			DiscoveredAt: time.Now().UTC().Truncate(time.Second),
		}
		if err := s.Unmatched.Save(ctx, uf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Unmatched.Get(ctx, uf.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Path != uf.Path {
			t.Errorf("Path = %q, want %q", got.Path, uf.Path)
		}
		if got.Status != domain.UnmatchedPending {
			t.Errorf("Status = %q, want pending", got.Status)
		}
		if got.ContentType != uf.ContentType {
			t.Errorf("ContentType = %q, want %q", got.ContentType, uf.ContentType)
		}
		if err := s.Unmatched.Delete(ctx, uf.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = s.Unmatched.Get(ctx, uf.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		ctx := context.Background()
		pending := &domain.UnmatchedFile{ID: "umf-status-pending", Path: "/media/status-pending.mp4", Size: 1, ContentType: domain.ContentTypeAdult, Status: domain.UnmatchedPending, DiscoveredAt: time.Now().UTC()}
		matched := &domain.UnmatchedFile{ID: "umf-status-matched", Path: "/media/status-matched.mp4", Size: 2, ContentType: domain.ContentTypeAdult, Status: domain.UnmatchedMatched, DiscoveredAt: time.Now().UTC()}
		for _, uf := range []*domain.UnmatchedFile{pending, matched} {
			if err := s.Unmatched.Save(ctx, uf); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, err := s.Unmatched.List(ctx, ports.UnmatchedFilter{Status: domain.UnmatchedPending})
		if err != nil {
			t.Fatalf("List by status: %v", err)
		}
		for _, r := range results {
			if r.Status != domain.UnmatchedPending {
				t.Errorf("status filter leaked record with status %q", r.Status)
			}
		}
		found := false
		for _, r := range results {
			if r.ID == pending.ID {
				found = true
			}
		}
		if !found {
			t.Error("pending record not found in status-filtered list")
		}
	})

	t.Run("ListByPath", func(t *testing.T) {
		ctx := context.Background()
		target := &domain.UnmatchedFile{ID: "umf-path-target", Path: "/media/path-target-unique-xyz.mp4", Size: 1, ContentType: domain.ContentTypeAdult, Status: domain.UnmatchedPending, DiscoveredAt: time.Now().UTC()}
		other := &domain.UnmatchedFile{ID: "umf-path-other", Path: "/media/path-other-unique-xyz.mp4", Size: 2, ContentType: domain.ContentTypeAdult, Status: domain.UnmatchedPending, DiscoveredAt: time.Now().UTC()}
		for _, uf := range []*domain.UnmatchedFile{target, other} {
			if err := s.Unmatched.Save(ctx, uf); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, err := s.Unmatched.List(ctx, ports.UnmatchedFilter{Path: target.Path})
		if err != nil {
			t.Fatalf("List by path: %v", err)
		}
		if len(results) != 1 || results[0].ID != target.ID {
			t.Errorf("path filter returned %d results, want 1 with ID %q", len(results), target.ID)
		}
	})

	t.Run("SaveIsIdempotent", func(t *testing.T) {
		ctx := context.Background()
		uf := &domain.UnmatchedFile{
			ID:           "umf-idempotent",
			Path:         "/media/idempotent.mp4",
			Size:         100,
			ContentType:  domain.ContentTypeAdult,
			Status:       domain.UnmatchedPending,
			DiscoveredAt: time.Now().UTC(),
		}
		if err := s.Unmatched.Save(ctx, uf); err != nil {
			t.Fatalf("Save initial: %v", err)
		}
		uf.Status = domain.UnmatchedMatched
		if err := s.Unmatched.Save(ctx, uf); err != nil {
			t.Fatalf("Save update: %v", err)
		}
		got, err := s.Unmatched.Get(ctx, uf.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Status != domain.UnmatchedMatched {
			t.Errorf("after update Status = %q, want matched", got.Status)
		}
	})
}
