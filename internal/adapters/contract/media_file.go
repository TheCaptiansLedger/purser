package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"reflect"
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

	t.Run("MatchDetailRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF MatchDetail Test")
		detail := map[string]any{
			"recording_mbid":       "rec-mbid-123",
			"recording_confidence": float64(0.97),
			"release_mbid":         "rel-mbid-456",
			"release_confidence":   float64(0.82),
		}
		mf := &domain.MediaFile{
			ItemID:          item.ID,
			Path:            "/media/mf-matchdetail-test.flac",
			Size:            5_000_000,
			OSHash:          "matchdetail-hash-001",
			MatchConfidence: domain.MatchVerified,
			MatchDetail:     detail,
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		for _, name := range []string{"ByItemID", "ByPath", "ByOSHash"} {
			var got *domain.MediaFile
			var err error
			switch name {
			case "ByItemID":
				got, err = s.MediaFiles.GetByItemID(ctx, item.ID)
			case "ByPath":
				got, err = s.MediaFiles.GetByPath(ctx, mf.Path)
			case "ByOSHash":
				got, err = s.MediaFiles.GetByOSHash(ctx, mf.OSHash)
			}
			if err != nil {
				t.Fatalf("Get%s: %v", name, err)
			}
			if got.MatchDetail == nil {
				t.Errorf("Get%s: MatchDetail is nil, want populated", name)
				continue
			}
			if !reflect.DeepEqual(got.MatchDetail, detail) {
				t.Errorf("Get%s: MatchDetail = %v, want %v", name, got.MatchDetail, detail)
			}
		}
	})

	t.Run("NilMatchDetailRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF NilMatchDetail Test")
		mf := &domain.MediaFile{
			ItemID:      item.ID,
			Path:        "/media/mf-nilmatchdetail-test.mp4",
			Size:        1_000,
			OSHash:      "nilmatchdetail-hash-002",
			MatchDetail: nil,
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetByItemID: %v", err)
		}
		if got.MatchDetail != nil {
			t.Errorf("MatchDetail = %v, want nil", got.MatchDetail)
		}
	})

	t.Run("SHA1RoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF SHA1 Test")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/media/mf-sha1-test.flac",
			Size:   8_000_000,
			OSHash: "sha1-oshash-001",
			SHA1:   "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetByItemID: %v", err)
		}
		if got.SHA1 != mf.SHA1 {
			t.Errorf("SHA1 = %q, want %q", got.SHA1, mf.SHA1)
		}
	})

	t.Run("MetadataRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF Metadata Test")
		meta := map[string]string{
			"release_id":  "release-abc-001",
			"isrc":        "USSM10012807",
			"disc_number": "1",
		}
		mf := &domain.MediaFile{
			ItemID:   item.ID,
			Path:     "/media/mf-metadata-test.flac",
			Size:     7_000_000,
			OSHash:   "meta-oshash-001",
			Metadata: meta,
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetByItemID: %v", err)
		}
		if got.Metadata == nil {
			t.Fatal("Metadata is nil, want populated map")
		}
		for key, want := range meta {
			if got.Metadata[key] != want {
				t.Errorf("Metadata[%q] = %q, want %q", key, got.Metadata[key], want)
			}
		}
	})

	t.Run("NilMetadataRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF NilMetadata Test")
		mf := &domain.MediaFile{
			ItemID:   item.ID,
			Path:     "/media/mf-nilmetadata-test.flac",
			Size:     6_000_000,
			OSHash:   "nilmeta-oshash-001",
			Metadata: nil,
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MediaFiles.GetByItemID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetByItemID: %v", err)
		}
		if len(got.Metadata) != 0 {
			t.Errorf("Metadata = %v, want nil or empty", got.Metadata)
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
