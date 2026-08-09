package afterdark_test

import (
	"context"
	"math"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// scoreTolerance absorbs float64 summation-order noise — scoreFuzzy sums
// fuzzySignalWeights by ranging over a map, whose iteration order Go
// deliberately randomizes, so bit-for-bit equality against a hand-computed
// expected value isn't safe even though the formula itself is
// deterministic. Same convention as music/confidence_score_test.go.
const scoreTolerance = 1e-9

func assertScore(t *testing.T, got, want float64, msgAndArgs string) {
	t.Helper()
	if math.Abs(got-want) > scoreTolerance {
		t.Errorf("%s: Score = %v, want %v", msgAndArgs, got, want)
	}
}

// mkCandidate builds one hand-fed domain.MatchCandidate for a direct
// formula test — the same Tier/Signals/Metadata shape identifier.go's own
// candidate constructors (directIDCandidate/fingerprintCandidate/
// javCodeTier/fuzzyCandidate) produce.
func mkCandidate(ref string, tier domain.MatchTier, signals map[string]float64, source string) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: ref,
		Tier:        tier,
		Signals:     signals,
		Metadata:    map[string]any{"source": source},
	}
}

func scoreOne(t *testing.T, c domain.MatchCandidate) float64 {
	t.Helper()
	scorer := afterdark.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, []domain.MatchCandidate{c})
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ConfidenceScore() returned %d candidates, want 1", len(got))
	}
	return got[0].Score
}

func byExternalRef(candidates []domain.MatchCandidate) map[string]domain.MatchCandidate {
	m := make(map[string]domain.MatchCandidate, len(candidates))
	for _, c := range candidates {
		m[c.ExternalRef] = c
	}
	return m
}

// ============================================================================
// Structural tests
// ============================================================================

func TestConfidenceScorer_ContentTypes(t *testing.T) {
	got := afterdark.NewConfidenceScorer().ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Errorf("ContentTypes() = %v, want [ContentTypeAdult]", got)
	}
}

func TestConfidenceScore_EmptyCandidates_Noop(t *testing.T) {
	scorer := afterdark.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, nil)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ConfidenceScore(nil) = %+v, want empty", got)
	}
}

// ============================================================================
// direct_id tier
// ============================================================================

// SHOULD: a direct-ID candidate (identifier.go's directIDCandidate — empty
// Signals) always scores the flat 0.98, regardless of provider.
func TestConfidenceScore_DirectID_Flat(t *testing.T) {
	for _, source := range []string{"stashdb", "tpdb"} {
		c := mkCandidate("scene-1", domain.MatchTierDirectID, map[string]float64{}, source)
		assertScore(t, scoreOne(t, c), 0.98, "direct_id flat score, source="+source)
	}
}

// ============================================================================
// unique_id (fingerprint) tier
// ============================================================================

// SHOULD: a fingerprint hit with no fingerprint_submissions signal at all
// (identifier.go only sets it when submissions > 0) scores exactly the
// tier's base — no bonus, no error from a missing key.
func TestConfidenceScore_Fingerprint_NoSubmissions_Base(t *testing.T) {
	c := mkCandidate("scene-1", domain.MatchTierUniqueID, map[string]float64{"fingerprint_match": 1.0}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.80, "fingerprint hit, no submissions data")
}

// SHOULD: a modest submissions count (StashDB's own live-verified example
// per docs/technical/afterdark-data_model.md was 4) produces a real,
// partial bonus above base but below the ceiling.
func TestConfidenceScore_Fingerprint_PartialSubmissions_PartialBonus(t *testing.T) {
	c := mkCandidate("scene-1", domain.MatchTierUniqueID, map[string]float64{
		"fingerprint_match":       1.0,
		"fingerprint_submissions": 4,
	}, "stashdb")
	// bonus = min(4/10, 1.0) = 0.4: 0.80 + 0.4*(0.95-0.80) = 0.86.
	assertScore(t, scoreOne(t, c), 0.86, "fingerprint hit, 4 submissions")
}

// SHOULD: submissions at or beyond the saturation point cap at the tier
// ceiling, never exceeding it (e.g. TPDBDedupsAcrossHashes-style summed
// counts can exceed the saturation point in practice).
func TestConfidenceScore_Fingerprint_SubmissionsBeyondSaturation_CapsAtCeiling(t *testing.T) {
	for _, submissions := range []float64{10, 25, 1000} {
		c := mkCandidate("scene-1", domain.MatchTierUniqueID, map[string]float64{
			"fingerprint_match":       1.0,
			"fingerprint_submissions": submissions,
		}, "tpdb")
		assertScore(t, scoreOne(t, c), 0.95, "fingerprint hit, saturated submissions")
	}
}

