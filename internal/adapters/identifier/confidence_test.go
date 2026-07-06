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

// ── computeRecordingConfidence ────────────────────────────────────────────────

func TestComputeRecordingConfidence_NoTagAgreement(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{"title": "Wrong Title", "artist": "Wrong Artist"},
	})
	got, reasons := computeRecordingConfidence(tagBaseFullSet, 1.0, "Bella Donna", "Stevie Nicks", 234, fp)
	if got != tagBaseFullSet {
		t.Errorf("got %.4f, want %.4f (base only when no tag agreement)", got, tagBaseFullSet)
	}
	if reasons.TitleTag != 0 || reasons.ArtistTag != 0 {
		t.Errorf("reasons should be zero on mismatch: title=%.2f artist=%.2f", reasons.TitleTag, reasons.ArtistTag)
	}
}

func TestComputeRecordingConfidence_AllBonuses(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title": "Bella Donna", "artist": "Stevie Nicks",
			"album": "Bella Donna", "duration_ms": "234000",
		},
	})
	got, reasons := computeRecordingConfidence(tagBaseFullSet, 1.0, "Bella Donna", "Stevie Nicks", 234, fp)
	want := tagBaseFullSet + (bonusTitleAgreement+bonusArtistAgreement+bonusDurationAgreement)*1.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("got %.4f, want %.4f", got, want)
	}
	if reasons.TitleTag != 1.0 || reasons.ArtistTag != 1.0 || reasons.Duration != 1.0 {
		t.Errorf("all reasons should be 1.0: title=%.2f artist=%.2f duration=%.2f", reasons.TitleTag, reasons.ArtistTag, reasons.Duration)
	}
}

func TestComputeRecordingConfidence_AcoustIDScalesBonuses(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title": "Bella Donna", "artist": "Stevie Nicks",
		},
	})
	const acoustidScore = 0.80
	got, _ := computeRecordingConfidence(tagBaseFullSet, acoustidScore, "Bella Donna", "Stevie Nicks", 0, fp)
	want := tagBaseFullSet + (bonusTitleAgreement+bonusArtistAgreement)*acoustidScore
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("got %.4f, want %.4f", got, want)
	}
}

func TestComputeRecordingConfidence_Cap(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title": "Bella Donna", "artist": "Stevie Nicks", "duration_ms": "234000",
		},
	})
	// baseMBZTrackID(0.95) + bonuses > 1.0 — must cap at 1.0
	got, _ := computeRecordingConfidence(baseMBZTrackID, 1.0, "Bella Donna", "Stevie Nicks", 234, fp)
	if got != 1.0 {
		t.Errorf("cap failed: got %.4f, want 1.0", got)
	}
}

func TestComputeRecordingConfidence_FingerprintInReasons(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{EmbeddedTags: map[string]string{}})
	_, reasons := computeRecordingConfidence(tagBaseNone, 0.75, "", "", 0, fp)
	if reasons.Fingerprint != 0.75 {
		t.Errorf("Fingerprint in reasons = %.2f, want 0.75", reasons.Fingerprint)
	}
}

// T3: duration score boundaries propagate correctly into recording confidence.
func TestComputeRecordingConfidence_T3_DurationBoundaries(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{"duration_ms": "180000"},
	})
	exact, _ := computeRecordingConfidence(tagBaseNone, 1.0, "", "", 180, fp)  // |diff|=0 → durScore=1.0
	over10, _ := computeRecordingConfidence(tagBaseNone, 1.0, "", "", 191, fp) // |diff|=11 → durScore=0.0
	if exact <= over10 {
		t.Errorf("T3: ≤1s diff (%.4f) must outscore >10s diff (%.4f)", exact, over10)
	}
	// Exact boundary: 1s diff must produce durScore=1.0
	at1s, _ := computeRecordingConfidence(tagBaseNone, 1.0, "", "", 181, fp) // |diff|=1 → durScore=1.0
	if at1s != exact {
		t.Errorf("1s diff should equal 0s diff (both score 1.0): 0s=%.4f 1s=%.4f", exact, at1s)
	}
}

// ── computeReleaseConfidence ──────────────────────────────────────────────────

func TestComputeReleaseConfidence_NoEmbeddedAlbum(t *testing.T) {
	got, _ := computeReleaseConfidence("", 0, "Hi Infidelity", "1981-02")
	if got != 0.50 {
		t.Errorf("no album tag: got %.4f, want 0.50", got)
	}
}

func TestComputeReleaseConfidence_ExactMatch(t *testing.T) {
	got, reasons := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity", "")
	want := 1.00 * 0.80
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("exact match: got %.4f, want %.4f", got, want)
	}
	if reasons.AlbumTag != 1.00 {
		t.Errorf("AlbumTag = %.2f, want 1.00", reasons.AlbumTag)
	}
}

func TestComputeReleaseConfidence_NormalizeAlbumMatch(t *testing.T) {
	// "Hi Infidelity" vs "Hi Infidelity (2024 Remaster)" — same album after
	// stripping edition qualifiers → albumSim = 0.90.
	got, _ := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity (2024 Remaster)", "")
	want := 0.90 * 0.80
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("normalizeAlbum match: got %.4f, want %.4f", got, want)
	}
}

func TestComputeReleaseConfidence_EmbeddedLongerThanRelease(t *testing.T) {
	// Root-cause case: file tagged "Hi Infidelity (2024 Remaster)", MBZ title "Hi Infidelity".
	// Previously returned 0.00; now returns 0.90 via normalizeAlbum path.
	got, _ := computeReleaseConfidence("Hi Infidelity (2024 Remaster)", 0, "Hi Infidelity", "")
	want := 0.90 * 0.80
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("embedded longer than release: got %.4f, want %.4f (was 0.00 before fix)", got, want)
	}
}

