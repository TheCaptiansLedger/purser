package music_test

import (
	"context"
	"fmt"
	"math"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ============================================================================
// Shared test helpers
// ============================================================================

// scoreTolerance absorbs float64 summation-order noise — scoreFuzzy sums
// fuzzySignalWeights by ranging over a map, whose iteration order Go
// deliberately randomizes, so bit-for-bit equality against a
// hand-computed expected value isn't safe even though the formula itself
// is deterministic.
const scoreTolerance = 1e-9

func assertScore(t *testing.T, got, want float64, msgAndArgs string) {
	t.Helper()
	if math.Abs(got-want) > scoreTolerance {
		t.Errorf("%s: Score = %v, want %v", msgAndArgs, got, want)
	}
}

func candidate(ref string, tier domain.MatchTier, signals map[string]float64, releaseGroupMBID string) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: ref,
		Tier:        tier,
		Signals:     signals,
		Metadata:    map[string]any{"release_group_mbid": releaseGroupMBID},
	}
}

func candidateWithStatus(ref string, tier domain.MatchTier, signals map[string]float64, releaseGroupMBID, releaseStatus string) domain.MatchCandidate {
	c := candidate(ref, tier, signals, releaseGroupMBID)
	c.Metadata["release_status"] = releaseStatus
	return c
}

func scoreOne(t *testing.T, c domain.MatchCandidate) float64 {
	t.Helper()
	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{c})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ConfidenceScore() returned %d candidates, want 1", len(got))
	}
	return got[0].Score
}

// byExternalRef indexes a scored candidate list for lookup by ExternalRef —
// every end-to-end scenario below needs this since Identify()'s output
// order isn't a contract.
func byExternalRef(candidates []domain.MatchCandidate) map[string]domain.MatchCandidate {
	m := make(map[string]domain.MatchCandidate, len(candidates))
	for _, c := range candidates {
		m[c.ExternalRef] = c
	}
	return m
}

// ============================================================================
// Canonical fixture universe 1: REO Speedwagon, "Hi Infidelity" (1980) —
// single disc, 10 tracks, two known editions: the 1980 original
// ("rel-1980") and a 2004 reissue ("rel-2004"), one release group
// ("rg-hi-infidelity"). Every Section A scenario is a variation on this
// same underlying album — what changes, and why, is documented per test.
// ============================================================================

var hiInfidelityTitles = []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9", "T10"}

var hiInfidelityDurations = []float64{180, 190, 200, 210, 220, 230, 240, 250, 260, 270}

func hiInfidelityISRCs() []string {
	isrcs := make([]string, len(hiInfidelityTitles))
	for i := range isrcs {
		isrcs[i] = fmt.Sprintf("USRE18%05d", i+1)
	}
	return isrcs
}

// hiInfidelityTracks builds a candidate release's track listing. isrcs may
// be nil (no ISRC data on the candidate side).
func hiInfidelityTracks(titles []string, durations []float64, isrcs []string) []ports.Track {
	tracks := make([]ports.Track, len(titles))
	for i, title := range titles {
		tr := ports.Track{Position: i + 1, Title: title, Length: int(durations[i] * 1000)}
		if isrcs != nil && isrcs[i] != "" {
			tr.Recording = &ports.Recording{ISRCs: []string{isrcs[i]}}
		}
		tracks[i] = tr
	}
	return tracks
}

// hiInfidelityRelease builds one candidate edition. status/barcode/isrcs
// are per-edition — the whole point of most Section A scenarios is
// varying exactly one of these against an otherwise-identical release.
func hiInfidelityRelease(id, status, barcode string, isrcs []string) ports.Release {
	return ports.Release{
		ID:           id,
		Title:        "Hi Infidelity",
		Status:       status,
		Barcode:      barcode,
		Media:        []ports.Medium{{Position: 1, TrackCount: len(hiInfidelityTitles), Tracks: hiInfidelityTracks(hiInfidelityTitles, hiInfidelityDurations, isrcs)}},
		ArtistCredit: []ports.ArtistCredit{{Name: "REO Speedwagon"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-hi-infidelity", Title: "Hi Infidelity"},
	}
}

// newHiInfidelityMB registers both known editions (Official status, no
// barcode/ISRC set — individual tests add those where the scenario needs
// them) findable by the correct artist/album tag-fuzzy search.
func newHiInfidelityMB() *fakeMusicBrainz {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}, {ID: "rel-2004"}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)
	mb.releases["rel-2004"] = hiInfidelityRelease("rel-2004", "Official", "", nil)
	return mb
}

// hiInfidelityFingerprint builds the "everything correct" baseline
// Fingerprint — every scenario below starts here and mutates exactly what
// it's testing.
func hiInfidelityFingerprint(mutate func(*domain.Fingerprint)) domain.Fingerprint {
	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "REO Speedwagon", "ALBUM": "Hi Infidelity"},
		Metadata: map[string]any{
			"track_count":     len(hiInfidelityTitles),
			"track_titles":    append([]string(nil), hiInfidelityTitles...),
			"track_durations": append([]float64(nil), hiInfidelityDurations...),
			"track_isrcs":     make([]string, len(hiInfidelityTitles)),
		},
	}
	if mutate != nil {
		mutate(&fp)
	}
	return fp
}

func hiInfidelityFilenameParser() *fakeFilenameParser {
	return &fakeFilenameParser{artist: "REO Speedwagon", album: "Hi Infidelity", ok: true}
}

// ============================================================================
// Section A: Hi Infidelity — single-disc canonical example, run end to end
// through the real Identifier (M7) + ConfidenceScorer (M8), not hand-fed
// Signals. Each test states what SHOULD happen given the real signal
// contract (verified directly against identifier.go's signal functions,
// not assumed) before asserting it.
// ============================================================================