// SHOULD: the fingerprint tier's ceiling stays strictly below direct_id's
// flat score even at full saturation — a computed hash match never outranks
// an authoritative direct-ID lookup.
func TestConfidenceScore_Fingerprint_CeilingBelowDirectID(t *testing.T) {
	c := mkCandidate("scene-1", domain.MatchTierUniqueID, map[string]float64{
		"fingerprint_match":       1.0,
		"fingerprint_submissions": 1000,
	}, "stashdb")
	if got := scoreOne(t, c); got >= 0.98 {
		t.Errorf("fingerprint ceiling Score = %v, want strictly below direct_id's 0.98", got)
	}
}

// ============================================================================
// acoustic (JAV-code) tier
// ============================================================================

// SHOULD: every candidate javCodeTier produces today carries
// jav_code_match=1.0 (ResolveJAVCode never reports partial match quality),
// so it scores the band's flat ceiling.
func TestConfidenceScore_JAVCode_FullMatch_BandCeiling(t *testing.T) {
	c := mkCandidate("jav-1", domain.MatchTierAcoustic, map[string]float64{"jav_code_match": 1.0}, "tpdb")
	assertScore(t, scoreOne(t, c), 0.75, "jav-code full match")
}

// SHOULD: the formula is written as a mapped range, not a bare constant —
// a synthetic partial match value (forward-compatible with a future
// code-similarity signal, even though identifier.go doesn't produce one
// today) lands proportionally inside the band, not clamped to either end.
func TestConfidenceScore_JAVCode_PartialMatch_ScalesWithinBand(t *testing.T) {
	c := mkCandidate("jav-1", domain.MatchTierAcoustic, map[string]float64{"jav_code_match": 0.5}, "tpdb")
	// 0.60 + 0.5*(0.75-0.60) = 0.675.
	assertScore(t, scoreOne(t, c), 0.675, "jav-code partial match")
}

// SHOULD: a JAV-code candidate always outscores fuzzy's own band ceiling —
// the two tiers are structurally separated per this file's non-overlapping
// band design, even at fuzzy's absolute best and jav-code's absolute worst.
func TestConfidenceScore_JAVCode_AboveFuzzyBand(t *testing.T) {
	worstJAV := mkCandidate("jav-1", domain.MatchTierAcoustic, map[string]float64{"jav_code_match": 0.0}, "tpdb")
	bestFuzzy := mkCandidate("fuzzy-1", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  1.0,
		"studio_similarity": 1.0,
	}, "stashdb")
	if scoreOne(t, worstJAV) <= scoreOne(t, bestFuzzy) {
		t.Errorf("worst-case jav-code Score (%v) <= best-case fuzzy Score (%v), want the bands to never touch", scoreOne(t, worstJAV), scoreOne(t, bestFuzzy))
	}
}

// ============================================================================
// fuzzy tier
// ============================================================================

// SHOULD: a strong match on both signals lands at the top of fuzzy's band.
func TestConfidenceScore_Fuzzy_StrongBothSignals_TopOfBand(t *testing.T) {
	c := mkCandidate("stash-1", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  1.0,
		"studio_similarity": 1.0,
	}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.58, "fuzzy, both signals perfect")
}

// SHOULD: a weak match on both signals lands at the bottom of fuzzy's band.
func TestConfidenceScore_Fuzzy_WeakBothSignals_BottomOfBand(t *testing.T) {
	c := mkCandidate("stash-1", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  0.0,
		"studio_similarity": 0.0,
	}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.35, "fuzzy, both signals zero")
}

// SHOULD: title_similarity alone (identifier.go's fuzzyCandidate omits
// studio_similarity whenever either side's studio name is empty — see
// TestIdentifier_Identify_FuzzyTier_NoStudioOnEitherSideOmitsSignal) still
// covers 0.60 of fuzzy's 1.0 possible weight — at or above
// coverageFloorThreshold, so a strong title match alone is NOT clamped and
// still reaches the top of the band.
func TestConfidenceScore_Fuzzy_TitleOnly_CoverageAtFloor_NotClamped(t *testing.T) {
	c := mkCandidate("stash-1", domain.MatchTierFuzzy, map[string]float64{"title_similarity": 1.0}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.58, "fuzzy, title_similarity only, strong")
}

