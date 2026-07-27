package ports

import (
	"context"
	"purser/internal/domain"
)

// ConfidenceScorer is a content-type-scoped capability: given the group's
// consensus domain.Fingerprint and the whole []domain.MatchCandidate list a
// ports.Identifier produced (Tier/Signals/Metadata populated, Score unset),
// return that same list with Score populated on every candidate. Batch-
// shaped rather than per-candidate — a correction to
// docs/adr/0024-pipeline-core.md's original per-candidate framing, the same
// way Grouping (M3) and FileFingerprinter.Consensus (M4) turned out to need
// batch shape once the mechanics were worked out: the release-group
// ambiguity cap needs to compare the top candidate against its runner-up,
// which a strictly per-candidate call can't see. Fanned out to by
// ConfidenceScoreResolver via ContentTypes(), the same registry pattern
// Identifier already established — adding a new content type's scoring is a
// new ConfidenceScorer implementation, never an edit to the registry or its
// caller. What signals exist and how they combine is entirely module-owned;
// what's shared is everything downstream of the returned number (the
// confidence threshold and the auto-import/review-queue decision), which
// this capability never sees. See
// docs/adr/0025-music-identification-confidence-scoring.md,
// docs/technical/pipeline-music-confidence-score.md.
type ConfidenceScorer interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// ConfidenceScore takes candidates (Tier/Signals/Metadata already
	// populated, Score unset) and returns them with Score populated on
	// every element, ambiguity already folded into whichever one ends up
	// on top. A nil/empty candidates slice returns a nil/empty slice, not
	// an error.
	ConfidenceScore(ctx context.Context, fingerprint domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error)
}

// ConfidenceScoreResolver is the fan-out dispatch capability a scoring
// caller depends on: given a Job's already-resolved content type, look up
// the matching registered ConfidenceScorer implementation (or fall back to
// a no-op default if none is registered) and call through to it. Kept
// distinct from ConfidenceScorer itself for the same reason
// IdentifierResolver is kept distinct from Identifier — a single
// ConfidenceScorer implementation never sees a contentType argument; the
// resolver is what decides which one to call.
type ConfidenceScoreResolver interface {
	ConfidenceScore(ctx context.Context, contentType domain.ContentType, fingerprint domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error)
}