// A1. SHOULD: an embedded MUSICBRAINZ_ALBUMID that resolves and sanity-checks
// (track count roughly matches) short-circuits straight to Tier=direct_id,
// a single candidate, flat 0.98 — no fuzzy search, no scoring cascade.
func TestConfidenceScore_HiInfidelity_DirectID_EmbeddedReleaseMBID(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["embedded_release_mbids"] = []string{"rel-1980"}
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Tier != domain.MatchTierDirectID {
		t.Fatalf("Identify() = %+v, want exactly one direct_id candidate", candidates)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(scored) != 1 {
		t.Fatalf("ConfidenceScore() returned %d candidates, want 1", len(scored))
	}
	assertScore(t, scored[0].Score, 0.98, "direct ID via embedded release MBID")
}

// A2. SHOULD: an embedded MUSICBRAINZ_RELEASEGROUPID with exactly one
// release under it that sanity-checks also short-circuits to direct_id,
// 0.98 — the release-group fallback path, not just the release-ID path.
func TestConfidenceScore_HiInfidelity_DirectID_EmbeddedReleaseGroupMBID(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroups["rg-hi-infidelity"] = ports.ReleaseGroup{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}
	// directIDFromReleaseGroup sanity-checks the ListReleasesForReleaseGroup
	// stub directly (never a separately looked-up full release), so the
	// stub itself needs a real track count, not just an ID.
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980", Media: []ports.Medium{{TrackCount: len(hiInfidelityTitles)}}}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["embedded_releasegroup_mbids"] = []string{"rg-hi-infidelity"}
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Tier != domain.MatchTierDirectID {
		t.Fatalf("Identify() = %+v, want exactly one direct_id candidate", candidates)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	assertScore(t, scored[0].Score, 0.98, "direct ID via embedded release-group MBID")
}

// A3. SHOULD: an embedded release MBID whose track count is wildly wrong
// (an incomplete rip: 2 of 10 files present) fails releaseTrackCountSane's
// check and falls all the way through to the ordinary fuzzy cascade —
// never accepted as direct_id just because an ID happened to be embedded.
// Verified directly against titleSetSignal/durationSignal (identifier.go):
// their denominator is always OUR OWN track count, not the release's, so
// for the 2 tracks actually present they correctly read as a perfect
// 1.0 -- they are coverage fractions of what's on disk, not a
// completeness check against the full release. track_count_match is the
// ONLY signal that captures the incompleteness (2/10=0.2), and it does so
// correctly. The resulting score should be real but moderately (not
// catastrophically) degraded from a complete match, since track_count_match
// only carries 20% of fuzzy's weight.
func TestConfidenceScore_HiInfidelity_DirectID_InsaneTrackCountFallsThrough(t *testing.T) {
	mb := newHiInfidelityMB()

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "REO Speedwagon", "ALBUM": "Hi Infidelity"},
		Metadata: map[string]any{
			"embedded_release_mbids": []string{"rel-1980"},
			"track_count":            2, // only 2 files actually on disk
			"track_titles":           hiInfidelityTitles[:2],
			"track_durations":        hiInfidelityDurations[:2],
			"track_isrcs":            make([]string, 2),
		},
	}
	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	for _, c := range candidates {
		if c.Tier == domain.MatchTierDirectID {
			t.Fatalf("candidate %q Tier = direct_id, want the insane track-count sanity check to reject the direct-ID short-circuit", c.ExternalRef)
		}
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	c := got["rel-1980"]
	t.Logf("insane-track-count fallthrough: Score=%v Signals=%v", c.Score, c.Signals)
	if v := c.Signals["track_count_match"]; math.Abs(v-2.0/10.0) > scoreTolerance {
		t.Errorf("track_count_match = %v, want 2/10 = 0.2 (2 files present, release has 10)", v)
	}
	if v := c.Signals["title_set_overlap"]; v != 1.0 {
		t.Errorf("title_set_overlap = %v, want exactly 1.0 -- the denominator is our own 2-track count, and both present titles match perfectly", v)
	}
	// weighted = name(0.30*1.0) + track_count(0.20*0.2) + title(0.30*1.0)
	// + duration(0.20*1.0) = 0.30+0.04+0.30+0.20 = 0.84, sumWeight=1.0:
	// score = 0.45+0.84*0.30 = 0.702 -- real, measurable degradation from
	// a complete match's 0.75, but not dramatic, since track_count_match
	// (the only signal that sees the incompleteness) carries only 20% of
	// the weight. That's a deliberate property of the formula, not a bug:
	// this test exists to make it visible, not to assert a lower number
	// than the formula actually produces.
	assertScore(t, c.Score, 0.702, "insane direct-ID sanity check falls through to an honestly-partial fuzzy score")
}

// A4. SHOULD (continuation of docs/technical/music-identification.md's own
// worked example 1): no embedded ID, no barcode, no ISRC, tags fully
// correct -> fuzzy tier, both editions land near the top of fuzzy's band.
// Nothing about tags/durations distinguishes 1980 from 2004, so their raw
// scores tie exactly -- resolveReleaseGroups must break that tie
// deterministically (see the M9 gap this closes, confidence_score.go).
func TestConfidenceScore_HiInfidelity_FullyTagged_TwoEditions_TieBroken(t *testing.T) {
	mb := newHiInfidelityMB()
	// One track runs a few seconds long on this rip -- still within
	// duration_match's tolerance for 9 of 10 tracks, not 10.
	riderDurations := append([]float64(nil), hiInfidelityDurations...)
	riderDurations[4] += 15

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["track_durations"] = riderDurations
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity (1980)", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("Identify() returned %d candidates, want 2 (both editions)", len(candidates))
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	for ref, c := range got {
		t.Logf("HiInfidelity two-edition (%s): Score=%v Signals=%v", ref, c.Score, c.Signals)
		if c.Score <= 0.70 || c.Score > 0.75 {
			t.Errorf("candidate %q Score = %v, want within fuzzy's upper range (0.70, 0.75]", ref, c.Score)
		}
	}
	if got["rel-1980"].Score <= got["rel-2004"].Score {
		t.Fatalf("rel-1980.Score (%v) <= rel-2004.Score (%v), want a strict unique maximum -- Identify sorts MBIDs before scoring, so rel-1980 is first-seen and must win the deterministic tie-break", got["rel-1980"].Score, got["rel-2004"].Score)
	}
	if diff := got["rel-1980"].Score - got["rel-2004"].Score; diff > 0.01 {
		t.Errorf("rel-1980.Score - rel-2004.Score = %v, want a small deterministic tie-break margin, not a real quality difference", diff)
	}
}

// A5. SHOULD: a BARCODE tag matching exactly one edition's real barcode
// elevates that candidate to Tier=unique_id (classifyTier requires
// barcode_match>=1.0), base=0.80 (barcode alone -- no ISRC tags present).
func TestConfidenceScore_HiInfidelity_BarcodeOnly_UniqueIDBase(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releasesByBarcode["076742116824"] = []ports.Release{{ID: "rel-1980"}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "076742116824", nil)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Tags["BARCODE"] = "076742116824"
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	if got["rel-1980"].Tier != domain.MatchTierUniqueID {
		t.Fatalf("rel-1980 Tier = %q, want unique_id (barcode_match=1.0)", got["rel-1980"].Tier)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	// base=0.80 (barcode alone), corroboration=average(track_count=1.0,
	// title_set=1.0, duration=1.0)=1.0: 0.80 + 1.0*(0.95-0.80) = 0.95.
	assertScore(t, c.Score, 0.95, "barcode-only unique_id")
}

// A6. SHOULD: barcode AND ISRC both present and both resolving to the SAME
// edition raises unique_id's base to 0.90 (both agree), not just 0.80.
func TestConfidenceScore_HiInfidelity_BarcodeAndISRCAgree_HigherBase(t *testing.T) {
	isrcs := hiInfidelityISRCs()
	mb := newFakeMusicBrainz()
	mb.releasesByBarcode["076742116824"] = []ports.Release{{ID: "rel-1980"}}
	for _, isrc := range isrcs {
		mb.recordingsByISRC[isrc] = []ports.Recording{{ID: "rec-" + isrc, Releases: []ports.Release{{ID: "rel-1980"}}}}
	}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "076742116824", isrcs)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Tags["BARCODE"] = "076742116824"
		fp.Metadata["track_isrcs"] = isrcs
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	if c.Signals["barcode_match"] != 1.0 || c.Signals["isrc_consensus_fraction"] != 1.0 {
		t.Fatalf("Signals = %+v, want barcode_match=1.0 and isrc_consensus_fraction=1.0 (both agree)", c.Signals)
	}
	// base=0.90 (both agree), corroboration=1.0: 0.90+1.0*(0.95-0.90)=0.95.
	assertScore(t, c.Score, 0.95, "barcode+ISRC both agree")
}

// A7. SHOULD: ISRC tags present, no barcode tag at all -> unique_id,
// base=0.80 (ISRC-only base, same numeric base as barcode-only -- the
// doc's formula doesn't distinguish which single signal supplied it).
func TestConfidenceScore_HiInfidelity_ISRCOnly_UniqueIDBase(t *testing.T) {
	isrcs := hiInfidelityISRCs()
	mb := newFakeMusicBrainz()
	for _, isrc := range isrcs {
		mb.recordingsByISRC[isrc] = []ports.Recording{{ID: "rec-" + isrc, Releases: []ports.Release{{ID: "rel-1980"}}}}
	}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", isrcs)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["track_isrcs"] = isrcs
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	if _, ok := c.Signals["barcode_match"]; ok {
		t.Errorf("Signals has barcode_match = %v, want absent (no BARCODE tag)", c.Signals["barcode_match"])
	}
	assertScore(t, c.Score, 0.95, "ISRC-only unique_id (base 0.80 + full corroboration)")
}

// A8. SHOULD: a BARCODE tag present but wrong (matches no release in the
// catalog) must not corrupt or elevate scoring for the correct candidate
// found via tag search -- barcode_match evaluates to a real, evaluable
// 0.0 (non-match, not absent) on that candidate, but barcode_match is not
// part of fuzzy's weighted average at all, so the fuzzy score is
// unaffected. Tier also correctly stays fuzzy, never unique_id.
func TestConfidenceScore_HiInfidelity_WrongBarcode_DoesNotCorruptFuzzyScore(t *testing.T) {
	mb := newHiInfidelityMB()
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "076742116824", nil)
	// "000000000000" matches no release in the catalog.

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Tags["BARCODE"] = "000000000000"
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c1980 := got["rel-1980"]
	if c1980.Tier != domain.MatchTierFuzzy {
		t.Fatalf("rel-1980 Tier = %q, want fuzzy (wrong barcode must never elevate to unique_id)", c1980.Tier)
	}
	if v, ok := c1980.Signals["barcode_match"]; !ok || v != 0.0 {
		t.Fatalf("rel-1980 barcode_match = %v (ok=%v), want an evaluable 0.0 (real comparison, real mismatch)", v, ok)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	// Identical math to A4's per-edition raw score (name/track/title all
	// 1.0, duration 1.0 -- no rider-duration perturbation here):
	// weighted=1.0, score=0.45+1.0*0.30=0.75 raw. Two editions still tie
	// and get tie-broken, so rel-1980 (first-seen) keeps the full 0.75.
	assertScore(t, c.Score, 0.75, "wrong barcode does not corrupt the fuzzy score")
}

// A9. SHOULD: a wrong-but-non-blank ALBUM tag ("Nine Lives" instead of
// "Hi Infidelity") is NOT rescued by the filename fallback for SCORING
// purposes -- resolveNameQuery only falls back to the filename when tags
// are genuinely BLANK, never when they're merely wrong. But candidate
// COLLECTION is a separate, independently-gated path (falls back to
// filename whenever the tag-based search found nothing, wrong or blank
// alike) -- so the correct release is still found via the correctly-named
// folder, just scored against the wrong album name.
func TestConfidenceScore_HiInfidelity_WrongAlbumTag_NotRescuedByFilename(t *testing.T) {
	mb := newHiInfidelityMB()
	// Only the correct query is registered -- "REO Speedwagon|Nine Lives"
	// (the wrong tag) returns nothing from MusicBrainz, same as reality.

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, hiInfidelityFilenameParser())
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Tags["ALBUM"] = "Nine Lives" // wrong, but not blank
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	if _, ok := got["rel-1980"]; !ok {
		t.Fatalf("Identify() = %+v, want rel-1980 still found via the filename-fallback COLLECTION path despite the wrong tag", candidates)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	t.Logf("wrong album tag: Score=%v Signals=%v", c.Score, c.Signals)
	nameFuzzy, ok := c.Signals["name_fuzzy_score"]
	if !ok {
		t.Fatalf("name_fuzzy_score absent, want evaluable (SCORING used the wrong-but-present tag value, not the correct filename)")
	}
	if nameFuzzy >= 0.99 {
		t.Errorf("name_fuzzy_score = %v, want clearly degraded ('Nine Lives' vs 'Hi Infidelity' is not a near-match) -- if this is ~1.0, scoring accidentally used the filename instead of the wrong tag", nameFuzzy)
	}
	// track_count/title_set/duration are all still perfect (only the
	// album tag was corrupted), so the wrong-tag penalty is entirely
	// carried by name_fuzzy_score's degraded value.
	if v := c.Signals["title_set_overlap"]; v != 1.0 {
		t.Errorf("title_set_overlap = %v, want 1.0 (only the ALBUM tag was corrupted, not track titles)", v)
	}
}

// A10. SHOULD: fully BLANK tags with a correctly-named folder DOES get
// rescued by the filename fallback, for both collection and scoring --
// resolveNameQuery falls back exactly when tags are blank. But blank tags
// also mean no per-track TITLE data, so title_set_overlap is evaluable
// (M4 always emits one array slot per file, per fingerprinter.go) at a
// near-zero VALUE, not absent -- the absent-vs-zero distinction from
// ADR-0025 made concrete: this candidate should score meaningfully LOWER
// than A4's fully-tagged case despite name_fuzzy_score itself being just
// as strong, because title_set_overlap's real (low) value still gets
// averaged in, never excluded.
func TestConfidenceScore_HiInfidelity_BlankTags_FilenameRescues_ButTitlesStayZero(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, hiInfidelityFilenameParser())
	blankTitles := make([]string, len(hiInfidelityTitles)) // every TITLE tag missing
	fp := domain.Fingerprint{
		Tags: map[string]string{}, // ALBUMARTIST/ALBUM both blank
		Metadata: map[string]any{
			"track_count":     len(hiInfidelityTitles),
			"track_titles":    blankTitles,
			"track_durations": append([]float64(nil), hiInfidelityDurations...),
			"track_isrcs":     make([]string, len(hiInfidelityTitles)),
		},
	}

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c1980 := got["rel-1980"]
	if v, ok := c1980.Signals["name_fuzzy_score"]; !ok || v < 0.99 {
		t.Fatalf("name_fuzzy_score = %v (ok=%v), want ~1.0 -- filename fallback should rescue this to a near-perfect match", v, ok)
	}
	if v, ok := c1980.Signals["title_set_overlap"]; !ok {
		t.Fatalf("title_set_overlap absent, want evaluable-but-zero -- M4 always emits one title slot per file, blank when TITLE is missing, so this must never read as 'no data to check'")
	} else if v != 0.0 {
		t.Fatalf("title_set_overlap = %v, want exactly 0.0 (every candidate title compared against a blank string)", v)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-1980"]
	// weighted = name(0.30*~1.0) + track_count(0.20*1.0) + title(0.30*0.0)
	// + duration(0.20*1.0) = ~0.70, sumWeight=1.0 (all FOUR evaluated,
	// title included at zero value): rawAvg~=0.70, score~=0.45+0.70*0.30=0.66.
	// Coverage = 1.0 (full weight evaluated) -> the floor never triggers
	// even though the value itself is mediocre; that's the correct
	// behavior per the absent/evaluated-zero split.
	t.Logf("blank tags, filename-rescued: Score=%v Signals=%v", c.Score, c.Signals)
	if c.Score <= 0.60 || c.Score >= 0.70 {
		t.Errorf("Score = %v, want in (0.60, 0.70) -- clearly inside fuzzy's band but well below A4's fully-tagged ~0.744, dragged down specifically by title_set_overlap's real zero value", c.Score)
	}
}

// A11. SHOULD: tags blank AND the folder name is unparseable junk AND
// there's no barcode/ISRC AND AcoustID also finds nothing -- every
// collection path comes up empty, so Identify legitimately returns zero
// candidates. Nothing to score; ConfidenceScore on an empty list is a
// no-op, not an error.
func TestConfidenceScore_HiInfidelity_NothingFound_EmptyResult(t *testing.T) {
	mb := newFakeMusicBrainz() // nothing registered at all
	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{},
		Metadata: map[string]any{
			"track_count":     len(hiInfidelityTitles),
			"track_titles":    make([]string, len(hiInfidelityTitles)),
			"track_durations": append([]float64(nil), hiInfidelityDurations...),
			"track_isrcs":     make([]string, len(hiInfidelityTitles)),
		},
	}
	paths := []string{"/music/Unknown/01.flac"}
	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/000", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("Identify() = %+v, want empty (no tags, no filename guess, no barcode/ISRC, no AcoustID match)", candidates)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(scored) != 0 {
		t.Fatalf("ConfidenceScore() = %+v, want empty", scored)
	}
}