// SHOULD: studio_similarity alone covers only 0.40 of fuzzy's possible
// weight — below coverageFloorThreshold — so even a perfect studio match is
// clamped to the band's midpoint, never allowed to reach the top of the
// band on weak, partial evidence alone.
func TestConfidenceScore_Fuzzy_StudioOnly_CoverageBelowFloor_ClampedToMidpoint(t *testing.T) {
	c := mkCandidate("stash-1", domain.MatchTierFuzzy, map[string]float64{"studio_similarity": 1.0}, "stashdb")
	// Raw would be fuzzyBandLow + 1.0*(fuzzyBandHigh-fuzzyBandLow) = 0.58,
	// but coverage=0.40<0.5 clamps to the midpoint: 0.35+(0.58-0.35)/2 = 0.465.
	assertScore(t, scoreOne(t, c), 0.465, "fuzzy, studio_similarity only, clamped to midpoint despite a perfect value")
}

// SHOULD: no evaluable signals at all (both sides had an empty string —
// structurally shouldn't happen since fuzzyTier bails out before producing
// any candidate, but the formula must still degrade safely) scores the
// band floor, not an error or NaN.
func TestConfidenceScore_Fuzzy_NoSignals_BandFloor(t *testing.T) {
	c := mkCandidate("stash-1", domain.MatchTierFuzzy, map[string]float64{}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.35, "fuzzy, no evaluable signals")
}

// SHOULD (mirrors M8's own differentiation assertion): a strong fuzzy match
// scores meaningfully above a weak one through the same formula — the
// property the whole tiered/coverage design exists to guarantee.
func TestConfidenceScore_Fuzzy_Differentiation_StrongBeatsWeak(t *testing.T) {
	strong := mkCandidate("strong", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  0.95,
		"studio_similarity": 0.90,
	}, "stashdb")
	weak := mkCandidate("weak", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  0.20,
		"studio_similarity": 0.10,
	}, "tpdb")
	strongScore, weakScore := scoreOne(t, strong), scoreOne(t, weak)
	if diff := strongScore - weakScore; diff < 0.15 {
		t.Errorf("strong.Score (%v) - weak.Score (%v) = %v, want >= 0.15 real differentiation", strongScore, weakScore, diff)
	}
}

// ============================================================================
// Unrecognized tier
// ============================================================================

// SHOULD: an unrecognized/zero-value Tier falls through to the fuzzy
// formula, the same structural fallback Music's scoreCandidate uses — never
// a panic or a zero score.
func TestConfidenceScore_UnknownTier_FallsBackToFuzzy(t *testing.T) {
	c := mkCandidate("mystery", domain.MatchTier("unknown"), map[string]float64{
		"title_similarity":  1.0,
		"studio_similarity": 1.0,
	}, "stashdb")
	assertScore(t, scoreOne(t, c), 0.58, "unrecognized tier falls back to fuzzy")
}

// ============================================================================
// No cross-candidate comparison — AD6's core verification item
// ============================================================================

// SHOULD: every candidate's Score is a pure function of its own Tier and
// Signals. Scoring a candidate alone must produce the exact same number as
// scoring it inside a slice alongside very differently-scored candidates
// from both providers — proof no code path compares one candidate's score
// against another's, per AD6's verification checklist.
func TestConfidenceScore_NoCrossCandidateComparison(t *testing.T) {
	target := mkCandidate("target", domain.MatchTierFuzzy, map[string]float64{
		"title_similarity":  0.72,
		"studio_similarity": 0.60,
	}, "stashdb")
	alone := scoreOne(t, target)

	crowded := []domain.MatchCandidate{
		mkCandidate("direct", domain.MatchTierDirectID, map[string]float64{}, "tpdb"),
		mkCandidate("fp-strong", domain.MatchTierUniqueID, map[string]float64{"fingerprint_match": 1.0, "fingerprint_submissions": 50}, "stashdb"),
		target,
		mkCandidate("jav", domain.MatchTierAcoustic, map[string]float64{"jav_code_match": 1.0}, "tpdb"),
		mkCandidate("fuzzy-weak", domain.MatchTierFuzzy, map[string]float64{"title_similarity": 0.05}, "tpdb"),
	}
	scorer := afterdark.NewConfidenceScorer()
	got, err := scorer.ConfidenceScore(context.Background(), domain.Fingerprint{}, crowded)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	withinCrowd := byExternalRef(got)["target"]
	assertScore(t, withinCrowd.Score, alone, "target candidate's score must be identical alone vs. inside a crowded, mixed-tier slice")
}

