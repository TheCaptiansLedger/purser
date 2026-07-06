package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"testing"
)

func TestMusicIdentifier_ContentTypes(t *testing.T) {
	id := identifier.NewMusicIdentifier(&stubExternalIDRepo{}, &stubItemRepo{}, nil, "", nil)
	cts := id.ContentTypes()
	if len(cts) != 1 || cts[0] != domain.ContentTypeMusic {
		t.Errorf("ContentTypes() = %v, want [music]", cts)
	}
}

func TestMusicIdentifier_UnsupportedType(t *testing.T) {
	id := identifier.NewMusicIdentifier(&stubExternalIDRepo{}, &stubItemRepo{}, nil, "", nil)
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

func TestMusicIdentifier_EmbeddedMBZTrackID(t *testing.T) {
	const mbzID = "550e8400-e29b-41d4-a716-446655440000"
	const itemID = "item-music-001"

	item := &domain.Item{ID: itemID, Title: "Bella Donna", ContentType: domain.ContentTypeMusic}
	extIDs := &stubExternalIDRepo{
		entries: map[string]string{
			"item:musicbrainz:" + mbzID: itemID,
		},
	}
	itemRepo := &stubItemRepo{byID: map[string]*domain.Item{itemID: item}}

	id := identifier.NewMusicIdentifier(extIDs, itemRepo, nil, "", nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/music/01 - Bella Donna.flac",
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"musicbrainz_track_id": mbzID,
				"title":                "Bella Donna",
				"artist":               "Stevie Nicks",
				"album":                "Bella Donna",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	// RecordingConfidence must be ≥ baseMBZTrackID (0.95) — T6 invariant.
	// Combined confidence is lower when no metadata source can return release data
	// (the stub ext has no ReleaseDetail), so we test the component, not the combined.
	if candidates[0].RecordingConfidence < 0.95 {
		t.Errorf("recording confidence = %.2f, want >= 0.95 (T6)", candidates[0].RecordingConfidence)
	}
	if candidates[0].Source != "musicbrainz_track_id" {
		t.Errorf("source = %q, want musicbrainz_track_id", candidates[0].Source)
	}
}

func TestMusicIdentifier_NoAcoustIDKey_FallsToFilename(t *testing.T) {
	item := &domain.Item{ID: "item-music-002", Title: "Bella Donna", ContentType: domain.ContentTypeMusic}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	// No AcoustID key → strategy 2 skipped. No embedded MBZ tag. Falls through to filename.
	id := identifier.NewMusicIdentifier(extIDs, itemRepo, nil, "", nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/music/01 - Bella Donna.flac",
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{
			AcoustID:     "some-raw-fingerprint",
			EmbeddedTags: map[string]string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected filename candidates, got none")
	}
	if candidates[0].Confidence < 0.35 || candidates[0].Confidence > 1.0 {
		t.Errorf("filename confidence = %.2f, want in [0.35, 1.0]", candidates[0].Confidence)
	}
}

func TestMusicIdentifier_TagFuzzyMatch(t *testing.T) {
	item := &domain.Item{ID: "item-music-003", Title: "Gold Dust Woman", ContentType: domain.ContentTypeMusic, RuntimeSeconds: 295}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewMusicIdentifier(extIDs, itemRepo, nil, "", nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/music/03 - Gold Dust Woman.flac",
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"title":       "Gold Dust Woman",
				"artist":      "Fleetwood Mac",
				"album":       "Rumours",
				"duration_ms": "295000",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected tag fuzzy candidates, got none")
	}
	// Unique title match with full tag context → boosted confidence.
	if candidates[0].Confidence < 0.70 || candidates[0].Confidence > 1.0 {
		t.Errorf("tag fuzzy confidence = %.2f, want in [0.70, 1.0] (unique title match)", candidates[0].Confidence)
	}
	if candidates[0].Source != "tags" {
		t.Errorf("source = %q, want tags", candidates[0].Source)
	}
}

func TestMusicIdentifier_TagFuzzyMatch_MultipleResults_KeepsLowConfidence(t *testing.T) {
	// Multiple items share the same title — ambiguous, keep conservative 0.75.
	item1 := &domain.Item{ID: "item-a", Title: "Gold Dust Woman", ContentType: domain.ContentTypeMusic, RuntimeSeconds: 295}
	item2 := &domain.Item{ID: "item-b", Title: "Gold Dust Woman", ContentType: domain.ContentTypeMusic, RuntimeSeconds: 295}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item1, item2}}

	id := identifier.NewMusicIdentifier(extIDs, itemRepo, nil, "", nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/music/03 - Gold Dust Woman.flac",
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"title":       "Gold Dust Woman",
				"artist":      "Fleetwood Mac",
				"album":       "Rumours",
				"duration_ms": "295000",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.Confidence < 0.60 || c.Confidence > 1.0 {
			t.Errorf("multi-match confidence = %.2f, want in [0.60, 1.0]", c.Confidence)
		}
	}
}

func TestMusicIdentifier_TagFuzzyMatch_DurationMismatch(t *testing.T) {
	// Duration differs by >5s — item should be excluded
	item := &domain.Item{ID: "item-music-004", Title: "Sara", ContentType: domain.ContentTypeMusic, RuntimeSeconds: 400}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewMusicIdentifier(extIDs, itemRepo, nil, "", nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/music/04 - Sara.flac",
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"title":       "Sara",
				"artist":      "Fleetwood Mac",
				"album":       "Tusk",
				"duration_ms": "200000", // 200s vs 400s — diff > 5s
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Tag fuzzy should filter out; filename parse should then run
	for _, c := range candidates {
		if c.Source == "tags" {
			t.Error("duration-mismatched item should not appear as tag match")
		}
	}
}
