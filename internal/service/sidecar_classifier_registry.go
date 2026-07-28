package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// SidecarClassifierRegistry implements ports.SidecarClassifierResolver by
// dispatching to whichever registered ports.SidecarClassifier declares the
// requested domain.ContentType via ContentTypes(), falling back to
// NoopClassifier when none is registered — the same ContentTypes()-fan-out
// registry pattern as GroupingRegistry. Adding a new content type's
// classification rule means constructing NewSidecarClassifierRegistry with
// one more implementation at the composition root, never editing this type.
type SidecarClassifierRegistry struct {
	byContentType map[domain.ContentType]ports.SidecarClassifier
}

var _ ports.SidecarClassifierResolver = (*SidecarClassifierRegistry)(nil)

// NewSidecarClassifierRegistry builds a SidecarClassifierRegistry from
// classifiers, indexing each by every domain.ContentType it declares via
// ContentTypes(). A later entry declaring a ContentType already claimed by
// an earlier one overwrites it — construction order matters only in that
// (deliberately unlikely) collision case.
func NewSidecarClassifierRegistry(classifiers ...ports.SidecarClassifier) *SidecarClassifierRegistry {
	byContentType := make(map[domain.ContentType]ports.SidecarClassifier, len(classifiers))
	for _, c := range classifiers {
		for _, ct := range c.ContentTypes() {
			byContentType[ct] = c
		}
	}
	return &SidecarClassifierRegistry{byContentType: byContentType}
}

// Classify implements ports.SidecarClassifierResolver.
func (r *SidecarClassifierRegistry) Classify(ctx context.Context, contentType domain.ContentType, path string) (ports.SidecarKind, error) {
	c, ok := r.byContentType[contentType]
	if !ok {
		c = NoopClassifier{}
	}
	return c.Classify(ctx, path)
}

// NoopClassifier is the default ports.SidecarClassifier implementation:
// SidecarClassifierRegistry's fallback when no implementation is registered
// for a content type. Every path classifies as SidecarKindNone — every file
// proceeds through the pipeline normally, the same "no cost to opt out"
// treatment every other default capability in this build gets. Deliberately
// not registered under any domain.ContentType itself (ContentTypes returns
// nil); SidecarClassifierRegistry falls back to it directly rather than
// looking it up by content type.
type NoopClassifier struct{}

var _ ports.SidecarClassifier = NoopClassifier{}

// ContentTypes implements ports.SidecarClassifier. NoopClassifier is never
// looked up by content type — SidecarClassifierRegistry falls back to it
// directly — so this returns nil.
func (NoopClassifier) ContentTypes() []domain.ContentType { return nil }

// Classify implements ports.SidecarClassifier: every path classifies as
// SidecarKindNone.
func (NoopClassifier) Classify(_ context.Context, _ string) (ports.SidecarKind, error) {
	return ports.SidecarKindNone, nil
}