// ============================================================================
// End-to-end: real Identifier (AD5) output scored by the real
// ConfidenceScorer (AD6) — "realistic candidate shapes from AD5" per this
// issue's verification checklist, not just hand-fed Signals maps.
// ============================================================================

// SHOULD: a StashDB fingerprint hit with real crowd-sourced submissions, a
// ThePornDB JAV-code hit, and a fuzzy hit on both providers, run through
// the real Identifier and then the real ConfidenceScorer, land each
// candidate in its expected band — and the fingerprint hit (which carries
// real corroboration data) outscores the flat-signal JAV-code hit, which in
// turn outscores the fuzzy hits.
func TestConfidenceScore_EndToEnd_RealisticMixedTierScene(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.scenesByFingerprint = []ports.Scene{
		{ID: "stash-fp", Title: "Real Scene", Fingerprints: []ports.Fingerprint{
			{Hash: "aa11", Algorithm: ports.FingerprintAlgorithmOSHash, Submissions: 6},
		}},
	}
	// FilenameParser.Parse splits the leaf on " - " into studio/title, so
	// the fuzzy-tier query term is the whole bracketed remainder, JAV code
	// included — registered here exactly as Parse will actually produce it,
	// not the human-readable title alone.
	const fuzzyQueryTitle = "Sneaking In [SSIS-001]"
	stashDB.scenesByTerm[fuzzyQueryTitle] = []ports.Scene{
		{ID: "stash-fuzzy", Title: fuzzyQueryTitle, Studio: &ports.Studio{Name: "Brazzers"}},
	}

	tpdb := newFakeThePornDB()
	tpdb.javByCode["SSIS-001"] = []ports.TPDBScene{{ID: "tpdb-jav", Title: "JAV Match"}}
	tpdb.scenesByTerm[fuzzyQueryTitle] = []ports.TPDBScene{
		{ID: "tpdb-fuzzy", Title: "Sneaking Into My Roommate", Site: &ports.TPDBSite{Name: "Brazzers Network"}},
	}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	fp := domain.Fingerprint{Metadata: map[string]any{"os_hash": "aa11"}}
	path := "/scenes/Brazzers - Sneaking In [SSIS-001].mp4"

	candidates, err := id.Identify(context.Background(), fp, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}

	scorer := afterdark.NewConfidenceScorer()
	scored, err := scorer.ConfidenceScore(context.Background(), fp, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	got := byExternalRef(scored)

	fingerprintC, ok := got["stash-fp"]
	if !ok {
		t.Fatalf("scored = %+v, want a stash-fp fingerprint candidate", scored)
	}
	if fingerprintC.Score <= 0.80 || fingerprintC.Score > 0.95 {
		t.Errorf("fingerprint candidate Score = %v, want within (0.80, 0.95]", fingerprintC.Score)
	}

	javC, ok := got["tpdb-jav"]
	if !ok {
		t.Fatalf("scored = %+v, want a tpdb-jav candidate", scored)
	}
	assertScore(t, javC.Score, 0.75, "jav-code candidate")

	stashFuzzyC, ok := got["stash-fuzzy"]
	if !ok {
		t.Fatalf("scored = %+v, want a stash-fuzzy candidate", scored)
	}
	if stashFuzzyC.Score < 0.35 || stashFuzzyC.Score > 0.58 {
		t.Errorf("stash fuzzy candidate Score = %v, want within [0.35, 0.58]", stashFuzzyC.Score)
	}

	// Band ordering: fingerprint > jav-code > fuzzy, exactly as designed.
	if fingerprintC.Score <= javC.Score {
		t.Errorf("fingerprint.Score (%v) <= jav-code.Score (%v), want fingerprint to outrank jav-code", fingerprintC.Score, javC.Score)
	}
	if javC.Score <= stashFuzzyC.Score {
		t.Errorf("jav-code.Score (%v) <= fuzzy.Score (%v), want jav-code to outrank fuzzy", javC.Score, stashFuzzyC.Score)
	}
}
