// This file is the ports.TemplateDataBuilder implementation for
// domain.ContentTypeAdult — AD10 (issue #564), mirroring
// internal/adapters/pipeline/music/template_data_builder.go's shape: the
// cross-entity lookups (LibraryEntry/Studio, ItemPerson/Person credits,
// ExternalID) a naming template needs, that the generic service.Organizer
// itself has no way to gather. Organizer mechanics themselves needed zero
// new code for this issue — the same generic ContentTypes()-fan-out
// registry Music's M11a already established.
package afterdark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// templateDataBuilderPageSize bounds each internal ItemPersonRepository.List
// page BuildTemplateData drains while assembling item's full performer
// credit list — same "small, fixed inner page size" convention
// AfterDarkBrowseService's innerListPageSize uses for the same repository.
const templateDataBuilderPageSize = 100

// TemplateDataBuilder implements ports.TemplateDataBuilder for
// domain.ContentTypeAdult — Studio, Performers, and Scene title/date/code,
// per issue #564. The generic service.Organizer handles Ext and raw
// Item/MediaFile metadata passthrough itself; this type only supplies the
// curated, AfterDark-specific keys that need cross-entity resolution.
type TemplateDataBuilder struct {
	libraryEntries ports.LibraryEntryRepository
	itemPeople     ports.ItemPersonRepository
	people         ports.PersonRepository
	externalIDs    ports.ExternalIDRepository

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.TemplateDataBuilder = (*TemplateDataBuilder)(nil)

// NewTemplateDataBuilder constructs a TemplateDataBuilder backed by
// libraryEntries, itemPeople, people, and externalIDs. Reuses the same
// Option/WithLogger/WithTracerProvider used by
// New/NewIdentifier/NewConfidenceScorer/NewPersister — all share the same
// {logger, tracerProvider} shape.
func NewTemplateDataBuilder(libraryEntries ports.LibraryEntryRepository, itemPeople ports.ItemPersonRepository, people ports.PersonRepository, externalIDs ports.ExternalIDRepository, opts ...Option) *TemplateDataBuilder {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &TemplateDataBuilder{
		libraryEntries: libraryEntries,
		itemPeople:     itemPeople,
		people:         people,
		externalIDs:    externalIDs,
		logger:         o.logger.With("component", "adapters.pipeline.afterdark.template_data_builder"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.TemplateDataBuilder.
func (b *TemplateDataBuilder) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// BuildTemplateData implements ports.TemplateDataBuilder. item.LibraryEntryID
// is Purser's own referential integrity — always set by Persister's
// getOrCreateStudio/createScene for every real scene — so a missing/
// unresolvable Studio is treated as a hard error here, not degraded
// gracefully: fabricating an empty Studio value would silently organize the
// file into a garbled path instead of surfacing a real data problem.
// SceneCode is different: not every scene is JAV-coded, so a missing
// jav_code ExternalID is simply an empty field, never an error.
func (b *TemplateDataBuilder) BuildTemplateData(ctx context.Context, item *domain.Item) (map[string]any, error) {
	ctx, span := b.tracer.Start(ctx, "afterdark.template_data_builder.build_template_data", trace.WithAttributes(
		attribute.String("afterdark.item_id", item.ID),
	))
	defer span.End()

	studio, err := b.libraryEntries.Get(ctx, item.LibraryEntryID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/afterdark: getting studio %q for item %q: %w", item.LibraryEntryID, item.ID, err)
	}

	performers, err := b.performerNames(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/afterdark: getting performers for item %q: %w", item.ID, err)
	}

	code, err := b.sceneCode(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/afterdark: getting scene code for item %q: %w", item.ID, err)
	}

	var date string
	if item.Date != nil {
		date = item.Date.Format("2006-01-02")
	}

	return map[string]any{
		"Studio":     studio.Name,
		"Performers": performers,
		"SceneTitle": item.Title,
		"SceneDate":  date,
		"SceneCode":  code,
	}, nil
}

// performerNames fully drains ItemPersonRepository.List(itemID) across
// every underlying page, resolving each row's PersonID to a display name —
// the row's own CreditedAs if the scene credited a different name than the
// Person's canonical one, falling back to Person.Name otherwise. Order
// follows whatever the repository itself returns, the same "no
// server-side reordering of what's already there" restraint every other
// list-and-render step in this codebase gives.
func (b *TemplateDataBuilder) performerNames(ctx context.Context, itemID string) ([]string, error) {
	var names []string
	pageToken := ""
	for {
		rows, next, err := b.itemPeople.List(ctx, itemID, "", templateDataBuilderPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			person, err := b.people.Get(ctx, row.PersonID)
			if err != nil {
				return nil, err
			}
			name := row.CreditedAs
			if name == "" {
				name = person.Name
			}
			names = append(names, name)
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return names, nil
}

// sceneCode looks up itemID's jav_code ExternalID (linked by
// Persister.getOrCreateScene whenever any merged candidate resolved one),
// returning "" if none exists — most scenes aren't JAV-coded, so this is
// the common case, not an error.
func (b *TemplateDataBuilder) sceneCode(ctx context.Context, itemID string) (string, error) {
	ext, err := b.externalIDs.Get(ctx, domain.EntityTypeItem, itemID, string(domain.ExternalIDSourceJAVCode))
	if err == nil {
		return ext.Value, nil
	}
	if errors.Is(err, ports.ErrNotFound) {
		return "", nil
	}
	return "", err
}
