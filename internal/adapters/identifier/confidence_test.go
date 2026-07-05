package identifier

import (
	"math"
	"purser/internal/domain"
	"testing"
)

func TestTagQualityBase(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want float64
	}{
		{"full with track", map[string]string{"title": "T", "artist": "A", "album": "L", "tracknumber": "1"}, tagBaseFullWithTrack},
		{"full with track (alt key)", map[string]string{"title": "T", "artist": "A", "album": "L", "track": "1"}, tagBaseFullWithTrack},
		{"full set", map[string]string{"title": "T", "artist": "A", "album": "L"}, tagBaseFullSet},
		{"title+artist", map[string]string{"title": "T", "artist": "A"}, tagBaseArtist},
		{"title only", map[string]string{"title": "T"}, tagBaseTitle},
		{"no tags", map[string]string{}, tagBaseNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := normalizeFingerprint(&domain.Fingerprint{EmbeddedTags: tt.tags})
			got := tagQualityBase(fp)
			if got != tt.want {
				t.Errorf("tagQualityBase = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestComputeConfidence_NoExternalItem(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{EmbeddedTags: map[string]string{"title": "T"}})
	c := domain.MatchCandidate{} // no ExternalItem
	got := computeConfidence(baseMBZTrackID, 1.0, c, fp)
	if got != baseMBZTrackID {
		t.Errorf("computeConfidence(no external item) = %.4f, want %.4f", got, baseMBZTrackID)
	}
}

func TestComputeConfidence_AllBonuses_VerificationOne(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":       "Bella Donna",
			"artist":      "Stevie Nicks",
			"album":       "Bella Donna",
			"duration_ms": "234000",
		},
	})
	c := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:       "Bella Donna",
			GroupTitle:  "Bella Donna",
			RuntimeSecs: 234,
			Studio:      &domain.ExternalStudio{Name: "Stevie Nicks"},
		},
	}
	const verificationScore = 1.0
	want := tagBaseFullSet + (bonusTitleAgreement+bonusArtistAgreement+bonusAlbumAgreement+bonusDurationAgreement)*verificationScore
	got := computeConfidence(tagBaseFullSet, verificationScore, c, fp)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("computeConfidence(all bonuses) = %.4f, want %.4f", got, want)
	}
}

func TestComputeConfidence_AcoustIDScalesBonuses(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":  "Bella Donna",
			"artist": "Stevie Nicks",
			"album":  "Bella Donna",
		},
	})
	c := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:      "Bella Donna",
			GroupTitle: "Bella Donna",
			Studio:     &domain.ExternalStudio{Name: "Stevie Nicks"},
		},
	}
	const acoustidScore = 0.80
	want := tagBaseFullSet + (bonusTitleAgreement+bonusArtistAgreement+bonusAlbumAgreement)*acoustidScore
	got := computeConfidence(tagBaseFullSet, acoustidScore, c, fp)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("computeConfidence(scaled) = %.4f, want %.4f", got, want)
	}
}

func TestComputeConfidence_AlbumOutweighsDuration(t *testing.T) {
	// The original bug: album match and duration match scored equally.
	// Album is a stronger signal (release-specific); it must rank higher.
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":       "I Wish You Were There",
			"artist":      "REO Speedwagon",
			"album":       "Hi Infidelity (2024 Remaster)",
			"duration_ms": "247000",
		},
	})
	withAlbum := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:      "I Wish You Were There",
			GroupTitle: "Hi Infidelity (2024 Remaster)",
			Studio:     &domain.ExternalStudio{Name: "REO Speedwagon"},
			// no RuntimeSecs → no duration bonus
		},
	}
	withDurationNotAlbum := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:       "I Wish You Were There",
			GroupTitle:  "Hi Infidelity", // different release — no album bonus
			RuntimeSecs: 247,             // matches duration
			Studio:      &domain.ExternalStudio{Name: "REO Speedwagon"},
		},
	}
	const acoustidScore = 0.95
	scoreAlbum := computeConfidence(tagBaseFullSet, acoustidScore, withAlbum, fp)
	scoreDuration := computeConfidence(tagBaseFullSet, acoustidScore, withDurationNotAlbum, fp)
	if scoreAlbum <= scoreDuration {
		t.Errorf("album-matched candidate (%.4f) must outscore duration-only candidate (%.4f)", scoreAlbum, scoreDuration)
	}
}

func TestComputeConfidence_Cap(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":       "Bella Donna",
			"artist":      "Stevie Nicks",
			"album":       "Bella Donna",
			"duration_ms": "234000",
		},
	})
	c := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:       "Bella Donna",
			GroupTitle:  "Bella Donna",
			RuntimeSecs: 234,
			Studio:      &domain.ExternalStudio{Name: "Stevie Nicks"},
		},
	}
	// baseMBZTrackID(0.95) + all bonuses at 1.0 > 1.0 — must cap.
	got := computeConfidence(baseMBZTrackID, 1.0, c, fp)
	if got != 1.0 {
		t.Errorf("computeConfidence(cap) = %.4f, want 1.0", got)
	}
}

func TestComputeConfidence_NoTags_FingerprintFallback(t *testing.T) {
	// No useful embedded tags: confidence must not exceed acoustidNoTagCeiling.
	fp := normalizeFingerprint(&domain.Fingerprint{EmbeddedTags: map[string]string{}})
	c := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{Title: "Something"},
	}
	got := computeConfidence(tagBaseNone, 0.95, c, fp)
	if got > acoustidNoTagCeiling {
		t.Errorf("no-tag confidence = %.4f, must not exceed ceiling %.4f", got, acoustidNoTagCeiling)
	}
}

func TestComputeConfidence_NoBonus_Mismatch(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":  "Wrong Title",
			"artist": "Wrong Artist",
			"album":  "Wrong Album",
		},
	})
	c := domain.MatchCandidate{
		ExternalItem: &domain.ExternalItem{
			Title:      "Bella Donna",
			GroupTitle: "Bella Donna",
			Studio:     &domain.ExternalStudio{Name: "Stevie Nicks"},
		},
	}
	want := tagBaseFullSet // no agreement → only the base
	got := computeConfidence(tagBaseFullSet, 1.0, c, fp)
	if got != want {
		t.Errorf("computeConfidence(mismatch) = %.4f, want %.4f", got, want)
	}
}
