package music_test

import (
	"context"
	"math"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

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

func TestConfidenceScore_AcousticSoleEvidence_PerfectAgreementHitsCeiling(t *testing.T) {
	c := candidate("rel-acoustic-perfect", domain.MatchTierAcoustic, map[string]float64{
		"acoustic_agreement": 1.0,
	}, "rg-acoustic-perfect")

	got := scoreOne(t, c)
	assertScore(t, got, 0.60, "acoustic sole evidence at maximum agreement")
}

// --- Fixture 1: perfect tags + barcode (unique_id, both-agree base). ---

func TestConfidenceScore_PerfectTagsPlusBarcode(t *testing.T) {
	c := candidate("rel-1", domain.MatchTierUniqueID, map[string]float64{
		"barcode_match":           1.0,
		"isrc_consensus_fraction": 1.0,
		"track_count_match":       1.0,
		"title_set_overlap":       1.0,
		"duration_match":          1.0,
	}, "rg-1")

	got := scoreOne(t, c)
	// base=0.90 (barcode+ISRC both agree), corroboration=1.0 (all three
	// present, all perfect): 0.90 + 1.0*(0.95-0.90) = 0.95.
	assertScore(t, got, 0.95, "perfect tags+barcode")
}

// --- Fixture 2: missing barcode, present ISRC (unique_id, ISRC-only base). ---

func TestConfidenceScore_ISRCOnly(t *testing.T) {
	c := candidate("rel-2", domain.MatchTierUniqueID, map[string]float64{
		"isrc_consensus_fraction": 0.75,
		"track_count_match":       1.0,
		"title_set_overlap":       0.9,
		"duration_match":          0.8,
	}, "rg-2")

	got := scoreOne(t, c)
	// base=0.80 (ISRC alone, no barcode_match at all), corroboration =
	// average(1.0, 0.9, 0.8) = 0.9: 0.80 + 0.9*(0.95-0.80) = 0.935.
	assertScore(t, got, 0.935, "ISRC-only")
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

// --- Fixture 3: correct tracks, wrong/missing album tag (fuzzy, exercises
// the evaluable-signals-only normalization directly — the actual
// convergence-bug fix). ---

func TestConfidenceScore_WrongMissingAlbumTag(t *testing.T) {
	c := candidate("rel-3", domain.MatchTierFuzzy, map[string]float64{
		// name_fuzzy_score deliberately absent: the album tag didn't
		// resolve to a usable name query at all, not merely mismatched.
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-3")

	got := scoreOne(t, c)
	// Evaluated weight = 0.20+0.30+0.20 = 0.70 (name_fuzzy_score's 0.30
	// excluded entirely, not treated as zero). rawAvg over what WAS
	// evaluated = 1.0 (all three perfect): 0.45 + 1.0*(0.75-0.45) = 0.75
	// — the top of the band, despite 30% of possible weight having no
	// data. v1's bug would have normalized against the full weight
	// instead (0.45 + 0.70*0.30 = 0.66) and silently dragged every
	// under-tagged-but-otherwise-perfect candidate down; this is the
	// assertion that guards against that regressing.
	assertScore(t, got, 0.75, "wrong/missing album tag")
}

// --- Fixture 4: multi-disc release. ---

func TestConfidenceScore_MultiDisc(t *testing.T) {
	c := candidate("rel-4", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 0.95,
		"duration_match":    1.0,
	}, "rg-4")

	got := scoreOne(t, c)
	// weighted sum = 0.30*1.0 + 0.20*1.0 + 0.30*0.95 + 0.20*1.0 = 0.985,
	// sumWeight = 1.0: 0.45 + 0.985*0.30 = 0.7455.
	assertScore(t, got, 0.7455, "multi-disc")
}

// --- Fixture 5: Various Artists compilation. ---

func TestConfidenceScore_VariousArtistsCompilation(t *testing.T) {
	c := candidate("rel-5", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.85, // VA sentinel artist name fuzzy-matches less precisely
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-5")

	got := scoreOne(t, c)
	// weighted sum = 0.30*0.85 + 0.20*1.0 + 0.30*1.0 + 0.20*1.0 = 0.955,
	// sumWeight = 1.0: 0.45 + 0.955*0.30 = 0.7365.
	assertScore(t, got, 0.7365, "Various Artists compilation")
}

// --- Fixture 6: two near-duplicate editions of the same release group —
// must NOT trigger the ambiguity cap. ---

func TestConfidenceScore_NearDuplicateEditionsSameReleaseGroup_NoCap(t *testing.T) {
	signals := map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    0.9,
	}
	c1 := candidate("rel-6a", domain.MatchTierFuzzy, signals, "rg-6")
	c2 := candidate("rel-6b", domain.MatchTierFuzzy, signals, "rg-6")

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{c1, c2})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ConfidenceScore() returned %d candidates, want 2", len(got))
	}
	// weighted sum = 0.30+0.20+0.30+0.18 = 0.98, sumWeight=1.0:
	// 0.45 + 0.98*0.30 = 0.744. Same release group -> single cluster ->
	// the ambiguity cap structurally cannot fire (needs >=2 clusters), so
	// both editions keep this uncapped score independently.
	for _, c := range got {
		assertScore(t, c.Score, 0.744, "near-duplicate edition "+c.ExternalRef)
	}
}

// --- Fixture 7: two different release groups scoring close — must
// trigger the ambiguity cap. ---

func TestConfidenceScore_TwoDifferentReleaseGroupsClose_CapApplies(t *testing.T) {
	strong := candidate("rel-7a", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    0.9,
	}, "rg-7a")
	// weighted sum = 0.98, score = 0.744 (same math as fixture 6).

	closeRunnerUp := candidate("rel-7b", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.95,
		"track_count_match": 1.0,
		"title_set_overlap": 0.95,
		"duration_match":    0.9,
	}, "rg-7b")
	// weighted sum = 0.30*0.95+0.20*1.0+0.30*0.95+0.20*0.9 = 0.285+0.2+0.285+0.18=0.95,
	// score = 0.45+0.95*0.30 = 0.735. Margin from strong's 0.744 is 0.009 < 0.10.

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{strong, closeRunnerUp})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}

	var winner, runnerUp domain.MatchCandidate
	for _, c := range got {
		switch c.ExternalRef {
		case "rel-7a":
			winner = c
		case "rel-7b":
			runnerUp = c
		}
	}
	assertScore(t, winner.Score, 0.50, "ambiguity-clamped winner")
	assertScore(t, runnerUp.Score, 0.735, "uncapped runner-up")
}

