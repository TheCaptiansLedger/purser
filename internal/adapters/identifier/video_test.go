package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"testing"
	"time"
)

func TestVideoIdentifier_ContentTypes(t *testing.T) {
	id := identifier.NewVideoIdentifier(&stubMediaFileRepo{}, &stubItemRepo{}, nil)
	cts := id.ContentTypes()
	if len(cts) != 2 {
		t.Fatalf("ContentTypes() len = %d, want 2", len(cts))
	}
}

func TestVideoIdentifier_UnsupportedType(t *testing.T) {
	id := identifier.NewVideoIdentifier(&stubMediaFileRepo{}, &stubItemRepo{}, nil)
	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/fake/scene.mp4",
		ContentType: domain.ContentTypeAdult,
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidates != nil {
		t.Errorf("expected nil for unsupported content type, got %v", candidates)
	}
}

func TestVideoIdentifier_OSHashMatch(t *testing.T) {
	const hash = "1122334455667788"
	const itemID = "item-video-001"

	item := &domain.Item{ID: itemID, Title: "The Matrix", ContentType: domain.ContentTypeMovie}
	mf := &domain.MediaFile{ID: "mf-v001", ItemID: itemID, OSHash: hash}

	mediaRepo := &stubMediaFileRepo{byHash: map[string]*domain.MediaFile{hash: mf}}
	itemRepo := &stubItemRepo{byID: map[string]*domain.Item{itemID: item}}

	id := identifier.NewVideoIdentifier(mediaRepo, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/movies/The.Matrix.1999.mkv",
		ContentType: domain.ContentTypeMovie,
		Fingerprint: &domain.Fingerprint{OSHash: hash},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Confidence != 0.97 {
		t.Errorf("confidence = %.2f, want 0.97", candidates[0].Confidence)
	}
	if candidates[0].Source != "oshash" {
		t.Errorf("source = %q, want oshash", candidates[0].Source)
	}
}

func TestVideoIdentifier_FilenameMovie_TitleYear(t *testing.T) {
	item := &domain.Item{
		ID:          "item-video-002",
		Title:       "The Matrix",
		ContentType: domain.ContentTypeMovie,
		Date:        time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewVideoIdentifier(&stubMediaFileRepo{}, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/movies/The.Matrix.1999.BluRay.1080p.mkv",
		ContentType: domain.ContentTypeMovie,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates from filename parse, got none")
	}
	if candidates[0].Confidence != 0.65 {
		t.Errorf("title+year confidence = %.2f, want 0.65", candidates[0].Confidence)
	}
}

func TestVideoIdentifier_FilenameTV_TitleOnly(t *testing.T) {
	item := &domain.Item{ID: "item-video-003", Title: "Breaking Bad S01E01", ContentType: domain.ContentTypeTV}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewVideoIdentifier(&stubMediaFileRepo{}, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/tv/Breaking.Bad.S01E01.720p.mkv",
		ContentType: domain.ContentTypeTV,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates, got none")
	}
	if candidates[0].Confidence >= 0.85 {
		t.Errorf("filename confidence = %.2f, want < 0.85", candidates[0].Confidence)
	}
}
