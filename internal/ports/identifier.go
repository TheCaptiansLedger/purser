package ports

import (
	"context"
	"purser/internal/domain"
)

// Identifier is a content-type-scoped capability: given a group's
// consensus domain.Fingerprint, produce a deduplicated
// []domain.MatchCandidate with Tier/Signals/Metadata populated — never
// Score, which is the shared decision service's downstream
// ConfidenceScore-shaped capability's job, not this one's. Fanned out to by
// IdentifierRegistry via ContentTypes(), the same registry pattern
// Grouping/FileFingerprinter/FilenameParser already established — adding a
// new content type's candidate generation is a new Identifier
// implementation, never an edit to the registry or its caller. See
// docs/adr/0025-music-identification-confidence-scoring.md,
// docs/technical/pipeline-music-identifier.md.
type Identifier interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Identify produces every plausible domain.MatchCandidate for one
	// identification group. fingerprint is the group's consensus
	// domain.Fingerprint (FileFingerprinter.Consensus's result); paths is
	// every file's path in the group — needed alongside fingerprint
	// because an acoustic-fingerprint signal (e.g. AcoustID) has to read
	// actual audio bytes per file, data the consensus Fingerprint alone
	// doesn't carry. groupPath/scanRoot are passed through to a
	// filename-fallback capability the same way FilenameParser.Parse
	// receives them. A group with no discoverable candidates at all
	// returns a nil/empty slice, not an error.
	Identify(ctx context.Context, fingerprint domain.Fingerprint, paths []string, groupPath, scanRoot string) ([]domain.MatchCandidate, error)
}

// IdentifierResolver is the fan-out dispatch capability a candidate-
// generation caller depends on: given a Job's already-resolved content
// type, look up the matching registered Identifier implementation (or fall
// back to a no-op default if none is registered) and call through to it.
// Kept distinct from Identifier itself for the same reason
// GroupingResolver is kept distinct from Grouping — a single Identifier
// implementation never sees a contentType argument; the resolver is what
// decides which one to call.
type IdentifierResolver interface {
	Identify(ctx context.Context, contentType domain.ContentType, fingerprint domain.Fingerprint, paths []string, groupPath, scanRoot string) ([]domain.MatchCandidate, error)
}
