package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// DecisionService is the shared, content-type-agnostic decide step of
// docs/adr/0024-pipeline-core.md's scan/fingerprint/identify/decide/persist
// pipeline. It runs once per group after a content type's ConfidenceScorer
// has populated every domain.MatchCandidate's Score: if the top candidate's
// Score clears the configured threshold, it dispatches to the
// ContentTypes()-registered ports.Persister for that content type. Nothing
// about any specific content type belongs here — what candidates mean and
// how they're scored is entirely upstream/module-owned; this type only
// compares a number and calls through a port. See
// docs/technical/pipeline-music-persist.md.
type DecisionService struct {
	threshold float64
	persister ports.PersisterResolver
}

// NewDecisionService builds a DecisionService that auto-imports a group
// whose top candidate's Score is at or above threshold (typically
// config.Pipeline.ConfidenceThreshold), dispatching to persister for the
// actual persistence cascade.
func NewDecisionService(threshold float64, persister ports.PersisterResolver) *DecisionService {
	return &DecisionService{threshold: threshold, persister: persister}
}

// Decide evaluates one group's candidates and, if the top-scored one clears
// the configured threshold, persists it. An empty/nil candidates list is
// left alone — a group with no candidates at all is a normal outcome (M7
// found nothing), not an error — and reports persisted=false, err=nil. A
// non-empty list whose best Score falls below threshold also reports
// persisted=false, err=nil: the group stays in the UnmatchedFile review
// queue either way, decided by the caller, not by this method. Only a
// cleared threshold calls through to persister; its error, if any, is
// returned unchanged so the caller can leave the group's rows untouched on
// failure, per docs/technical/pipeline-music-persist.md's "No true
// cross-repository transaction" section.
func (s *DecisionService) Decide(ctx context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) (persisted bool, err error) {
	if len(candidates) == 0 {
		return false, nil
	}

	top := candidates[0]
	for _, c := range candidates[1:] {
		if c.Score > top.Score {
			top = c
		}
	}

	if top.Score < s.threshold {
		return false, nil
	}

	if err := s.persister.Persist(ctx, contentType, fingerprint, top, files); err != nil {
		return false, err
	}
	return true, nil
}
