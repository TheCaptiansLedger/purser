package identifier

import (
	"purser/internal/domain"
	"testing"
)

func TestComputeConfidence_BaseOnly(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":  "Bella Donna",
			"artist": "Stevie Nicks",
		},
	})
	c := domain.MatchCandidate{}
	got := computeConfidence(baseMBZTrackID, c, fp)
	if got != baseMBZTrackID {
		t.Errorf("computeConfidence(base-only) = %.4f, want %.4f", got, baseMBZTrackID)
	}
}

func TestComputeConfidence_AllBonuses(t *testing.T) {
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
	want := baseAcoustID + bonusTitle + bonusArtist + bonusAlbum + bonusDuration
	got := computeConfidence(baseAcoustID, c, fp)
	if got != want {
		t.Errorf("computeConfidence(all-bonuses) = %.4f, want %.4f", got, want)
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
	// baseMBZTrackID(0.95) + 4×bonus(0.20) = 1.15, must cap to 1.0
	got := computeConfidence(baseMBZTrackID, c, fp)
	if got != 1.0 {
		t.Errorf("computeConfidence(cap) = %.4f, want 1.0", got)
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
	got := computeConfidence(baseAcoustID, c, fp)
	if got != baseAcoustID {
		t.Errorf("computeConfidence(mismatch) = %.4f, want %.4f", got, baseAcoustID)
	}
}
