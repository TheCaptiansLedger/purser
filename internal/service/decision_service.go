package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// DecisionService is the shared, content-type-agnostic decide step of
// docs/adr/0024-pipeline-core.md's scan/fingerprint/identify/decide/persist
// pipeline. It runs once per group after a content type's ConfidenceScorer
// has populated every domain.MatchCandidate's Score: every candidate whose
// Score clears the configured threshold — 0, 1, or N of them, one per
// independently scored provider (StashDB and ThePornDB score independently,
// per docs/adr/0027-provider-independence.md's "no server-side picking a
// winner") — is dispatched together to the ContentTypes()-registered
// ports.Persister for that content type. Nothing about any specific content
// type belongs here — what candidates mean, how they're scored, and how
// multiple above-threshold candidates get merged into one set of rows is
// entirely upstream/module-owned; this type only filters by a number and
// calls through a port. See docs/technical/pipeline-music-persist.md.
type DecisionService struct {
	threshold float64
	persister ports.PersisterResolver
}

// NewDecisionService builds a DecisionService that auto-imports every
// candidate in a group whose Score is at or above threshold (typically
// config.Pipeline.ConfidenceThreshold), dispatching them together to
// persister for the actual persistence cascade.
func NewDecisionService(threshold float64, persister ports.PersisterResolver) *DecisionService {
	return &DecisionService{threshold: threshold, persister: persister}
}

// Decide evaluates one group's candidates and, if any clear the configured
// threshold, persists all of them together. An empty/nil candidates list is
// left alone — a group with no candidates at all is a normal outcome (M7
// found nothing), not an error — and reports persisted=false, err=nil. A
// non-empty list none of whose candidates clear threshold also reports
// persisted=false, err=nil: the group stays in the UnmatchedFile review
// queue either way, decided by the caller, not by this method. Candidates
// are passed to Persist in the order they were given — DecisionService never
// sorts or ranks them, per ADR-0027's no-server-side-picking-a-winner stance
// extended to ordering. Only a cleared threshold calls through to persister;
// its error, if any, is returned unchanged so the caller can leave the
// group's rows untouched on failure, per
// docs/technical/pipeline-music-persist.md's "No true cross-repository
// transaction" section.
func (s *DecisionService) Decide(ctx context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) (persisted bool, err error) {
	if len(candidates) == 0 {
		return false, nil
	}

	var cleared []domain.MatchCandidate
	for _, c := range candidates {
		if c.Score >= s.threshold {
			cleared = append(cleared, c)
		}
	}
	if len(cleared) == 0 {
		return false, nil
	}

	if err := s.persister.Persist(ctx, contentType, fingerprint, cleared, files); err != nil {
		return false, err
	}
	return true, nil
}