// A12. SHOULD: tags blank, folder junk, but AcoustID confidently resolves
// to a release whose MusicBrainz data is a bare stub (no track listing at
// all). track_count_match/title_set_overlap/duration_match are then all
// structurally absent (the CANDIDATE side has zero tracks, not our side),
// leaving fewer than two "agreeing" fuzzy signals -- classifyTier's
// fallback to Tier=acoustic fires, and it's scored on the capped
// sole-evidence band, never the full fuzzy band.
func TestConfidenceScore_HiInfidelity_AcoustIDSoleEvidence_StubRelease(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releasesByRG["rg-stub"] = []ports.Release{{ID: "rel-stub"}}
	mb.releases["rel-stub"] = ports.Release{
		ID:           "rel-stub",
		Title:        "Hi Infidelity (damaged rip, MB stub)",
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-stub", Title: "Hi Infidelity"},
		// Deliberately no Media/Tracks -- an incomplete MB entry.
	}
	paths := make([]string, len(hiInfidelityTitles))
	matches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-1",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-1", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-stub", Title: "Hi Infidelity"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for i := range paths {
		paths[i] = fmt.Sprintf("/music/000/%02d.flac", i+1)
		matchesByPath[paths[i]] = matches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{},
		Metadata: map[string]any{
			"track_count":     len(hiInfidelityTitles),
			"track_titles":    make([]string, len(hiInfidelityTitles)),
			"track_durations": append([]float64(nil), hiInfidelityDurations...),
			"track_isrcs":     make([]string, len(hiInfidelityTitles)),
		},
	}
	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/000", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c := got["rel-stub"]
	if c.Tier != domain.MatchTierAcoustic {
		t.Fatalf("rel-stub Tier = %q, want acoustic (stub release has no track data to corroborate on)", c.Tier)
	}
	if v := c.Signals["acoustic_agreement"]; v != 1.0 {
		t.Fatalf("acoustic_agreement = %v, want 1.0 (every file agreed)", v)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	assertScore(t, byExternalRef(scored)["rel-stub"].Score, 0.60, "acoustic sole evidence, perfect agreement, capped ceiling")
}

