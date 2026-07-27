package ports

import (
	"context"
	"purser/internal/domain"
)

// Persister is a content-type-scoped capability: given a group's consensus
// domain.Fingerprint, the winning domain.MatchCandidate (either the shared
// DecisionService's auto-import pick or a human's manual accept), and the
// group's domain.UnmatchedFile rows, create the real library rows (Artist/
// Release Group/Release/Item/MediaFile for Music; the equivalent cascade for
// any other content type) and return. What that cascade looks like is
// entirely module-owned — nothing about it belongs in the shared decision
// service that calls it. Fanned out to by PersisterResolver via
// ContentTypes(), the same registry pattern
// Grouping/FileFingerprinter/Identifier/ConfidenceScorer already
// established — adding a new content type's persistence cascade is a new
// Persister implementation, never an edit to the registry or its caller.
// See docs/adr/0024-pipeline-core.md, docs/technical/pipeline-music-persist.md.
type Persister interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Persist creates the real library rows for candidate against
	// fingerprint and files. On success, the caller is responsible for
	// removing files from the review queue — Persist itself only creates,
	// never deletes.
	Persist(ctx context.Context, fingerprint *domain.Fingerprint, candidate domain.MatchCandidate, files []*domain.UnmatchedFile) error
}

// PersisterResolver is the fan-out dispatch capability a decide/persist
// caller depends on: given a group's already-resolved content type, look up
// the matching registered Persister implementation (or fall back to a no-op
// default if none is registered) and call through to it. Kept distinct from
// Persister itself for the same reason ConfidenceScoreResolver is kept
// distinct from ConfidenceScorer — a single Persister implementation never
// sees a contentType argument; the resolver is what decides which one to
// call.
type PersisterResolver interface {
	Persist(ctx context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidate domain.MatchCandidate, files []*domain.UnmatchedFile) error
}
