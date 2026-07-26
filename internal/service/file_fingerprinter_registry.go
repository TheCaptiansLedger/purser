package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// FileFingerprinterRegistry implements ports.FileFingerprinterResolver by
// dispatching to whichever registered ports.FileFingerprinter declares the
// requested domain.ContentType via ContentTypes(), falling back to
// NoopFingerprinter when none is registered — the same ContentTypes()-fan-
// out registry pattern GroupingRegistry already established, per
// docs/adr/0024-pipeline-core.md. Adding a new content type's fingerprinter
// means constructing NewFileFingerprinterRegistry with one more
// implementation at the composition root, never editing this type.
type FileFingerprinterRegistry struct {
	byContentType map[domain.ContentType]ports.FileFingerprinter
}

var _ ports.FileFingerprinterResolver = (*FileFingerprinterRegistry)(nil)

// NewFileFingerprinterRegistry builds a FileFingerprinterRegistry from
// fingerprinters, indexing each by every domain.ContentType it declares via
// ContentTypes(). A later entry declaring a ContentType already claimed by
// an earlier one overwrites it — construction order matters only in that
// (deliberately unlikely) collision case.
func NewFileFingerprinterRegistry(fingerprinters ...ports.FileFingerprinter) *FileFingerprinterRegistry {
	byContentType := make(map[domain.ContentType]ports.FileFingerprinter, len(fingerprinters))
	for _, f := range fingerprinters {
		for _, ct := range f.ContentTypes() {
			byContentType[ct] = f
		}
	}
	return &FileFingerprinterRegistry{byContentType: byContentType}
}

// Fingerprint implements ports.FileFingerprinterResolver.
func (r *FileFingerprinterRegistry) Fingerprint(ctx context.Context, contentType domain.ContentType, path string, discNumberGuess int) (domain.Fingerprint, error) {
	return r.resolve(contentType).Fingerprint(ctx, path, discNumberGuess)
}

// Consensus implements ports.FileFingerprinterResolver.
func (r *FileFingerprinterRegistry) Consensus(ctx context.Context, contentType domain.ContentType, fingerprints []domain.Fingerprint) (domain.Fingerprint, error) {
	return r.resolve(contentType).Consensus(ctx, fingerprints)
}

func (r *FileFingerprinterRegistry) resolve(contentType domain.ContentType) ports.FileFingerprinter {
	f, ok := r.byContentType[contentType]
	if !ok {
		return NoopFingerprinter{}
	}
	return f
}

// NoopFingerprinter is the default ports.FileFingerprinter implementation:
// FileFingerprinterRegistry's fallback when no implementation is registered
// for a content type. Both methods return an empty domain.Fingerprint{} —
// a content type with no fingerprinter yet simply gets no identification
// signal, the same "no cost to opt out, no crash either" treatment
// IdentityGrouping gets. Deliberately not registered under any
// domain.ContentType itself (ContentTypes returns nil); the registry falls
// back to it directly rather than looking it up by content type.
type NoopFingerprinter struct{}

var _ ports.FileFingerprinter = NoopFingerprinter{}

// ContentTypes implements ports.FileFingerprinter. NoopFingerprinter is
// never looked up by content type — FileFingerprinterRegistry falls back to
// it directly — so this returns nil.
func (NoopFingerprinter) ContentTypes() []domain.ContentType { return nil }

// Fingerprint implements ports.FileFingerprinter: always an empty
// domain.Fingerprint{}, no error.
func (NoopFingerprinter) Fingerprint(_ context.Context, _ string, _ int) (domain.Fingerprint, error) {
	return domain.Fingerprint{}, nil
}

// Consensus implements ports.FileFingerprinter: always an empty
// domain.Fingerprint{}, no error, regardless of input.
func (NoopFingerprinter) Consensus(_ context.Context, _ []domain.Fingerprint) (domain.Fingerprint, error) {
	return domain.Fingerprint{}, nil
}
