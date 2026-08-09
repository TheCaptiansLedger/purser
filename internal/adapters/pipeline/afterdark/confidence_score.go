// This file is the ports.ConfidenceScorer implementation for
// domain.ContentTypeAdult — AD6 (issue #560), mirroring
// internal/adapters/pipeline/music/confidence_score.go (M8)'s shape, with
// one deliberate simplification: AD6's scope is per-candidate scoring only.
// Unlike Music's ConfidenceScore, nothing here ever compares one
// candidate's Score against another's — no release-group-style ambiguity
// cap, no clustering. Per docs/adr/0027-provider-independence.md, StashDB
// and ThePornDB candidates are never ranked against each other either; that
// cross-candidate decision belongs to AD7, not this file.
package afterdark

import (
	"context"
	"log/slog"
	"math"
	"purser/internal/domain"
	"purser/internal/ports"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Per-tier score bands, ascending by evidence strength and deliberately
// non-overlapping (docs/adr/0025-music-identification-confidence-scoring.md
// decision #3's "clearly separated score bands" principle, applied here by
// analogy even though that ADR is Music-scoped) so a fuzzy match can never
// land on the same number as a fingerprint hit. A starting proposal flagged
// for validation against the fixture suite below — not asserted correct on
// first pass, the same caveat every other unvalidated constant in
// identifier.go already carries.
const (
	// fuzzyBandLow/fuzzyBandHigh — free-text studio/title search, the
	// weakest tier.
	fuzzyBandLow  = 0.35
	fuzzyBandHigh = 0.58

	// javCodeBandLow/javCodeBandHigh — ThePornDB's /jav?parse= tier.
	// Structured (a parsed product code, not free text) but tolerant
	// (ports.ThePornDBClient.ResolveJAVCode's own doc comment: "ordinarily
	// including scenes that don't share the exact code"), so it sits above
	// fuzzy but below the two deterministic tiers.
	javCodeBandLow  = 0.60
	javCodeBandHigh = 0.75

	// fingerprintBase/fingerprintCeiling — OSHash/PHash tier
	// (Tier=MatchTierUniqueID, per identifier.go's fingerprintCandidate).
	// A matching perceptual/exact hash is near-1:1 identifying on its own;
	// fingerprintBase is the score with no corroborating submissions data
	// at all, fingerprintCeiling is the cap once the corroboration bonus
	// below is fully saturated — kept below direct_id's flat score since a
	// computed hash match is still one step short of an authoritative ID
	// lookup.
	fingerprintBase    = 0.80
	fingerprintCeiling = 0.95

	// fingerprintSubmissionsSaturation is the Signals["fingerprint_
	// submissions"] value (a raw crowd-sourced count, not a 0-1 fraction —
	// both StashDB's fingerprints[].submissions and ThePornDB's
	// hashes[].submissions report it, per
	// docs/technical/afterdark-data_model.md's "a strong
	// duplicate-confirmation signal" framing) at which the corroboration
	// bonus below saturates at 1.0. Below this, the bonus scales linearly —
	// a simple, defensible curve for a first pass; not validated against
	// real submission-count distributions yet.
	fingerprintSubmissionsSaturation = 10.0

	// directIDScore is direct_id's flat score — identifier.go's
	// directIDCandidate already short-circuits to a single, authoritative
	// per-provider UUID lookup, nothing to average or differentiate.
	// Matches Music's own direct_id constant.
	directIDScore = 0.98

	// coverageFloorThreshold is the minimum fraction of fuzzy's possible
	// signal weight that must have real data on both sides before a
	// candidate is allowed into the top half of fuzzy's band — the same
	// generic coverage-floor concept ADR-0025 introduced for Music's own
	// convergence-bug fix (a candidate scored on very little evidence must
	// not be able to reach the top of its tier), applied here since it's a
	// scoring-hygiene concern, not something specific to Music's tag
	// signals.
	coverageFloorThreshold = 0.5
)

// fuzzySignalWeights are fuzzy's weighted-average terms —
// title_similarity/studio_similarity from identifier.go's fuzzyCandidate.
// Title weighted higher: a studio-name match alone is common (many scenes
// share a studio) and weaker evidence than a title match. Their sum
// (fuzzyBaseWeight below) is kept as a separate literal, not computed from
// this map, so the "possible weight" figure the coverage floor uses doesn't
// silently drift if a weight changes without the constant being reviewed
// alongside it — same convention as Music's fuzzySignalWeights.
var fuzzySignalWeights = map[string]float64{
	"title_similarity":  0.60,
	"studio_similarity": 0.40,
}

// fuzzyBaseWeight is the sum of fuzzySignalWeights — see that var's comment.
const fuzzyBaseWeight = 1.0

// ConfidenceScorer implements ports.ConfidenceScorer for
// domain.ContentTypeAdult: the AD6 per-tier scoring formulas. Needs no
// ports of its own — pure computation over an already-populated
// domain.MatchCandidate slice — but is still adapter-layer, not
// domain-layer, for the same reason Identifier lives here rather than in
// internal/domain: the formulas are AfterDark-specific "module-owned
// computation" per docs/adr/0024-pipeline-core.md.
type ConfidenceScorer struct {
	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.ConfidenceScorer = (*ConfidenceScorer)(nil)

// NewConfidenceScorer constructs a ConfidenceScorer. Reuses the same
// Option/WithLogger/WithTracerProvider declared in fingerprinter.go — every
// type in this package shares the same {logger, tracerProvider} shape.
func NewConfidenceScorer(opts ...Option) *ConfidenceScorer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &ConfidenceScorer{
		logger: o.logger.With("component", "adapters.pipeline.afterdark.confidence_score"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.ConfidenceScorer.
func (s *ConfidenceScorer) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// ConfidenceScore implements ports.ConfidenceScorer: scores every candidate
// independently against its own Tier's formula. Deliberately does not read
// or write anything about any other candidate in the slice — see this
// file's package comment for why that's AD6's scope, not an oversight.
func (s *ConfidenceScorer) ConfidenceScore(ctx context.Context, _ domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	ctx, span := s.tracer.Start(ctx, "afterdark.confidence_score.score", trace.WithAttributes(
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

	s.logger.DebugContext(ctx, "scored candidates", "candidate_count", len(scored))
	return scored, nil
}

// scoreCandidate dispatches to c.Tier's formula. Tier is a closed set
// identifier.go assigns; an unrecognized/zero-value Tier falls to the fuzzy
// formula, the same structural fallback Music's scoreCandidate uses, never
// an error.
func scoreCandidate(c domain.MatchCandidate) float64 {
	switch c.Tier {
	case domain.MatchTierDirectID:
		return directIDScore
	case domain.MatchTierUniqueID:
		return scoreFingerprint(c.Signals)
	case domain.MatchTierAcoustic:
		return scoreJAVCode(c.Signals)
	default:
		return scoreFuzzy(c.Signals)
	}
}

// scoreFingerprint implements the fingerprint (unique_id) formula:
// fingerprintBase, plus a corroboration bonus from Signals["fingerprint_
// submissions"] (identifier.go's stashdbFingerprintSubmissions/
// tpdbHashSubmissions) that scales linearly up to
// fingerprintSubmissionsSaturation submissions, capped at fingerprintCeiling.
// A candidate with no submissions data (the signal is absent whenever
// neither provider's matching fingerprint entry carried a positive count —
// see fingerprintCandidate) scores exactly fingerprintBase, never treated
// as a zero-value submissions count with its own effect.
func scoreFingerprint(signals map[string]float64) float64 {
	bonus := 0.0
	if submissions, ok := signals["fingerprint_submissions"]; ok && submissions > 0 {
		bonus = math.Min(submissions/fingerprintSubmissionsSaturation, 1.0)
	}
	return fingerprintBase + bonus*(fingerprintCeiling-fingerprintBase)
}

// scoreJAVCode implements the JAV-code (acoustic-tier) formula:
// Signals["jav_code_match"] mapped directly into javCodeBand. Every
// candidate javCodeTier produces today carries jav_code_match=1.0
// unconditionally (ResolveJAVCode's own doc comment: ThePornDB's endpoint
// is tolerant and doesn't itself report a per-scene match quality), so this
// formula currently returns the band's flat ceiling for every JAV-code
// candidate — written as a mapped range rather than a bare constant so a
// future fractional match-quality signal (e.g. code-string similarity) is a
// signals-map change in identifier.go, not a scoring-formula rewrite here.
func scoreJAVCode(signals map[string]float64) float64 {
	match := signals["jav_code_match"]
	return javCodeBandLow + match*(javCodeBandHigh-javCodeBandLow)
}

// scoreFuzzy implements the fuzzy formula: a weighted average over only the
// signals that had real data on both sides (title_similarity/
// studio_similarity — identifier.go's fuzzyCandidate only sets a key when
// both the query and candidate side were non-empty), mapped into fuzzy's
// band. Below coverageFloorThreshold evaluated-weight coverage — i.e. only
// the weaker studio_similarity signal was evaluable, title_similarity
// wasn't — the result is clamped to the band's midpoint, so a studio-only
// match can never reach the top of fuzzy's own band on its own.
func scoreFuzzy(signals map[string]float64) float64 {
	sumWeight, sumWeighted := 0.0, 0.0
	for key, weight := range fuzzySignalWeights {
		if v, ok := signals[key]; ok {
			sumWeight += weight
			sumWeighted += v * weight
		}
	}

	rawAvg := 0.0
	if sumWeight > 0 {
		rawAvg = sumWeighted / sumWeight
	}
	score := fuzzyBandLow + rawAvg*(fuzzyBandHigh-fuzzyBandLow)

	coverage := sumWeight / fuzzyBaseWeight
	if coverage < coverageFloorThreshold {
		score = math.Min(score, fuzzyBandLow+(fuzzyBandHigh-fuzzyBandLow)/2)
	}
	return score
}