// A13. SHOULD: fully-tagged, no barcode/ISRC -> AcoustID's structural gate
// ("no unique-ID match found") still fires and runs. Since this candidate
// already qualifies for fuzzy on its own tag signals, a confident AcoustID
// match becomes ONE MORE weighted term in the fuzzy average (corroboration
// role, not sole evidence) -- the score should end up HIGHER than the
// no-AcoustID baseline (A4/A8's 0.75 raw), not just "also present."
func TestConfidenceScore_HiInfidelity_AcoustIDCorroborates(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)

	paths := make([]string, len(hiInfidelityTitles))
	matches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-1",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-1", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-hi-infidelity", Title: "Hi Infidelity"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for i := range paths {
		paths[i] = fmt.Sprintf("/music/REO Speedwagon/Hi Infidelity/%02d.flac", i+1)
		matchesByPath[paths[i]] = matches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(nil)

	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c := got["rel-1980"]
	if c.Tier != domain.MatchTierFuzzy {
		t.Fatalf("rel-1980 Tier = %q, want fuzzy (qualifies on its own tag signals; AcoustID here is corroboration, not sole evidence)", c.Tier)
	}
	if v := c.Signals["acoustic_agreement"]; v != 1.0 {
		t.Fatalf("acoustic_agreement = %v, want 1.0", v)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c = byExternalRef(scored)["rel-1980"]
	// weighted = 1.0(core, all 4 signals perfect) + 1.0*0.30(acoustic) =
	// 1.30, sumWeight = 1.0+0.30 = 1.30: rawAvg=1.0, score=0.45+1.0*0.30=
	// 0.75 -- ties the top of the band exactly, same numeric ceiling as
	// a perfect no-AcoustID match, since rawAvg was already a perfect 1.0
	// either way. The meaningful case (A14, below) is where the *value*
	// of acoustic_agreement isn't already 1.0.
	assertScore(t, c.Score, 0.75, "AcoustID corroboration folded into fuzzy average")
}

// A14. SHOULD: correct tags, no barcode/ISRC -> AcoustID structurally runs
// and CONTRADICTS (every file's fingerprint confidently resolves to some
// other, unrelated release group, never rg-hi-infidelity). That's not "one
// low signal" -- acoustic_agreement evaluates to exactly 0.0 with real
// data behind it, triggering the demotion multiplier on top of the
// (otherwise strong) weighted average. The result must land well below
// even fuzzy's own band floor, clearly distinguishable from "just no
// AcoustID data at all."
func TestConfidenceScore_HiInfidelity_AcoustIDContradicts(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}}
	mb.releases["rel-1980"] = hiInfidelityRelease("rel-1980", "Official", "", nil)

	paths := make([]string, len(hiInfidelityTitles))
	imposterMatches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-imposter",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-imposter", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-imposter", Title: "Some Unrelated Album"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for i := range paths {
		paths[i] = fmt.Sprintf("/music/REO Speedwagon/Hi Infidelity/%02d.flac", i+1)
		matchesByPath[paths[i]] = imposterMatches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())
	fp := hiInfidelityFingerprint(nil)

	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c := got["rel-1980"]
	if v, ok := c.Signals["acoustic_agreement"]; !ok || v != 0.0 {
		t.Fatalf("acoustic_agreement = %v (ok=%v), want an evaluable 0.0 (real AcoustID data, none of it agreeing)", v, ok)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c = byExternalRef(scored)["rel-1980"]
	// weighted = 1.0(core) + 0.0*0.30 = 1.0, sumWeight=1.30: rawAvg=1.0/1.30,
	// beforeDemotion = 0.45+(1.0/1.30)*0.30, demoted *0.5.
	rawAvg := 1.0 / 1.30
	want := (0.45 + rawAvg*0.30) * 0.5
	assertScore(t, c.Score, want, "AcoustID contradiction demotes an otherwise-strong fuzzy match")
	if c.Score >= 0.45 {
		t.Errorf("Score = %v, want well below fuzzy's own band floor (0.45) -- confident contradiction must cost more than a merely-absent signal would", c.Score)
	}
}