func TestConfidenceScore_AmbiguityCap_UsesUpdatedClusterTop(t *testing.T) {
	weak := candidate("rel-a-weak", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.5,
		"track_count_match": 0.5,
		"title_set_overlap": 0.5,
		"duration_match":    0.5,
	}, "rg-multi")
	// score = 0.45 + 0.5*0.30 = 0.60.

	strong := candidate("rel-a-strong", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  1.0,
		"track_count_match": 1.0,
		"title_set_overlap": 1.0,
		"duration_match":    1.0,
	}, "rg-multi") // same release group as weak, scores higher -> becomes the cluster's top.
	// score = 0.45 + 1.0*0.30 = 0.75.

	otherGroup := candidate("rel-b", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":  0.95,
		"track_count_match": 1.0,
		"title_set_overlap": 0.95,
		"duration_match":    0.9,
	}, "rg-other")
	// score = 0.45 + 0.95*0.30 = 0.735. Margin from strong's 0.75 is
	// 0.015 < 0.10 -> cap applies to whichever candidate is actually the
	// rg-multi cluster's top (strong, listed second) — not weak, listed
	// first, which the "topIndex" bookkeeping must have updated away
	// from.

	scorer := music.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{weak, strong, otherGroup})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}

	byRef := map[string]domain.MatchCandidate{}
	for _, c := range got {
		byRef[c.ExternalRef] = c
	}
	assertScore(t, byRef["rel-a-weak"].Score, 0.60, "weak (untouched, not the cluster's top)")
	assertScore(t, byRef["rel-a-strong"].Score, 0.50, "strong (cluster's actual top, clamped)")
	assertScore(t, byRef["rel-b"].Score, 0.735, "other group (uncapped runner-up)")
}

