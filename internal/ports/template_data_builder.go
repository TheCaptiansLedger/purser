package ports

import (
	"context"
	"purser/internal/domain"
)

// TemplateDataBuilder is a content-type-scoped capability: given an Item,
// gather whatever cross-entity data (Group, release, library entry, etc.)
// a naming template needs and return it as an open map — the generic
// Organizer has no idea what the map's keys mean, only that it's what gets
// handed to text/template.Execute. Fanned out to by
// TemplateDataBuilderResolver via ContentTypes(), the same registry
// pattern Grouping/FileFingerprinter/Identifier/ConfidenceScorer/Persister
// already established — adding a new content type's naming data is a new
// TemplateDataBuilder implementation, never an edit to the Organizer or
// the registry. See docs/adr/0024-pipeline-core.md,
// docs/technical/pipeline-music-organizer.md.
type TemplateDataBuilder interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// BuildTemplateData gathers item's naming-template data as a map
	// suitable for text/template.Execute.
	BuildTemplateData(ctx context.Context, item *domain.Item) (map[string]any, error)
}

// TemplateDataBuilderResolver is the fan-out dispatch capability the
// Organizer depends on: given a MediaFile's Item's already-resolved
// content type, look up the matching registered TemplateDataBuilder
// implementation (or fall back to a no-op default if none is registered)
// and call through to it. Kept distinct from TemplateDataBuilder itself
// for the same reason PersisterResolver is kept distinct from Persister —
// a single TemplateDataBuilder implementation never sees a contentType
// argument; the resolver is what decides which one to call.
type TemplateDataBuilderResolver interface {
	BuildTemplateData(ctx context.Context, contentType domain.ContentType, item *domain.Item) (map[string]any, error)
}