// A15. SHOULD: two editions where the Bootleg-status one happens to have a
// marginally HIGHER raw signal score than the Official one (e.g. a
// slightly better-preserved rip). The Official edition must still win as
// the release group's representative -- release_status is checked before
// raw score, not as a tiebreak after it.
func TestConfidenceScore_HiInfidelity_OfficialBeatsBootlegDespiteLowerRawScore(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-official"}, {ID: "rel-bootleg"}}
	mb.releases["rel-official"] = hiInfidelityRelease("rel-official", "Official", "", nil)
	mb.releases["rel-bootleg"] = hiInfidelityRelease("rel-bootleg", "Bootleg", "", nil)

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	// Official's rip has one track running long (imperfect duration_match);
	// the bootleg's rip happens to be pristine -- a strictly HIGHER raw
	// score for the bootleg, on purpose.
	riderDurations := append([]float64(nil), hiInfidelityDurations...)
	riderDurations[4] += 15
	fp := hiInfidelityFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["track_durations"] = riderDurations
	})

	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	t.Logf("official=%v bootleg=%v", got["rel-official"].Score, got["rel-bootleg"].Score)
	if got["rel-official"].Score <= got["rel-bootleg"].Score {
		t.Fatalf("official.Score (%v) <= bootleg.Score (%v), want Official to win the release group despite scoring lower on raw signals alone", got["rel-official"].Score, got["rel-bootleg"].Score)
	}
}

// ============================================================================
// Canonical fixture universe 2: Stevie Nicks, "Enhanced" [Box Set] -- 3
// discs, 32 tracks (12+12+8), release group "rg-enhanced". Continues
// docs/technical/music-identification.md's worked example 2.
// ============================================================================

func enhancedTitles() []string {
	titles := make([]string, 32)
	for i := range titles {
		titles[i] = "Track " + string(rune('A'+i%26))
	}
	return titles
}

func enhancedDurations() []float64 {
	durations := make([]float64, 32)
	for i := range durations {
		durations[i] = 200
	}
	return durations
}

func enhancedTracks(titles []string, durations []float64) []ports.Track {
	tracks := make([]ports.Track, len(titles))
	for i, title := range titles {
		tracks[i] = ports.Track{Position: i + 1, Title: title, Length: int(durations[i] * 1000)}
	}
	return tracks
}

func enhancedFingerprint(mutate func(*domain.Fingerprint)) domain.Fingerprint {
	titles, durations := enhancedTitles(), enhancedDurations()
	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "Stevie Nicks", "ALBUM": "Enhanced"},
		Metadata: map[string]any{
			"track_count":     32,
			"disc_count":      3,
			"track_titles":    titles,
			"track_durations": durations,
			"track_isrcs":     make([]string, 32),
		},
	}
	if mutate != nil {
		mutate(&fp)
	}
	return fp
}

func enhancedPaths() []string {
	paths := make([]string, 32)
	for i := range paths {
		paths[i] = fmt.Sprintf("/music/Stevie Nicks/Enhanced [Box Set]/f%02d.flac", i+1)
	}
	return paths
}