// --- Fixture 8: AcoustID contradicts otherwise-strong tag signals —
// exercises the demotion multiplier. ---

func TestConfidenceScore_AcoustIDContradictsStrongTagSignals(t *testing.T) {
	c := candidate("rel-8", domain.MatchTierFuzzy, map[string]float64{
		"name_fuzzy_score":   1.0,
		"track_count_match":  1.0,
		"title_set_overlap":  1.0,
		"duration_match":     1.0,
		"acoustic_agreement": 0.0, // AcoustID ran, confidently resolved elsewhere
	}, "rg-8")

	got := scoreOne(t, c)
	// weighted sum = 1.0 (core) + 0.0*0.30 (acoustic) = 1.0,
	// sumWeight = 1.0+0.30 = 1.30. rawAvg = 1.0/1.30. score before
	// demotion = 0.45 + (1.0/1.30)*0.30. Demotion multiplies by 0.5.
	rawAvg := 1.0 / 1.30
	beforeDemotion := 0.45 + rawAvg*0.30
	want := beforeDemotion * 0.5
	assertScore(t, got, want, "AcoustID contradiction")
	if got >= 0.45 {
		t.Errorf("Score = %v, want well below fuzzy's own band floor (0.45) — a confident contradiction must cost more than one absent signal would", got)
	}
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

// --- New fixture (issue #516's coverage-floor requirement): an
// untagged/filename-only candidate with only track-count and duration
// evaluable must not reach auto-import-eligible territory — asserted here
// as "does not reach the top half of fuzzy's band", the generic mechanism
// the issue asks for. The maintainer reviews this actual number, not just
// pass/fail. ---

func TestConfidenceScore_CoverageFloor_FilenameOnlyCandidate(t *testing.T) {
	c := candidate("rel-9", domain.MatchTierFuzzy, map[string]float64{
		// Only signals derivable without any tag data at all: track count
		// from the file listing, duration from the audio itself.
		"track_count_match": 1.0,
		"duration_match":    1.0,
	}, "rg-9")

	got := scoreOne(t, c)
	// Uncapped this would be 0.45 + 1.0*0.30 = 0.75 (top of band) —
	// exactly the auto-import-adjacent number ADR-0025's decision #1
	// forbids a filename-only candidate from reaching. Evaluated weight
	// = 0.40 of a possible 1.0 (40% coverage, below the 50% floor), so it
	// clamps to the band's midpoint instead: 0.45 + (0.75-0.45)/2 = 0.60.
	assertScore(t, got, 0.60, "coverage-floor filename-only candidate")
	if got >= fuzzyBandMidpointForTest {
		t.Errorf("Score = %v, want strictly below the band midpoint guard %v", got, fuzzyBandMidpointForTest)
	}
}

// fuzzyBandMidpointForTest mirrors the production fuzzyBandLow/fuzzyBandHigh
// midpoint (0.45, 0.75) without importing an unexported constant across
// packages — kept local to this test file.
const fuzzyBandMidpointForTest = 0.60 + scoreTolerance

// --- Worked-example continuations (M7 -> M8 checkpoint): rerun M7's own
// fixtures through ConfidenceScore end to end. The maintainer reviews
// these two final numbers directly, per the issue's verification
// checklist. ---

func TestConfidenceScore_WorkedExample1_HiInfidelity(t *testing.T) {
	titles := []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9", "T10"}
	durations := []float64{180, 190, 200, 210, 220, 230, 240, 250, 260, 270}
	riderDurations := append([]float64(nil), durations...)
	riderDurations[4] += 15

	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}, {ID: "rel-2004"}}

	makeTracks := func() []ports.Track {
		tracks := make([]ports.Track, len(titles))
		for i, title := range titles {
			tracks[i] = ports.Track{Position: i + 1, Title: title, Length: int(durations[i] * 1000)}
		}
		return tracks
	}
	for _, id := range []string{"rel-1980", "rel-2004"} {
		mb.releases[id] = ports.Release{
			ID:           id,
			Title:        "Hi Infidelity",
			Media:        []ports.Medium{{Position: 1, TrackCount: len(titles), Tracks: makeTracks()}},
			ArtistCredit: []ports.ArtistCredit{{Name: "REO Speedwagon"}},
			ReleaseGroup: &ports.ReleaseGroup{ID: "rg-hi-infidelity", Title: "Hi Infidelity"},
		}
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())
	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "REO Speedwagon", "ALBUM": "Hi Infidelity"},
		Metadata: map[string]any{
			"track_count":     10,
			"track_titles":    titles,
			"track_durations": riderDurations,
			"track_isrcs":     make([]string, 10),
		},
	}
	candidates, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity (1980)", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(scored) != 2 {
		t.Fatalf("ConfidenceScore() returned %d candidates, want 2 (both editions)", len(scored))
	}
	for _, c := range scored {
		t.Logf("worked example 1 (%s): Score=%v Signals=%v", c.ExternalRef, c.Score, c.Signals)
		if c.Score <= 0.70 || c.Score > 0.75 {
			t.Errorf("candidate %q Score = %v, want within fuzzy's upper range (0.70, 0.75] given near-perfect signal agreement", c.ExternalRef, c.Score)
		}
	}
	// Same release group -> not ambiguity by construction: both editions
	// keep their own independently computed score.
	if scored[0].Score != scored[1].Score {
		t.Errorf("edition scores differ (%v vs %v) despite identical signals — same release group must not trigger the ambiguity cap", scored[0].Score, scored[1].Score)
	}
}

