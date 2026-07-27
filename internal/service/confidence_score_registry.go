package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ConfidenceScoreRegistry implements ports.ConfidenceScoreResolver by
// dispatching to whichever registered ports.ConfidenceScorer declares the
// requested domain.ContentType via ContentTypes(), falling back to
// NoopConfidenceScorer when none is registered — the same
// ContentTypes()-fan-out registry pattern IdentifierRegistry already
// established, per docs/adr/0024-pipeline-core.md. Adding a new content
// type's scoring means constructing NewConfidenceScoreRegistry with one
// more implementation at the composition root, never editing this type.
type ConfidenceScoreRegistry struct {
	byContentType map[domain.ContentType]ports.ConfidenceScorer
}

var _ ports.ConfidenceScoreResolver = (*ConfidenceScoreRegistry)(nil)

// NewConfidenceScoreRegistry builds a ConfidenceScoreRegistry from scorers,
// indexing each by every domain.ContentType it declares via ContentTypes().
// A later entry declaring a ContentType already claimed by an earlier one
// overwrites it — construction order matters only in that (deliberately
// unlikely) collision case.
func NewConfidenceScoreRegistry(scorers ...ports.ConfidenceScorer) *ConfidenceScoreRegistry {
	byContentType := make(map[domain.ContentType]ports.ConfidenceScorer, len(scorers))
	for _, s := range scorers {
		for _, ct := range s.ContentTypes() {
			byContentType[ct] = s
		}
	}
	return &ConfidenceScoreRegistry{byContentType: byContentType}
}

// ConfidenceScore implements ports.ConfidenceScoreResolver.
func (r *ConfidenceScoreRegistry) ConfidenceScore(ctx context.Context, contentType domain.ContentType, fingerprint domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	return r.resolve(contentType).ConfidenceScore(ctx, fingerprint, candidates)
}

func (r *ConfidenceScoreRegistry) resolve(contentType domain.ContentType) ports.ConfidenceScorer {
	s, ok := r.byContentType[contentType]
	if !ok {
		return NoopConfidenceScorer{}
	}
	return s
}

// NoopConfidenceScorer is the default ports.ConfidenceScorer implementation:
// ConfidenceScoreRegistry's fallback when no implementation is registered
// for a content type. ConfidenceScore returns candidates unchanged (Score
// stays whatever the caller already set, i.e. zero) — a content type with
// no scorer yet simply never produces a trusted score, the same "no cost to
// opt out, no crash either" treatment NoopIdentifier gets. Deliberately not
// registered under any domain.ContentType itself (ContentTypes returns
// nil); the registry falls back to it directly rather than looking it up by
// content type.
type NoopConfidenceScorer struct{}

var _ ports.ConfidenceScorer = NoopConfidenceScorer{}

// ContentTypes implements ports.ConfidenceScorer. NoopConfidenceScorer is
// never looked up by content type — ConfidenceScoreRegistry falls back to
// it directly — so this returns nil.
func (NoopConfidenceScorer) ContentTypes() []domain.ContentType { return nil }

// ConfidenceScore implements ports.ConfidenceScorer: returns candidates
// unchanged, regardless of fingerprint.
func (NoopConfidenceScorer) ConfidenceScore(_ context.Context, _ domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	return candidates, nil
}