// B1. SHOULD (continuation of worked example 2): tags/DISCNUMBER fully
// correct across all 3 discs; a same-named-but-unrelated compilation also
// turns up via fuzzy name search. Track/medium-count signals -- not the
// name match -- are what actually separate the two: the real box set
// should clearly outscore the fake, with no ambiguity cap (the gap is
// large, not close).
func TestConfidenceScore_Enhanced_HappyPath_RealVsUnrelatedFake(t *testing.T) {
	titles := enhancedTitles()
	realTitles := append([]string(nil), titles...)
	realTitles[30] += " (Live)"
	realTitles[31] += " (Live)"

	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{
		{ID: "rg-real", Title: "Enhanced"},
		{ID: "rg-fake", Title: "Enhanced"},
	}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	mb.releasesByRG["rg-fake"] = []ports.Release{{ID: "rel-fake"}}

	realTracks := enhancedTracks(realTitles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}
	fakeTracks := make([]ports.Track, 11)
	for i := range fakeTracks {
		fakeTracks[i] = ports.Track{Position: i + 1, Title: "Unrelated Track", Length: 150000}
	}
	mb.releases["rel-fake"] = ports.Release{
		ID:           "rel-fake",
		Title:        "Enhanced",
		Media:        []ports.Medium{{Position: 1, TrackCount: 11, Tracks: fakeTracks}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Various Artists"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-fake", Title: "Enhanced"},
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := enhancedFingerprint(nil)
	candidates, err := identifier.Identify(context.Background(), fp, enhancedPaths(), "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	real, fake := got["rel-real"], got["rel-fake"]
	if real.ExternalRef == "" || fake.ExternalRef == "" {
		t.Fatalf("ConfidenceScore() = %+v, want both rel-real and rel-fake", scored)
	}
	t.Logf("Enhanced happy path: real=%v fake=%v", real.Score, fake.Score)
	if real.Score <= fake.Score {
		t.Errorf("real.Score (%v) <= fake.Score (%v), want the real box set to clearly outscore the unrelated compilation", real.Score, fake.Score)
	}
	if real.Score <= 0.50 {
		t.Errorf("real.Score = %v, want well above the ambiguity-cap ceiling (0.50) -- this pair is not ambiguous", real.Score)
	}
}

// B2. SHOULD: per ADR-0025 decision #5, when a multi-disc box set's
// subfolder split is genuinely ambiguous (no conclusive DISCNUMBER tags
// and no clean folder pattern), grouping is REQUIRED to leave subfolders
// split rather than guess a merge -- that's M3's job, not M8's, and is
// already covered by M3's own grouping tests. What THIS test proves at
// M8's layer is why that design choice is safe: a group that only
// contains one disc's worth of files (an accidental/conservative split)
// scores measurably, honestly lower than the complete box set (B1's
// ~0.744) -- "recoverable at review," not silently treated as a full
// match. It is NOT catastrophically low, and that's expected, not a bug:
// title_set_overlap/duration_match are coverage fractions of the GROUP'S
// OWN track count (verified directly against titleSetSignal/
// durationSignal), so the 12 tracks actually present still match
// perfectly on their own terms -- only track_count_match (weighted 20%)
// captures the incompleteness. The real signal a reviewer should look at
// is track_count_match itself reading a plainly-partial 12/32, not the
// aggregate Score alone.
func TestConfidenceScore_Enhanced_AccidentalSplitScoresHonestlyLow(t *testing.T) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{{ID: "rg-real", Title: "Enhanced"}}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	realTracks := enhancedTracks(titles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	// Only CD1's 12 tracks made it into this (accidentally split) group.
	fp := enhancedFingerprint(func(fp *domain.Fingerprint) {
		fp.Metadata["track_count"] = 12
		fp.Metadata["track_titles"] = titles[0:12]
		fp.Metadata["track_durations"] = enhancedDurations()[0:12]
		fp.Metadata["track_isrcs"] = make([]string, 12)
	})
	candidates, err := identifier.Identify(context.Background(), fp, enhancedPaths()[0:12], "/music/Stevie Nicks/Enhanced [Box Set]/CD1", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-real"]
	t.Logf("accidental CD1-only split: Score=%v Signals=%v", c.Score, c.Signals)
	if v := c.Signals["track_count_match"]; math.Abs(v-12.0/32.0) > scoreTolerance {
		t.Errorf("track_count_match = %v, want 12/32 = 0.375 (only CD1 present, release has all 32) -- this is the signal that actually exposes the incompleteness", v)
	}
	if v := c.Signals["title_set_overlap"]; v != 1.0 {
		t.Errorf("title_set_overlap = %v, want exactly 1.0 -- coverage fraction of our own 12 tracks, all of which genuinely match", v)
	}
	// weighted = name(0.30*1.0) + track_count(0.20*0.375) + title(0.30*1.0)
	// + duration(0.20*1.0) = 0.30+0.075+0.30+0.20 = 0.875, sumWeight=1.0:
	// score = 0.45+0.875*0.30 = 0.7125.
	assertScore(t, c.Score, 0.7125, "accidental single-disc split scores below the complete box set, not catastrophically")
	if c.Score >= 0.744 {
		t.Errorf("Score = %v, want strictly below the complete box set's ~0.744 (B1) -- some real degradation, even if not dramatic", c.Score)
	}
}

// B3. SHOULD: a genuinely close-scoring different release group (not the
// wildly-different fake from B1) DOES trigger the ambiguity cap. Built as
// a same-artist near-duplicate release group with a different disc split
// (11/11/8 instead of 12/12/8) -- plausible real MusicBrainz data quality
// noise (a regional variant mistakenly catalogued as its own release
// group), not a wildly unrelated compilation.
func TestConfidenceScore_Enhanced_CloseDifferentReleaseGroup_CapApplies(t *testing.T) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{
		{ID: "rg-real", Title: "Enhanced"},
		{ID: "rg-close", Title: "Enhanced"},
	}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	mb.releasesByRG["rg-close"] = []ports.Release{{ID: "rel-close"}}

	realTracks := enhancedTracks(titles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}
	// Same 32 titles/durations, same artist -- only the disc boundaries
	// differ (11/11/10 instead of 12/12/8), which shifts several
	// (medium,track)-position comparisons enough to matter but not enough
	// to be a landslide.
	closeTracks := enhancedTracks(titles, enhancedDurations())
	mb.releases["rel-close"] = ports.Release{
		ID:    "rel-close",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 11, Tracks: closeTracks[0:11]},
			{Position: 2, TrackCount: 11, Tracks: closeTracks[11:22]},
			{Position: 3, TrackCount: 10, Tracks: closeTracks[22:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-close", Title: "Enhanced"},
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := enhancedFingerprint(nil)
	candidates, err := identifier.Identify(context.Background(), fp, enhancedPaths(), "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	real, closeMatch := got["rel-real"], got["rel-close"]
	t.Logf("Enhanced close-groups: real=%v close=%v", real.Score, closeMatch.Score)
	if real.Score != 0.50 && closeMatch.Score != 0.50 {
		t.Fatalf("neither candidate was clamped to 0.50 (real=%v, close=%v) -- want the ambiguity cap to fire for this close a pair of different release groups", real.Score, closeMatch.Score)
	}
}

// B4. SHOULD: correct tags, no barcode/ISRC -> AcoustID's structural gate
// fires regardless (same as docs/technical/music-identification.md's own
// narrative for this exact album) and, agreeing with the real candidate,
// folds in as corroboration -- a measurable boost versus the no-AcoustID
// baseline (B1's real candidate).
// newEnhancedRealOnlyMB builds a fresh fake registering only the real box
// set (no unrelated compilation) — a separate instance per call, since
// fakeMusicBrainz tracks call counts and both the baseline and
// AcoustID-equipped runs below need independent ones.
func newEnhancedRealOnlyMB() (*fakeMusicBrainz, []string) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{{ID: "rg-real", Title: "Enhanced"}}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	// Same two-track "(Live)" variant as B1 -- deliberately NOT a perfect
	// title match, so tests that need headroom for a corroborating signal
	// to actually move the score (e.g. AcoustID agreement) have room to
	// show it; a rawAvg already pinned at 1.0 can't be pushed any higher.
	candidateTitles := append([]string(nil), titles...)
	candidateTitles[30] += " (Live)"
	candidateTitles[31] += " (Live)"
	realTracks := enhancedTracks(candidateTitles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}
	return mb, titles
}

func TestConfidenceScore_Enhanced_AcoustIDCorroborates(t *testing.T) {
	fp := enhancedFingerprint(nil)

	baselineMB, _ := newEnhancedRealOnlyMB()
	baselineIdentifier := music.NewIdentifier(baselineMB, &fakeAcoustID{}, noGuessFilenameParser())
	candidatesNoAcoustic, err := baselineIdentifier.Identify(context.Background(), fp, nil, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify (no AcoustID) returned error: %v", err)
	}
	baseline, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidatesNoAcoustic)
	if err != nil {
		t.Fatalf("ConfidenceScore (no AcoustID) returned error: %v", err)
	}
	baselineScore := byExternalRef(baseline)["rel-real"].Score

	mb, _ := newEnhancedRealOnlyMB()
	paths := enhancedPaths()
	matches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-1",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-1", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-real", Title: "Enhanced"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for _, p := range paths {
		matchesByPath[p] = matches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	corroborated := byExternalRef(scored)["rel-real"].Score

	t.Logf("Enhanced AcoustID corroboration: baseline=%v corroborated=%v", baselineScore, corroborated)
	if corroborated <= baselineScore {
		t.Errorf("corroborated Score (%v) <= no-AcoustID baseline (%v), want AcoustID agreement to fold in as a real boost", corroborated, baselineScore)
	}
}

// B5. SHOULD: correct tags, no barcode/ISRC -> AcoustID structurally runs
// (a hypothetical corrupted/mislabeled CD1 whose audio is actually
// something else entirely) and CONTRADICTS. Demotion applies the same way
// it does for Hi Infidelity (A14) -- the mechanism is generic across both
// canonical albums, not special-cased to either.
func TestConfidenceScore_Enhanced_AcoustIDContradicts(t *testing.T) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{{ID: "rg-real", Title: "Enhanced"}}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	realTracks := enhancedTracks(titles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}

	paths := enhancedPaths()
	imposterMatches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-imposter",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-imposter", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-imposter", Title: "Some Unrelated Album"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for _, p := range paths {
		matchesByPath[p] = imposterMatches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())
	fp := enhancedFingerprint(nil)

	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	if v, ok := got["rel-real"].Signals["acoustic_agreement"]; !ok || v != 0.0 {
		t.Fatalf("acoustic_agreement = %v (ok=%v), want an evaluable 0.0", v, ok)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	c := byExternalRef(scored)["rel-real"]
	t.Logf("Enhanced AcoustID contradiction: Score=%v", c.Score)
	if c.Score >= 0.45 {
		t.Errorf("Score = %v, want well below fuzzy's own band floor (0.45) after contradiction demotion", c.Score)
	}
}

// B6. SHOULD: blank tags AND an unparseable folder name -- name_fuzzy_score
// is absent (haveNameQuery false), but track_count_match/title_set_overlap/
// duration_match all remain evaluable (the candidate's own Media has real
// track data). Coverage = 0.70 of a possible 1.0 -- ABOVE the 50% floor
// threshold, so the floor itself does not trigger here; the score is kept
// honest purely by title_set_overlap's real near-zero value, the same
// mechanism as A10. This documents precisely why "untagged" alone doesn't
// always hit the floor -- see the synthetic Section C test for the case
// that actually does.
func TestConfidenceScore_Enhanced_BlankTagsJunkFolder_FloorDoesNotFire(t *testing.T) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	realTracks := enhancedTracks(titles, enhancedDurations())
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}
	// This candidate is only reachable via AcoustID here (no tags, no
	// filename guess, no barcode/ISRC), so it must resolve confidently for
	// the release to be collected at all.
	paths := enhancedPaths()
	matches := []ports.AcoustIDMatch{{
		AcoustID:   "aid-1",
		Recordings: []ports.AcoustIDRecording{{MBID: "rec-1", ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-real", Title: "Enhanced"}}}},
	}}
	matchesByPath := map[string][]ports.AcoustIDMatch{}
	for _, p := range paths {
		matchesByPath[p] = matches
	}
	acoustID := &fakeAcoustID{matchesByPath: matchesByPath}
	identifier := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{},
		Metadata: map[string]any{
			"track_count":     32,
			"disc_count":      3,
			"track_titles":    make([]string, 32),
			"track_durations": enhancedDurations(),
			"track_isrcs":     make([]string, 32),
		},
	}
	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/000", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	got := byExternalRef(candidates)
	c := got["rel-real"]
	if _, ok := c.Signals["name_fuzzy_score"]; ok {
		t.Fatalf("name_fuzzy_score present, want absent (no tags, no filename guess)")
	}
	if v, ok := c.Signals["title_set_overlap"]; !ok || v != 0.0 {
		t.Fatalf("title_set_overlap = %v (ok=%v), want evaluable-but-zero", v, ok)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	scoredC := byExternalRef(scored)["rel-real"]
	// weighted = track_count(0.20*1.0) + title(0.30*0.0) + duration(0.20*1.0)
	// + acoustic(0.30*1.0) = 0.20+0+0.20+0.30 = 0.70, sumWeight = 0.20+0.30+
	// 0.20+0.30 = 1.0 (name_fuzzy_score's 0.30 excluded, everything else
	// including acoustic evaluated): coverage = 1.0/1.0 = 1.0 -- full
	// coverage of what's structurally possible once name is absent, so the
	// floor (0.5 threshold) does NOT trigger. rawAvg=0.70, score=
	// 0.45+0.70*0.30=0.66.
	t.Logf("Enhanced blank tags + AcoustID: Score=%v Signals=%v", scoredC.Score, scoredC.Signals)
	if scoredC.Score <= 0.60 || scoredC.Score >= 0.70 {
		t.Errorf("Score = %v, want in (0.60, 0.70) -- degraded by title_set_overlap's zero value alone, not by the coverage floor", scoredC.Score)
	}
}

// B7. SHOULD: two editions of the same box set (e.g. the original release
// vs. a "Deluxe Edition" repackage) that both fully match on tags land on
// an exact raw tie, the same mechanism as Hi Infidelity's A4 -- proving
// the tie-break mechanism is generic across single-disc and multi-disc
// albums alike, not something that happens to work for one and not the
// other.
func TestConfidenceScore_Enhanced_TwoEditions_TieBroken(t *testing.T) {
	titles := enhancedTitles()
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{{ID: "rg-real", Title: "Enhanced"}}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-original"}, {ID: "rel-deluxe"}}
	for _, id := range []string{"rel-original", "rel-deluxe"} {
		tracks := enhancedTracks(titles, enhancedDurations())
		mb.releases[id] = ports.Release{
			ID:    id,
			Title: "Enhanced",
			Media: []ports.Medium{
				{Position: 1, TrackCount: 12, Tracks: tracks[0:12]},
				{Position: 2, TrackCount: 12, Tracks: tracks[12:24]},
				{Position: 3, TrackCount: 8, Tracks: tracks[24:32]},
			},
			ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
			ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
		}
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := enhancedFingerprint(nil)
	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("Identify() returned %d candidates, want 2", len(candidates))
	}
	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)
	// "rel-deluxe" < "rel-original" alphabetically -> rel-deluxe is
	// first-seen after Identify's MBID sort, so it wins the tie-break.
	if got["rel-deluxe"].Score <= got["rel-original"].Score {
		t.Fatalf("rel-deluxe.Score (%v) <= rel-original.Score (%v), want a strict unique maximum within the release group", got["rel-deluxe"].Score, got["rel-original"].Score)
	}
}

// ============================================================================
// Section C: cross-cutting formula-level tests. These are deliberately
// synthetic (hand-fed Signals, not run through Identify()) -- either
// because the scenario doesn't map onto either canonical album (a Various
// Artists compilation isn't Stevie Nicks or REO Speedwagon), or because
// it's testing a signal COMBINATION that today's exact M7 signal-emptiness
// rules can't actually produce end to end (documented explicitly where
// that's the case), but the generic formula must still hold for it.
// ============================================================================

func TestConfidenceScore_ContentTypes(t *testing.T) {
	s := music.NewConfidenceScorer()
	got := s.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestConfidenceScore_EmptyCandidatesReturnsEmpty(t *testing.T) {
	s := music.NewConfidenceScorer()
	got, err := s.ConfidenceScore(context.Background(), domain.Fingerprint{}, nil)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ConfidenceScore() = %+v, want empty", got)
	}
}

func TestConfidenceScore_DirectID_FlatScore(t *testing.T) {
	c := candidate("rel-direct", domain.MatchTierDirectID, map[string]float64{}, "rg-direct")
	got := scoreOne(t, c)
	assertScore(t, got, 0.98, "direct_id")
}

func TestConfidenceScore_AcousticSoleEvidence_Capped(t *testing.T) {
	c := candidate("rel-acoustic", domain.MatchTierAcoustic, map[string]float64{
		"acoustic_agreement": 0.8,
	}, "rg-acoustic")
	got := scoreOne(t, c)
	// Sole evidence, capped band: 0.8*0.60 = 0.48 — below the tier's
	// 0.60 ceiling even at fairly strong agreement, since audio
	// fingerprinting alone is less trustworthy than corroborated tags.
	assertScore(t, got, 0.48, "acoustic sole evidence")
}

// SHOULD: the issue's own illustrative case ("only track-count and
// duration are evaluable") requires name_fuzzy_score AND title_set_overlap
// BOTH absent simultaneously. Verified against identifier.go directly:
// titleSetSignal and durationSignal share the exact same
// len(flattenReleaseTracks(release))==0 guard on the candidate side, so in
// practice they can only be absent TOGETHER (see Section A/B's real
// end-to-end tests, where an untagged candidate's title_set_overlap is
// always evaluable-but-zero, never literally absent, whenever the
// candidate release has real track data). This test proves the FLOOR
// MECHANISM ITSELF holds generically for the exact signal combination the
// issue describes, independent of whether today's M7 can produce it --
// the mechanism must not be a source-based special case, per the issue,
// and this is what "generic" is actually verified against.
func TestConfidenceScore_CoverageFloor_FormulaProof_OnlyTrackCountAndDuration(t *testing.T) {
	c := candidate("rel-synthetic-sparse", domain.MatchTierFuzzy, map[string]float64{
		"track_count_match": 1.0,
		"duration_match":    1.0,
	}, "rg-synthetic-sparse")

	got := scoreOne(t, c)
	// Uncapped this would be 0.45 + 1.0*0.30 = 0.75 (top of band).
	// Evaluated weight = 0.40 of a possible 1.0 (40% coverage, below the
	// 50% floor), so it clamps to the band's midpoint instead:
	// 0.45 + (0.75-0.45)/2 = 0.60.
	assertScore(t, got, 0.60, "coverage-floor formula proof")
	const bandMidpoint = 0.60
	if got >= bandMidpoint+scoreTolerance {
		t.Errorf("Score = %v, want strictly at or below the band midpoint guard %v", got, bandMidpoint)
	}
}

func TestConfidenceScore_CoverageFloor_UniqueIDSparseCorroboration(t *testing.T) {
	c := candidate("rel-sparse-unique", domain.MatchTierUniqueID, map[string]float64{
		"isrc_consensus_fraction": 0.75,
		"track_count_match":       1.0, // only 1 of 3 corroboration signals evaluable
	}, "rg-sparse-unique")
	got := scoreOne(t, c)
	// Uncapped: base=0.80 (ISRC alone), corroboration=average(1.0)=1.0:
	// 0.80 + 1.0*0.15 = 0.95. Coverage = 1/3 < 0.5, so it clamps to the
	// band midpoint instead: 0.80 + (0.95-0.80)/2 = 0.875.
	assertScore(t, got, 0.875, "unique_id sparse corroboration")
}

// SHOULD: a Various Artists compilation doesn't map onto either canonical
// single-artist album (REO Speedwagon, Stevie Nicks) -- kept as an
// explicitly-labeled generic fixture rather than forcing a false
// narrative onto either.
func TestConfidenceScore_VariousArtistsCompilation(t *testing.T) {
	c := candidate("rel-va", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.85, // VA sentinel artist name fuzzy-matches less precisely
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-va")
	got := scoreOne(t, c)
	// weighted sum = 0.30*0.85 + 0.20*1.0 + 0.30*1.0 + 0.20*1.0 = 0.955,
	// sumWeight = 1.0: 0.45 + 0.955*0.30 = 0.7365.
	assertScore(t, got, 0.7365, "Various Artists compilation")
}

func TestConfidenceScore_AmbiguityCap_UsesUpdatedClusterTop(t *testing.T) {
	weak := candidate("rel-a-weak", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.5,
		"track_count_match": 0.5,
		"title_set_overlap": 0.5,
		"duration_match":    0.5,
	}, "rg-multi")
	strong := candidate("rel-a-strong", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-multi")
	otherGroup := candidate("rel-b", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.95,
		"track_count_match": 1.0,
		"title_set_overlap": 0.95,
		"duration_match":    0.9,
	}, "rg-other")

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{weak, strong, otherGroup})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	byRef := byExternalRef(got)
	assertScore(t, byRef["rel-a-weak"].Score, 0.60, "weak (untouched, not the cluster's top)")
	assertScore(t, byRef["rel-a-strong"].Score, 0.50, "strong (cluster's actual top, clamped)")
	assertScore(t, byRef["rel-b"].Score, 0.735, "other group (uncapped runner-up)")
}

