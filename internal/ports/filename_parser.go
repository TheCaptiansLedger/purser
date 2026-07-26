package ports

import (
	"context"
	"purser/internal/domain"
)

// FilenameParser is a content-type-scoped capability: given a group's
// folder path, produce a best-effort (artist, album) guess for the
// tag-derived-candidate-generation fallback described in
// docs/adr/0025-music-identification-confidence-scoring.md and
// docs/technical/pipeline-music-filename-parser.md. Fanned out to by
// FilenameParserRegistry via ContentTypes(), the same registry pattern
// Grouping and FileFingerprinter already established — adding a new
// content type's parsing heuristics is a new FilenameParser implementation,
// never an edit to the registry or its caller.
//
// Pure string parsing: no I/O. Never constructs or touches a
// domain.MatchCandidate — that's the caller's job (M7's candidate-
// generation gate), which also decides *when* Parse is worth calling
// (only after tag-derived generation finds nothing).
type FilenameParser interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Parse produces a best-effort (artist, album) guess from groupPath, the
	// folder path of one identification group (already at the right level —
	// for a multi-disc group this is the album-level grandparent, per
	// Grouping's roll-up). scanRoot is the configured root groupPath was
	// discovered under; an implementation uses it to recognize when a
	// candidate parent-directory guess is actually the scan root itself
	// (no real artist folder above the album), not another content-derived
	// signal. ok is false when nothing usable could be extracted; a partial
	// result (only one of artist/album populated) is still returned with
	// ok = true.
	Parse(ctx context.Context, groupPath, scanRoot string) (artist, album string, ok bool)
}

// FilenameParserResolver is the fan-out dispatch capability a candidate-
// generation caller depends on: given a Job's already-resolved content
// type, a group's folder path, and the scan root it was discovered under,
// look up the matching registered FilenameParser implementation (or fall
// back to a no-guess default if none is registered) and return its result.
// Kept distinct from FilenameParser itself for the same reason
// GroupingResolver is kept distinct from Grouping — a single FilenameParser
// implementation never sees a contentType argument; the resolver is what
// decides which one to call.
type FilenameParserResolver interface {
	Parse(ctx context.Context, contentType domain.ContentType, groupPath, scanRoot string) (artist, album string, ok bool)
}
