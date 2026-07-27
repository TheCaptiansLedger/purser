package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// IdentifierRegistry implements ports.IdentifierResolver by dispatching to
// whichever registered ports.Identifier declares the requested
// domain.ContentType via ContentTypes(), falling back to NoopIdentifier
// when none is registered — the same ContentTypes()-fan-out registry
// pattern GroupingRegistry/FileFingerprinterRegistry/FilenameParserRegistry
// already established, per docs/adr/0024-pipeline-core.md. Adding a new
// content type's candidate generation means constructing
// NewIdentifierRegistry with one more implementation at the composition
// root, never editing this type.
type IdentifierRegistry struct {
	byContentType map[domain.ContentType]ports.Identifier
}

var _ ports.IdentifierResolver = (*IdentifierRegistry)(nil)

// NewIdentifierRegistry builds an IdentifierRegistry from identifiers,
// indexing each by every domain.ContentType it declares via ContentTypes().
// A later entry declaring a ContentType already claimed by an earlier one
// overwrites it — construction order matters only in that (deliberately
// unlikely) collision case.
func NewIdentifierRegistry(identifiers ...ports.Identifier) *IdentifierRegistry {
	byContentType := make(map[domain.ContentType]ports.Identifier, len(identifiers))
	for _, i := range identifiers {
		for _, ct := range i.ContentTypes() {
			byContentType[ct] = i
		}
	}
	return &IdentifierRegistry{byContentType: byContentType}
}

// Identify implements ports.IdentifierResolver.
func (r *IdentifierRegistry) Identify(ctx context.Context, contentType domain.ContentType, fingerprint domain.Fingerprint, paths []string, groupPath, scanRoot string) ([]domain.MatchCandidate, error) {
	return r.resolve(contentType).Identify(ctx, fingerprint, paths, groupPath, scanRoot)
}

func (r *IdentifierRegistry) resolve(contentType domain.ContentType) ports.Identifier {
	i, ok := r.byContentType[contentType]
	if !ok {
		return NoopIdentifier{}
	}
	return i
}

// NoopIdentifier is the default ports.Identifier implementation:
// IdentifierRegistry's fallback when no implementation is registered for a
// content type. Identify always returns (nil, nil) — a content type with
// no identifier yet simply produces no candidates, the same "no cost to
// opt out, no crash either" treatment NoopFingerprinter/NoopFilenameParser
// get. Deliberately not registered under any domain.ContentType itself
// (ContentTypes returns nil); the registry falls back to it directly
// rather than looking it up by content type.
type NoopIdentifier struct{}

var _ ports.Identifier = NoopIdentifier{}

// ContentTypes implements ports.Identifier. NoopIdentifier is never looked
// up by content type — IdentifierRegistry falls back to it directly — so
// this returns nil.
func (NoopIdentifier) ContentTypes() []domain.ContentType { return nil }

// Identify implements ports.Identifier: always (nil, nil), regardless of
// input.
func (NoopIdentifier) Identify(_ context.Context, _ domain.Fingerprint, _ []string, _, _ string) ([]domain.MatchCandidate, error) {
	return nil, nil
}
