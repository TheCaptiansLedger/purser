package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// FilenameParserRegistry implements ports.FilenameParserResolver by
// dispatching to whichever registered ports.FilenameParser declares the
// requested domain.ContentType via ContentTypes(), falling back to
// NoopFilenameParser when none is registered — the same ContentTypes()-
// fan-out registry pattern GroupingRegistry and FileFingerprinterRegistry
// already established, per docs/adr/0024-pipeline-core.md. Adding a new
// content type's filename/folder-name parser means constructing
// NewFilenameParserRegistry with one more implementation at the
// composition root, never editing this type.
type FilenameParserRegistry struct {
	byContentType map[domain.ContentType]ports.FilenameParser
}

var _ ports.FilenameParserResolver = (*FilenameParserRegistry)(nil)

// NewFilenameParserRegistry builds a FilenameParserRegistry from parsers,
// indexing each by every domain.ContentType it declares via ContentTypes().
// A later entry declaring a ContentType already claimed by an earlier one
// overwrites it — construction order matters only in that (deliberately
// unlikely) collision case.
func NewFilenameParserRegistry(parsers ...ports.FilenameParser) *FilenameParserRegistry {
	byContentType := make(map[domain.ContentType]ports.FilenameParser, len(parsers))
	for _, p := range parsers {
		for _, ct := range p.ContentTypes() {
			byContentType[ct] = p
		}
	}
	return &FilenameParserRegistry{byContentType: byContentType}
}

// Parse implements ports.FilenameParserResolver.
func (r *FilenameParserRegistry) Parse(ctx context.Context, contentType domain.ContentType, groupPath, scanRoot string) (first, second string, ok bool) {
	return r.resolve(contentType).Parse(ctx, groupPath, scanRoot)
}

func (r *FilenameParserRegistry) resolve(contentType domain.ContentType) ports.FilenameParser {
	p, registered := r.byContentType[contentType]
	if !registered {
		return NoopFilenameParser{}
	}
	return p
}

// NoopFilenameParser is the default ports.FilenameParser implementation:
// FilenameParserRegistry's fallback when no implementation is registered
// for a content type. Parse always returns ok = false — a content type
// with no parser yet simply gets no filename-fallback guess, the same
// "no cost to opt out" treatment IdentityGrouping and NoopFingerprinter
// get. Deliberately not registered under any domain.ContentType itself
// (ContentTypes returns nil); the registry falls back to it directly
// rather than looking it up by content type.
type NoopFilenameParser struct{}

var _ ports.FilenameParser = NoopFilenameParser{}

// ContentTypes implements ports.FilenameParser. NoopFilenameParser is
// never looked up by content type — FilenameParserRegistry falls back to
// it directly — so this returns nil.
func (NoopFilenameParser) ContentTypes() []domain.ContentType { return nil }

// Parse implements ports.FilenameParser: always ok = false, regardless of
// input.
func (NoopFilenameParser) Parse(_ context.Context, _, _ string) (first, second string, ok bool) {
	return "", "", false
}
