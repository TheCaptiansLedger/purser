package ports

import (
	"context"
	"purser/internal/domain"
)

// FilenameParser is a content-type-scoped capability: given a group's
// folder path, produce a best-effort pair of content-specific guess
// strings for the candidate-generation fallback described in
// docs/adr/0024-pipeline-core.md's "Grouping is a pluggable, fanned-out
// capability" section. Fanned out to by FilenameParserRegistry via
// ContentTypes(), the same registry pattern Grouping and FileFingerprinter
// already established — adding a new content type's parsing heuristics is
// a new FilenameParser implementation, never an edit to the registry or
// its caller. Music's implementation (a (artist, album) guess) is
// documented in docs/adr/0025-music-identification-confidence-scoring.md
// and docs/technical/pipeline-music-filename-parser.md; other content
// types' implementations name and interpret the returned pair however
// fits their own domain (e.g. AfterDark's (studio, title)).
//
// Pure string parsing: no I/O. Never constructs or touches a
// domain.MatchCandidate — that's the caller's job (Music's M7 candidate-
// generation gate is one such caller), which also decides *when* Parse is
// worth calling (Music's convention: only after tag-derived generation
// finds nothing — not necessarily every content type's).
type FilenameParser interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Parse produces a best-effort, content-specific pair of guess strings
	// from groupPath, the folder path of one identification group (already
	// at the right level — for Music, a multi-disc group's album-level
	// grandparent, per Grouping's roll-up). scanRoot is the configured
	// root groupPath was discovered under; an implementation uses it to
	// recognize when a candidate parent-directory guess is actually the
	// scan root itself (no real parent folder carrying signal above the
	// leaf), not another content-derived signal. ok is false when nothing
	// usable could be extracted; a partial result (only one of the two
	// return strings populated) is still returned with ok = true.
	Parse(ctx context.Context, groupPath, scanRoot string) (first, second string, ok bool)
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
	Parse(ctx context.Context, contentType domain.ContentType, groupPath, scanRoot string) (first, second string, ok bool)
}