func TestConfidenceScore_WorkedExample2_StevieNicksBoxSet(t *testing.T) {
	groupTitles := make([]string, 32)
	groupDurations := make([]float64, 32)
	for i := range groupTitles {
		groupTitles[i] = "Track " + string(rune('A'+i%26))
		groupDurations[i] = 200
	}
	realTitles := append([]string(nil), groupTitles...)
	realTitles[30] = groupTitles[30] + " (Live)"
	realTitles[31] = groupTitles[31] + " (Live)"

	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{
		{ID: "rg-real", Title: "Enhanced"},
		{ID: "rg-fake", Title: "Enhanced"},
	}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	mb.releasesByRG["rg-fake"] = []ports.Release{{ID: "rel-fake"}}

	realTracks := make([]ports.Track, 32)
	for i, title := range realTitles {
		realTracks[i] = ports.Track{Position: i + 1, Title: title, Length: int(groupDurations[i] * 1000)}
	}
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
	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "Stevie Nicks", "ALBUM": "Enhanced"},
		Metadata: map[string]any{
			"track_count":     32,
			"disc_count":      3,
			"track_titles":    groupTitles,
			"track_durations": groupDurations,
			"track_isrcs":     make([]string, 32),
		},
	}
	paths := make([]string, 32)
	for i := range paths {
		paths[i] = "/music/Stevie Nicks/Enhanced/f.flac"
	}
	candidates, err := identifier.Identify(context.Background(), fp, paths, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}

	scored, err := music.NewConfidenceScorer().ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}

	var real, fake *domain.MatchCandidate
	for i := range scored {
		switch scored[i].ExternalRef {
		case "rel-real":
			real = &scored[i]
		case "rel-fake":
			fake = &scored[i]
		}
	}
	if real == nil || fake == nil {
		t.Fatalf("ConfidenceScore() = %+v, want both rel-real and rel-fake as candidates", scored)
	}
	t.Logf("worked example 2: real Score=%v Signals=%v", real.Score, real.Signals)
	t.Logf("worked example 2: fake Score=%v Signals=%v", fake.Score, fake.Signals)

	if real.Score <= fake.Score {
		t.Errorf("real.Score (%v) <= fake.Score (%v), want the real box set to clearly outscore the unrelated compilation", real.Score, fake.Score)
	}
	// Different release groups, but far apart (real's track/medium counts
	// dominate) -> the ambiguity cap must not fire here.
	if real.Score <= 0.50 {
		t.Errorf("real.Score = %v, want well above the ambiguity-cap ceiling (0.50) — this pair should not be ambiguous", real.Score)
	}
}
