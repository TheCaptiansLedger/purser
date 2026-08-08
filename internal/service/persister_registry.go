package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// PersisterRegistry implements ports.PersisterResolver by dispatching to
// whichever registered ports.Persister declares the requested
// domain.ContentType via ContentTypes(), falling back to NoopPersister when
// none is registered — the same ContentTypes()-fan-out registry pattern
// ConfidenceScoreRegistry/IdentifierRegistry/GroupingRegistry already
// established, per docs/adr/0024-pipeline-core.md. Adding a new content
// type's persistence cascade means constructing NewPersisterRegistry with
// one more implementation at the composition root, never editing this type.
type PersisterRegistry struct {
	byContentType map[domain.ContentType]ports.Persister
}

var _ ports.PersisterResolver = (*PersisterRegistry)(nil)

// NewPersisterRegistry builds a PersisterRegistry from persisters, indexing
// each by every domain.ContentType it declares via ContentTypes(). A later
// entry declaring a ContentType already claimed by an earlier one overwrites
// it — construction order matters only in that (deliberately unlikely)
// collision case.
func NewPersisterRegistry(persisters ...ports.Persister) *PersisterRegistry {
	byContentType := make(map[domain.ContentType]ports.Persister, len(persisters))
	for _, p := range persisters {
		for _, ct := range p.ContentTypes() {
			byContentType[ct] = p
		}
	}
	return &PersisterRegistry{byContentType: byContentType}
}

// Persist implements ports.PersisterResolver.
func (r *PersisterRegistry) Persist(ctx context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) error {
	return r.resolve(contentType).Persist(ctx, fingerprint, candidates, files)
}

func (r *PersisterRegistry) resolve(contentType domain.ContentType) ports.Persister {
	p, ok := r.byContentType[contentType]
	if !ok {
		return NoopPersister{}
	}
	return p
}

// NoopPersister is the default ports.Persister implementation:
// PersisterRegistry's fallback when no implementation is registered for a
// content type. Persist always returns nil — a content type with no
// persister yet simply never creates library rows, the same "no cost to
// opt out, no crash either" treatment NoopIdentifier/NoopConfidenceScorer
// get. Deliberately not registered under any domain.ContentType itself
// (ContentTypes returns nil); the registry falls back to it directly rather
// than looking it up by content type.
type NoopPersister struct{}

var _ ports.Persister = NoopPersister{}

// ContentTypes implements ports.Persister. NoopPersister is never looked up
// by content type — PersisterRegistry falls back to it directly — so this
// returns nil.
func (NoopPersister) ContentTypes() []domain.ContentType { return nil }

// Persist implements ports.Persister: always nil, regardless of input.
func (NoopPersister) Persist(_ context.Context, _ *domain.Fingerprint, _ []domain.MatchCandidate, _ []*domain.UnmatchedFile) error {
	return nil
}
