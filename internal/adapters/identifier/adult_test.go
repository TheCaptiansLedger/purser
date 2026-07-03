package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"testing"
)

func TestAdultIdentifier_ContentTypes(t *testing.T) {
	id := identifier.NewAdultIdentifier(&stubMediaFileRepo{}, &stubItemRepo{}, nil)
	cts := id.ContentTypes()
	if len(cts) != 2 {
		t.Fatalf("ContentTypes() len = %d, want 2", len(cts))
	}
}

func TestAdultIdentifier_UnsupportedType(t *testing.T) {
	id := identifier.NewAdultIdentifier(&stubMediaFileRepo{}, &stubItemRepo{}, nil)
	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/fake/movie.mkv",
		ContentType: domain.ContentTypeMovie,
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidates != nil {
		t.Errorf("expected nil for unsupported content type, got %v", candidates)
	}
}

func TestAdultIdentifier_OSHashMatch(t *testing.T) {
	const hash = "aabbccdd11223344"
	const itemID = "item-001"

	item := &domain.Item{ID: itemID, Title: "Test Scene", ContentType: domain.ContentTypeAdult}
	mf := &domain.MediaFile{ID: "mf-001", ItemID: itemID, OSHash: hash}

	mediaRepo := &stubMediaFileRepo{byHash: map[string]*domain.MediaFile{hash: mf}}
	itemRepo := &stubItemRepo{byID: map[string]*domain.Item{itemID: item}}

	id := identifier.NewAdultIdentifier(mediaRepo, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/media/adult/scene.mp4",
		ContentType: domain.ContentTypeAdult,
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
	if candidates[0].Item.ID != itemID {
		t.Errorf("item ID = %q, want %q", candidates[0].Item.ID, itemID)
	}
}

func TestAdultIdentifier_FilenameMatch_TitleOnly(t *testing.T) {
	item := &domain.Item{ID: "item-002", Title: "Amazing Scene", ContentType: domain.ContentTypeAdult}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewAdultIdentifier(&stubMediaFileRepo{}, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/media/adult/amazing-scene.mp4",
		ContentType: domain.ContentTypeAdult,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates from filename parse, got none")
	}
	if candidates[0].Confidence >= 0.85 {
		t.Errorf("filename confidence = %.2f, want < 0.85", candidates[0].Confidence)
	}
	if candidates[0].Source != "filename" {
		t.Errorf("source = %q, want filename", candidates[0].Source)
	}
}

func TestAdultIdentifier_FilenameMatch_HighFieldCount(t *testing.T) {
	item := &domain.Item{ID: "item-003", Title: "Studio Scene", ContentType: domain.ContentTypeJAV}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewAdultIdentifier(&stubMediaFileRepo{}, itemRepo, nil)

	// Studio slug + date + title = 3 fields → confidence 0.60
	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/media/jav/SOD 2024-03-15 Studio Scene.mp4",
		ContentType: domain.ContentTypeJAV,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates from filename parse, got none")
	}
	if candidates[0].Confidence != 0.60 {
		t.Errorf("confidence = %.2f, want 0.60", candidates[0].Confidence)
	}
}

func TestAdultIdentifier_NilFingerprint(t *testing.T) {
	item := &domain.Item{ID: "item-004", Title: "Scene Title", ContentType: domain.ContentTypeAdult}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewAdultIdentifier(&stubMediaFileRepo{}, itemRepo, nil)

	// Nil fingerprint must not panic
	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/media/adult/scene-title.mp4",
		ContentType: domain.ContentTypeAdult,
		Fingerprint: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = candidates
}
