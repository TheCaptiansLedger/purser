package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"testing"
)

func runMediaFileContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveAndLookupByItemID", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF ItemID Test")
		mf := &domain.MediaFile{
			ItemID:          item.ID,
			Path:            "/media/mf-itemid-test.mp4",
			Size:            100_000,
			OSHash:          "aabbcc001",
			MatchConfidence: domain.MatchManual,
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if mf.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetByItemID: %v", err)
		}
		if got.Path != mf.Path {
			t.Errorf("Path = %q, want %q", got.Path, mf.Path)
		}
		if got.Size != mf.Size {
			t.Errorf("Size = %d, want %d", got.Size, mf.Size)
		}
	})

	t.Run("SaveAndLookupByPath", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF Path Test")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/media/mf-path-test.mp4",
			Size:   200_000,
			OSHash: "aabbcc002",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByPath(ctx, mf.Path)
		if err != nil {
			t.Fatalf("GetByPath: %v", err)
		}
		if got.ItemID != item.ID {
			t.Errorf("ItemID = %q, want %q", got.ItemID, item.ID)
		}
	})

	t.Run("SaveAndLookupByOSHash", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF OSHash Test")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/media/mf-oshash-test.mp4",
			Size:   300_000,
			OSHash: "deadbeef-contract-003",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByOSHash(ctx, mf.OSHash)
		if err != nil {
			t.Fatalf("GetByOSHash: %v", err)
		}
		if got.Path != mf.Path {
			t.Errorf("Path = %q, want %q", got.Path, mf.Path)
		}
	})

	t.Run("GetByItemIDNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.MediaFiles.GetByItemID(ctx, "no-item-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("GetByPathNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.MediaFiles.GetByPath(ctx, "/no/such/path.mp4")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF Delete Test")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/media/mf-delete-test.mp4",
			Size:   400_000,
			OSHash: "aabbcc-delete-004",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.MediaFiles.Delete(ctx, mf.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete GetByItemID: want ErrNotFound, got %v", err)
		}
		_, err = s.MediaFiles.GetByPath(ctx, mf.Path)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete GetByPath: want ErrNotFound, got %v", err)
		}
		_, err = s.MediaFiles.GetByOSHash(ctx, mf.OSHash)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete GetByOSHash: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ResaveCleansPreviousIndexEntries", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF Resave Test")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/media/mf-resave-old.mp4",
			Size:   500_000,
			OSHash: "aabbcc-resave-old-005",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save initial: %v", err)
		}
		// Move to a new path and hash — old index entries must be removed.
		mf.Path = "/media/mf-resave-new.mp4"
		mf.OSHash = "aabbcc-resave-new-005"
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save updated: %v", err)
		}
		_, err := s.MediaFiles.GetByPath(ctx, "/media/mf-resave-old.mp4")
		if !errs.IsNotFound(err) {
			t.Errorf("old path still resolvable after resave, want ErrNotFound, got %v", err)
		}
		got, err := s.MediaFiles.GetByPath(ctx, "/media/mf-resave-new.mp4")
		if err != nil {
			t.Fatalf("new path not found after resave: %v", err)
		}
		if got.OSHash != "aabbcc-resave-new-005" {
			t.Errorf("OSHash = %q, want new hash", got.OSHash)
		}
	})
}
