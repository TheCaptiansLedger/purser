package ports

import (
	"context"
	"purser/internal/domain"
)

// FileFingerprinter is a content-type-scoped capability: what
// identification-relevant data can be extracted from a discovered file, and
// how a group of per-file results reduces to one consensus
// domain.Fingerprint, differs by content type (embedded tags for music,
// perceptual hash for video, ISBN for books). Fanned out to by
// FileFingerprinterRegistry via ContentTypes(), the same registry pattern
// Grouping already established — adding a new content type's fingerprinter
// is a new FileFingerprinter implementation, never an edit to the registry
// or ScanExecutor. See docs/adr/0024-pipeline-core.md,
// docs/adr/0025-music-identification-confidence-scoring.md.
type FileFingerprinter interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Fingerprint extracts one discovered file's raw tags/metadata.
	// discNumberGuess is the grouping implementation's folder-derived disc
	// guess (ports.GroupingResult.DiscNumber, 0 if none) — an embedded
	// disc-number tag, when present, overrides it in the returned
	// Fingerprint's Metadata. The result is never persisted per-file; it
	// exists only to feed Consensus below.
	Fingerprint(ctx context.Context, path string, discNumberGuess int) (domain.Fingerprint, error)

	// Consensus reduces every file's Fingerprint in one identification
	// group (files sharing a GroupKey) into the single domain.Fingerprint
	// written onto every domain.UnmatchedFile row in that group. Never
	// resolves a genuine conflict (e.g. disagreeing embedded release IDs)
	// to one value — conflicting facts are recorded, not silently picked
	// between, so a later scoring step can see them.
	Consensus(ctx context.Context, fingerprints []domain.Fingerprint) (domain.Fingerprint, error)
}

// FileFingerprinterResolver is the fan-out dispatch capability ScanExecutor
// depends on: given a Job's already-resolved content type, look up the
// matching registered FileFingerprinter implementation (or fall back to a
// no-op default if none is registered) and call through to it. Kept
// distinct from FileFingerprinter itself for the same reason
// GroupingResolver is kept distinct from Grouping — a single
// FileFingerprinter implementation never sees a contentType argument; the
// resolver is what decides which one to call.
type FileFingerprinterResolver interface {
	Fingerprint(ctx context.Context, contentType domain.ContentType, path string, discNumberGuess int) (domain.Fingerprint, error)
	Consensus(ctx context.Context, contentType domain.ContentType, fingerprints []domain.Fingerprint) (domain.Fingerprint, error)
}