func TestConfidenceScore_EditionTieBreak_PrefersOfficialStatusOverRawScore(t *testing.T) {
	official := candidateWithStatus("rel-official", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.9,
		"track_count_match": 1.0,
		"title_set_overlap": 0.9,
		"duration_match":    0.9,
	}, "rg-status", "Official")
	bootleg := candidateWithStatus("rel-bootleg", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-status", "Bootleg")

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{official, bootleg})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	byRef := byExternalRef(got)
	assertScore(t, byRef["rel-official"].Score, 0.726, "official edition (representative, untouched)")
	if byRef["rel-bootleg"].Score >= byRef["rel-official"].Score {
		t.Fatalf("bootleg.Score (%v) >= official.Score (%v), want the official edition to win despite its lower raw signal agreement", byRef["rel-bootleg"].Score, byRef["rel-official"].Score)
	}
}

func TestConfidenceScore_TwoDifferentReleaseGroupsClose_FormulaProof(t *testing.T) {
	strong := candidate("rel-7a", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    0.9,
	}, "rg-7a")
	closeRunnerUp := candidate("rel-7b", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.95,
		"track_count_match": 1.0,
		"title_set_overlap": 0.95,
		"duration_match":    0.9,
	}, "rg-7b")

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{strong, closeRunnerUp})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	byRef := byExternalRef(got)
	assertScore(t, byRef["rel-7a"].Score, 0.50, "ambiguity-clamped winner")
	assertScore(t, byRef["rel-7b"].Score, 0.735, "uncapped runner-up")
}

