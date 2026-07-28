package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// TemplateDataBuilderRegistry implements ports.TemplateDataBuilderResolver
// by dispatching to whichever registered ports.TemplateDataBuilder
// declares the requested domain.ContentType via ContentTypes(), falling
// back to NoopTemplateDataBuilder when none is registered — the same
// ContentTypes()-fan-out registry pattern PersisterRegistry/
// ConfidenceScoreRegistry/IdentifierRegistry/GroupingRegistry already
// established, per docs/adr/0024-pipeline-core.md. Adding a new content
// type's naming data means constructing NewTemplateDataBuilderRegistry
// with one more implementation at the composition root, never editing
// this type.
type TemplateDataBuilderRegistry struct {
	byContentType map[domain.ContentType]ports.TemplateDataBuilder
}

var _ ports.TemplateDataBuilderResolver = (*TemplateDataBuilderRegistry)(nil)

// NewTemplateDataBuilderRegistry builds a TemplateDataBuilderRegistry from
// builders, indexing each by every domain.ContentType it declares via
// ContentTypes(). A later entry declaring a ContentType already claimed by
// an earlier one overwrites it — construction order matters only in that
// (deliberately unlikely) collision case.
func NewTemplateDataBuilderRegistry(builders ...ports.TemplateDataBuilder) *TemplateDataBuilderRegistry {
	byContentType := make(map[domain.ContentType]ports.TemplateDataBuilder, len(builders))
	for _, b := range builders {
		for _, ct := range b.ContentTypes() {
			byContentType[ct] = b
		}
	}
	return &TemplateDataBuilderRegistry{byContentType: byContentType}
}

// BuildTemplateData implements ports.TemplateDataBuilderResolver.
func (r *TemplateDataBuilderRegistry) BuildTemplateData(ctx context.Context, contentType domain.ContentType, item *domain.Item) (map[string]any, error) {
	return r.resolve(contentType).BuildTemplateData(ctx, item)
}

func (r *TemplateDataBuilderRegistry) resolve(contentType domain.ContentType) ports.TemplateDataBuilder {
	b, ok := r.byContentType[contentType]
	if !ok {
		return NoopTemplateDataBuilder{}
	}
	return b
}

// NoopTemplateDataBuilder is the default ports.TemplateDataBuilder
// implementation: TemplateDataBuilderRegistry's fallback when no
// implementation is registered for a content type. BuildTemplateData
// always returns an empty map and a nil error — a content type with no
// naming data yet still renders whatever a template's literal text/
// defaults produce, the same "no cost to opt out, no crash either"
// treatment NoopPersister/NoopIdentifier get. Deliberately not registered
// under any domain.ContentType itself (ContentTypes returns nil); the
// registry falls back to it directly rather than looking it up by content
// type.
type NoopTemplateDataBuilder struct{}

var _ ports.TemplateDataBuilder = NoopTemplateDataBuilder{}

// ContentTypes implements ports.TemplateDataBuilder. NoopTemplateDataBuilder
// is never looked up by content type — TemplateDataBuilderRegistry falls
// back to it directly — so this returns nil.
func (NoopTemplateDataBuilder) ContentTypes() []domain.ContentType { return nil }

// BuildTemplateData implements ports.TemplateDataBuilder: always an empty
// map and a nil error, regardless of input.
func (NoopTemplateDataBuilder) BuildTemplateData(_ context.Context, _ *domain.Item) (map[string]any, error) {
	return map[string]any{}, nil
}
