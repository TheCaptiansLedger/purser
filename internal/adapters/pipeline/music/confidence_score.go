package music

import (
	"context"
	"log/slog"
	"math"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Per-tier score bands/constants, per
// docs/technical/pipeline-music-confidence-score.md. Every numeric value
// here is a starting point flagged for validation against the fixture
// suite, the same caveat every other unvalidated constant in identifier.go
// carries — not asserted correct on first pass.
const (
	// directIDScore is direct_id's flat score — M7 already short-circuits
	// to a single candidate for this tier, nothing to average or
	// differentiate.
	directIDScore = 0.98

	// unique_id's base band: barcode-or-ISRC consensus alone starts at
	// uniqueIDBaseSingle; both agreeing on the same release starts higher.
	// Either way the corroboration term caps the result below direct_id's
	// 0.98.
	uniqueIDBaseSingle    = 0.80
	uniqueIDBaseBothAgree = 0.90
	uniqueIDCeiling       = 0.95

	// fuzzy's band — the tier the convergence-bug fix targets directly.
	fuzzyBandLow  = 0.45
	fuzzyBandHigh = 0.75

	// acousticSoleCeiling is acoustic-as-sole-evidence's hard cap: audio
	// fingerprinting alone is less trustworthy than corroborated tags
	// (cover versions, remasters), so it gets a structurally lower
	// ceiling even at maximum confidence, not just a lower starting
	// point.
	acousticSoleCeiling = 0.60

	// acousticContradictionMultiplier demotes a fuzzy-tier candidate
	// AcoustID confidently resolved to a *different* release group than
	// the one being scored — stronger evidence than merely absent
	// corroboration, so it multiplies the result rather than just
	// contributing one more low term to the average.
	acousticContradictionMultiplier = 0.5

	// coverageFloorThreshold is the minimum fraction of a formula's
	// possible signal weight that must have real data on both sides
	// before a candidate is allowed into the top half of its tier's
	// band. Below this, the score is clamped to the band's midpoint —
	// closes the design-review gap in issue #516: a candidate scored on
	// very few signals (e.g. an untagged, filename-sourced candidate
	// where only track-count and duration are evaluable) must not be
	// able to reach the top of its tier's band, since that's how a
	// filename-only candidate could otherwise land in auto-import
	// territory, contradicting
	// docs/adr/0025-music-identification-confidence-scoring.md's decision
	// #1. Generic across tiers — not a source-based special case.
	coverageFloorThreshold = 0.5

	// releaseGroupAmbiguityMargin/releaseGroupAmbiguityCeiling implement
	// the release-group ambiguity cap: two *different* release-group
	// clusters whose top scores land within this margin of each other are
	// real ambiguity, and the winner is clamped to this fixed ceiling —
	// comfortably below fuzzy's entire band — regardless of what the
	// per-candidate formula computed. The auto-import threshold is
	// shared-service/config-owned; a fixed ceiling keeps this working for
	// any sane threshold without Music's scoring code needing to know
	// what it actually is.
	releaseGroupAmbiguityMargin  = 0.10
	releaseGroupAmbiguityCeiling = 0.50
)

// fuzzySignalWeights are fuzzy's weighted-average terms. Their sum
// (0.30+0.20+0.30+0.20) is fuzzyBaseWeight below — kept as a separate
// literal rather than computed from this map so the "possible weight"
// figure used by the coverage floor doesn't silently drift if a weight
// changes without the constant being reviewed alongside it.
var fuzzySignalWeights = map[string]float64{
	"name_fuzzy_score":  0.30,
	"track_count_match": 0.20,
	"title_set_overlap": 0.30,
	"duration_match":    0.20,
}

const (
	// fuzzyBaseWeight is the sum of fuzzySignalWeights — see that var's
	// comment.
	fuzzyBaseWeight = 1.0

	// acousticAgreementFuzzyWeight is acoustic_agreement's weight when it
	// folds into the fuzzy average as a corroborating term (Tier==fuzzy
	// candidates only — see scoreFuzzy). Not specified numerically by
	// docs/technical/pipeline-music-confidence-score.md beyond "one more
	// term in the weighted average"; chosen on par with the two primary
	// tag signals since independent acoustic corroboration is meaningful
	// evidence when available.
	acousticAgreementFuzzyWeight = 0.30
)

// uniqueIDCorroborationSignals are unique_id's corroboration term inputs —
// an unweighted average per
// docs/technical/pipeline-music-confidence-score.md ("average", not
// "weighted average", unlike fuzzy).
var uniqueIDCorroborationSignals = [...]string{"track_count_match", "title_set_overlap", "duration_match"}

// ConfidenceScorer implements ports.ConfidenceScorer for
// domain.ContentTypeMusic: the M8 per-tier scoring formulas, the coverage
// floor, and the release-group ambiguity cap, per
// docs/adr/0025-music-identification-confidence-scoring.md and
// docs/technical/pipeline-music-confidence-score.md. Needs no ports of its
// own — pure computation over an already-populated
// domain.Fingerprint/[]domain.MatchCandidate — but is still adapter-layer,
// not domain-layer: the formulas are Music-specific per
// docs/adr/0024-pipeline-core.md's "module-owned computation" framing, the
// same reason Identifier lives here rather than in internal/domain/music.
type ConfidenceScorer struct {
	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.ConfidenceScorer = (*ConfidenceScorer)(nil)

// NewConfidenceScorer constructs a ConfidenceScorer. Reuses the same
// Option/WithLogger/WithTracerProvider used by New (FileFingerprinter's
// constructor) and NewIdentifier — all three share the same
// {logger, tracerProvider} shape.
func NewConfidenceScorer(opts ...Option) *ConfidenceScorer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &ConfidenceScorer{
		logger: o.logger.With("component", "adapters.pipeline.music.confidence_score"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.ConfidenceScorer.
func (s *ConfidenceScorer) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// ConfidenceScore implements ports.ConfidenceScorer: scores every candidate
// independently against its own Tier's formula, then folds the
// release-group ambiguity cap into whichever one ends up on top.
func (s *ConfidenceScorer) ConfidenceScore(ctx context.Context, _ domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	ctx, span := s.tracer.Start(ctx, "music.confidence_score.score", trace.WithAttributes(
		attribute.Int("pipeline.candidate_count", len(candidates)),
	))
	defer span.End()

	if len(candidates) == 0 {
		return candidates, nil
	}

	scored := make([]domain.MatchCandidate, len(candidates))
	for i, c := range candidates {
		scored[i] = c
		scored[i].Score = scoreCandidate(c)
	}

	ambiguityCapApplied := resolveReleaseGroups(scored)
	span.SetAttributes(attribute.Bool("pipeline.ambiguity_cap_applied", ambiguityCapApplied))

	s.logger.DebugContext(ctx, "scored candidates", "candidate_count", len(scored), "ambiguity_cap_applied", ambiguityCapApplied)
	return scored, nil
}

// scoreCandidate dispatches to c.Tier's formula. Tier is a closed set M7's
// classifyTier assigns; its own structural fallback is fuzzy (see
// identifier.go), so an unrecognized/zero-value Tier here falls to the same
// formula, never an error.
func scoreCandidate(c domain.MatchCandidate) float64 {
	switch c.Tier {
	case domain.MatchTierDirectID:
		return directIDScore
	case domain.MatchTierUniqueID:
		return scoreUniqueID(c.Signals)
	case domain.MatchTierAcoustic:
		return scoreAcousticSoleEvidence(c.Signals)
	default:
		return scoreFuzzy(c.Signals)
	}
}

// scoreUniqueID implements the unique_id formula: base (barcode-or-ISRC
// alone, or higher if both agree) plus a continuous corroboration average
// over whichever of track_count_match/title_set_overlap/duration_match were
// evaluable, capped below direct_id's 0.98. Below coverageFloorThreshold
// corroboration coverage, the result is clamped to the band's midpoint.
func scoreUniqueID(signals map[string]float64) float64 {
	base := uniqueIDBaseSingle
	if barcode, ok := signals["barcode_match"]; ok && barcode >= 1.0 {
		if isrc, ok := signals["isrc_consensus_fraction"]; ok && isrc > isrcConsensusMajorityThreshold {
			base = uniqueIDBaseBothAgree
		}
	}

	vals := make([]float64, 0, len(uniqueIDCorroborationSignals))
	for _, key := range uniqueIDCorroborationSignals {
		if v, ok := signals[key]; ok {
			vals = append(vals, v)
		}
	}
	score := base + average(vals)*(uniqueIDCeiling-base)

	coverage := float64(len(vals)) / float64(len(uniqueIDCorroborationSignals))
	if coverage < coverageFloorThreshold {
		score = math.Min(score, base+(uniqueIDCeiling-base)/2)
	}
	return score
}

// scoreFuzzy implements the fuzzy formula — the actual convergence-bug fix:
// a weighted average over only the signals that had real data on both
// sides (never normalized against the weight of every possible signal),
// mapped into fuzzy's band. acoustic_agreement folds in as one more
// weighted term when it ran (Tier==fuzzy means this candidate already
// qualifies on its own tag signals, so AcoustID here is corroboration, not
// sole evidence — see scoreAcousticSoleEvidence for the other role). Below
// coverageFloorThreshold evaluated-weight coverage, the result is clamped
// to the band's midpoint. A confident contradiction (acoustic_agreement
// present and exactly zero — real AcoustID match data existed for the
// group but none of it agreed with this candidate) demotes the result
// afterward, on top of whatever the weighted average and floor already
// produced.
func scoreFuzzy(signals map[string]float64) float64 {
	sumWeight, sumWeighted := 0.0, 0.0
	for key, weight := range fuzzySignalWeights {
		if v, ok := signals[key]; ok {
			sumWeight += weight
			sumWeighted += v * weight
		}
	}

	totalPossibleWeight := fuzzyBaseWeight
	acousticAgreement, acousticEvaluated := signals["acoustic_agreement"]
	if acousticEvaluated {
		sumWeight += acousticAgreementFuzzyWeight
		sumWeighted += acousticAgreement * acousticAgreementFuzzyWeight
		totalPossibleWeight += acousticAgreementFuzzyWeight
	}

	rawAvg := 0.0
	if sumWeight > 0 {
		rawAvg = sumWeighted / sumWeight
	}
	score := fuzzyBandLow + rawAvg*(fuzzyBandHigh-fuzzyBandLow)

	coverage := sumWeight / totalPossibleWeight
	if coverage < coverageFloorThreshold {
		score = math.Min(score, fuzzyBandLow+(fuzzyBandHigh-fuzzyBandLow)/2)
	}

	if acousticEvaluated && acousticAgreement == 0.0 {
		score *= acousticContradictionMultiplier
	}

	return score
}

// scoreAcousticSoleEvidence implements acoustic-as-sole-evidence: mapped
// directly into the capped band, no other corroborating signal present by
// construction (M7's classifyTier only assigns Tier=acoustic when fewer
// than two fuzzy signals fired). acoustic_agreement is contractually
// 0.0-1.0, so this is already bounded by acousticSoleCeiling without an
// explicit min().
func scoreAcousticSoleEvidence(signals map[string]float64) float64 {
	return signals["acoustic_agreement"] * acousticSoleCeiling
}

// releaseStatusOfficial is MusicBrainz's release-status string for a
// standard commercial/official release, as opposed to "Promotion",
// "Bootleg", or "Pseudo-Release" — the one substitute signal actually
// available at this layer for IsDefault-style edition preference (see
// resolveReleaseGroups). A starting proposal, like every other constant
// here — not validated against real MusicBrainz data yet.
const releaseStatusOfficial = "Official"

// editionTieBreakMargin is how far below a cluster's chosen representative
// every other same-release-group candidate is pushed when it would
// otherwise tie or beat it — small enough not to cross a tier band or the
// ambiguity margin, just enough to guarantee a strict, deterministic
// single maximum per release group.
const editionTieBreakMargin = 0.001

// resolveReleaseGroups clusters scored by Metadata["release_group_mbid"]
// and does two things in place:
//
//  1. Within each cluster, picks one representative — preferring
//     Metadata["release_status"]=="Official" (IsDefault itself is
//     unreachable at this layer: no ports.Release/domain.MatchCandidate
//     field carries it, since nothing is persisted yet at scoring time —
//     see docs/technical/pipeline-music-confidence-score.md), falling back
//     to highest score, falling back to first-seen for full determinism —
//     then demotes every other same-cluster candidate that would tie or
//     exceed the representative's score by editionTieBreakMargin. Without
//     this, near-duplicate editions of the correct release group routinely
//     score identically (see WorkedExample1_HiInfidelity), leaving a
//     downstream "take the single highest Score" consumer
//     (docs/technical/pipeline-music-persist.md's DecisionService) with an
//     arbitrary, unprincipled tie to break on its own. This closes that
//     gap at the source instead of leaving it for M9 to rediscover.
//  2. If two *different* clusters' (now-unique) top scores land within
//     releaseGroupAmbiguityMargin of each other, clamps the overall
//     winner's Score to releaseGroupAmbiguityCeiling.
//
// Reports whether the cross-cluster ambiguity cap fired, for
// telemetry/logging.
func resolveReleaseGroups(scored []domain.MatchCandidate) bool {
	type cluster struct {
		topIndex      int
		topScore      float64
		topIsOfficial bool
	}

	isOfficial := func(c domain.MatchCandidate) bool {
		status, _ := c.Metadata["release_status"].(string)
		return status == releaseStatusOfficial
	}

	clusters := make(map[string]*cluster)
	order := make([]string, 0)
	for i, c := range scored {
		key, _ := c.Metadata["release_group_mbid"].(string)
		official := isOfficial(c)
		cl, ok := clusters[key]
		if !ok {
			clusters[key] = &cluster{topIndex: i, topScore: c.Score, topIsOfficial: official}
			order = append(order, key)
			continue
		}
		if (official && !cl.topIsOfficial) || (official == cl.topIsOfficial && c.Score > cl.topScore) {
			cl.topIndex = i
			cl.topScore = c.Score
			cl.topIsOfficial = official
		}
	}

	for i, c := range scored {
		key, _ := c.Metadata["release_group_mbid"].(string)
		cl := clusters[key]
		if i == cl.topIndex {
			continue
		}
		if c.Score >= cl.topScore {
			scored[i].Score = cl.topScore - editionTieBreakMargin
		}
	}

	if len(clusters) < 2 {
		return false
	}

	sort.SliceStable(order, func(i, j int) bool {
		return clusters[order[i]].topScore > clusters[order[j]].topScore
	})

	winner, runnerUp := clusters[order[0]], clusters[order[1]]
	if winner.topScore-runnerUp.topScore >= releaseGroupAmbiguityMargin {
		return false
	}

	scored[winner.topIndex].Score = math.Min(scored[winner.topIndex].Score, releaseGroupAmbiguityCeiling)
	return true
}