func TestComputeReleaseConfidence_NoOverlap(t *testing.T) {
	got, _ := computeReleaseConfidence("Hi Infidelity", 0, "Find Your Own Way Home", "")
	if got != 0.00 {
		t.Errorf("no overlap: got %.4f, want 0.00", got)
	}
}

func TestComputeReleaseConfidence_YearBonus(t *testing.T) {
	withYear, _ := computeReleaseConfidence("Hi Infidelity", 1981, "Hi Infidelity", "1981-02")
	withoutYear, _ := computeReleaseConfidence("Hi Infidelity", 1981, "Hi Infidelity", "1982-01")
	want := withoutYear + 0.20
	if math.Abs(withYear-want) > 1e-9 {
		t.Errorf("year bonus: got %.4f, want %.4f (base %.4f + 0.20)", withYear, want, withoutYear)
	}
}

func TestComputeReleaseConfidence_YearPartialDate(t *testing.T) {
	// releaseDate may be a bare year string ("1981"), not full ISO date
	got, _ := computeReleaseConfidence("Hi Infidelity", 1981, "Hi Infidelity", "1981")
	withFull, _ := computeReleaseConfidence("Hi Infidelity", 1981, "Hi Infidelity", "1981-02")
	if math.Abs(got-withFull) > 1e-9 {
		t.Errorf("bare year and ISO date should produce the same score: %f vs %f", got, withFull)
	}
}

// T2: exact (1.00 similarity) > prefix (0.85) > no match (0.00) release confidence.
func TestComputeReleaseConfidence_T2_Ordering(t *testing.T) {
	exact, _ := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity", "")
	prefix, _ := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity (2024 Remaster)", "")
	none, _ := computeReleaseConfidence("Hi Infidelity", 0, "Find Your Own Way Home", "")
	if !(exact > prefix && prefix > none) {
		t.Errorf("T2 violated: exact=%.4f prefix=%.4f none=%.4f", exact, prefix, none)
	}
}

// ── combinedConfidence ────────────────────────────────────────────────────────

// T5: two candidates that differ in any component cannot produce an equal combined score.
func TestCombinedConfidence_T5_NoCombinedTie(t *testing.T) {
	recConf := 0.80

	// Same recording confidence, different release confidence → different combined.
	relA := 0.80 // exact album match: 1.0 × 0.80
	relB := 0.72 // normalizeAlbum match: 0.90 × 0.80
	confA := combinedConfidence(recConf, relA)
	confB := combinedConfidence(recConf, relB)
	if confA == confB {
		t.Errorf("T5: different release confidences (%.4f vs %.4f) gave identical combined %.4f", relA, relB, confA)
	}

	// Different recording confidence, same release confidence → different combined.
	recC, recD := 0.90, 0.85
	relSame := 0.80
	confC := combinedConfidence(recC, relSame)
	confD := combinedConfidence(recD, relSame)
	if confC == confD {
		t.Errorf("T5: different recording confidences (%.4f vs %.4f) gave identical combined %.4f", recC, recD, confC)
	}
}

// ── Hi Infidelity regression ──────────────────────────────────────────────────

// TestHiInfidelityRegression asserts that the original bug (all candidates scoring
// identically) cannot recur. A file tagged "album: Hi Infidelity" must score the
// actual album strictly higher than a compilation with no album overlap.
func TestHiInfidelityRegression(t *testing.T) {
	fp := normalizeFingerprint(&domain.Fingerprint{
		EmbeddedTags: map[string]string{
			"title":       "Take It on the Run",
			"artist":      "REO Speedwagon",
			"album":       "Hi Infidelity",
			"duration_ms": "234000",
		},
	})
	const acoustidScore = 0.95
	base := tagQualityBase(fp)

	// Candidate A: the correct release
	recA, _ := computeRecordingConfidence(base, acoustidScore, "Take It on the Run", "REO Speedwagon", 234, fp)
	relA, _ := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity", "1981-02")
	confA := combinedConfidence(recA, relA)

	// Candidate B: a compilation with no album overlap
	recB, _ := computeRecordingConfidence(base, acoustidScore, "Take It on the Run", "REO Speedwagon", 234, fp)
	relB, _ := computeReleaseConfidence("Hi Infidelity", 0, "Find Your Own Way Home", "")
	confB := combinedConfidence(recB, relB)

	if confA <= confB {
		t.Errorf("regression: Hi Infidelity (%.4f) must outscore compilation (%.4f)", confA, confB)
	}
	if relA-relB < 0.30 {
		t.Errorf("release confidence diff must be ≥0.30: album=%.4f compilation=%.4f diff=%.4f", relA, relB, relA-relB)
	}
}

// T1: any release whose normalized title matches the embedded album tag must score
// strictly higher release confidence than any release with no title overlap.
func TestComputeReleaseConfidence_T1_AlbumMatchOutscoresNoMatch(t *testing.T) {
	match, _ := computeReleaseConfidence("Hi Infidelity", 0, "Hi Infidelity", "")
	noMatch, _ := computeReleaseConfidence("Hi Infidelity", 0, "Find Your Own Way Home", "")
	if match <= noMatch {
		t.Errorf("T1 violated: matching release (%.4f) must outscore non-matching (%.4f)", match, noMatch)
	}
}