func TestConfidenceScore_AcoustIDContradictsStrongTagSignals_FormulaProof(t *testing.T) {
	c := candidate("rel-8", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":   1.0,
		"track_count_match":  1.0,
		"title_set_overlap":  1.0,
		"duration_match":     1.0,
		"acoustic_agreement": 0.0,
	}, "rg-8")
	got := scoreOne(t, c)
	rawAvg := 1.0 / 1.30
	want := (0.45 + rawAvg*0.30) * 0.5
	assertScore(t, got, want, "AcoustID contradiction formula proof")
}

// --- Differentiation assertion: a strong fuzzy match must score at least
// 0.15 above a weak/wrong fuzzy match through the same code path. This is
// the actual regression guard against the convergence bug recurring —
// without it, a future change could reintroduce convergent scoring and
// every other test here would still pass. ---
func TestConfidenceScore_Differentiation_StrongVsWeakFuzzy(t *testing.T) {
	strong := candidate("rel-strong", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-strong")
	weak := candidate("rel-weak", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.2,
		"track_count_match": 0.3,
		"title_set_overlap": 0.1,
		"duration_match":    0.4,
	}, "rg-weak")

	strongScore := scoreOne(t, strong)
	weakScore := scoreOne(t, weak)

	const minMargin = 0.15
	if margin := strongScore - weakScore; margin < minMargin {
		t.Fatalf("strong (%v) - weak (%v) = %v, want >= %v (differentiation assertion — convergence-bug regression guard)", strongScore, weakScore, margin, minMargin)
	}
}
