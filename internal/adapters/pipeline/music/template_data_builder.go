package music

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TemplateDataBuilder implements ports.TemplateDataBuilder for
// domain.ContentTypeMusic — the cross-entity lookups (Item, Group,
// MusicRelease, LibraryEntry) a Music naming template needs, per
// docs/technical/pipeline-music-organizer.md. The generic
// service.Organizer handles Ext and raw Item/MediaFile metadata
// passthrough itself; this type only supplies the curated, Music-specific
// keys that need cross-entity resolution.
type TemplateDataBuilder struct {
	groups         ports.GroupRepository
	releases       ports.MusicReleaseRepository
	libraryEntries ports.LibraryEntryRepository

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.TemplateDataBuilder = (*TemplateDataBuilder)(nil)

// NewTemplateDataBuilder constructs a TemplateDataBuilder backed by groups,
// releases, and libraryEntries. Reuses the same Option/WithLogger/
// WithTracerProvider used by New/NewIdentifier/NewConfidenceScorer/
// NewPersister — all share the same {logger, tracerProvider} shape.
func NewTemplateDataBuilder(groups ports.GroupRepository, releases ports.MusicReleaseRepository, libraryEntries ports.LibraryEntryRepository, opts ...Option) *TemplateDataBuilder {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &TemplateDataBuilder{
		groups:         groups,
		releases:       releases,
		libraryEntries: libraryEntries,
		logger:         o.logger.With("component", "adapters.pipeline.music.template_data_builder"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.TemplateDataBuilder.
func (b *TemplateDataBuilder) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// BuildTemplateData implements ports.TemplateDataBuilder. item.GroupID and
// item.Metadata["release_id"] are Purser's own referential integrity —
// always set by Persister.buildTrackItem for every real Music track — so a
// missing/unresolvable Group or MusicRelease link is treated as a hard
// error here, not degraded gracefully: fabricating empty ArtistName/
// AlbumTitle values would silently organize the file into a garbled path
// instead of surfacing a real data problem. This is a different judgment
// than TrackNumber below, where a non-numeric value is MusicBrainz's own
// upstream data being sparse/irregular, not a Purser integrity issue.
func (b *TemplateDataBuilder) BuildTemplateData(ctx context.Context, item *domain.Item) (map[string]any, error) {
	ctx, span := b.tracer.Start(ctx, "music.template_data_builder.build_template_data", trace.WithAttributes(
		attribute.String("music.item_id", item.ID),
	))
	defer span.End()

	group, err := b.groups.Get(ctx, item.GroupID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/music: getting group %q for item %q: %w", item.GroupID, item.ID, err)
	}

	releaseID, _ := item.Metadata["release_id"].(string)
	release, err := b.releases.Get(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/music: getting release %q for item %q: %w", releaseID, item.ID, err)
	}

	artist, err := b.libraryEntries.Get(ctx, group.LibraryEntryID)
	if err != nil {
		return nil, fmt.Errorf("adapters/pipeline/music: getting library entry %q for group %q: %w", group.LibraryEntryID, group.ID, err)
	}

	var year int
	if release.Date != nil {
		year = release.Date.Year()
	}

	return map[string]any{
		"TrackTitle":  item.Title,
		"TrackNumber": trackNumber(item.Sequence),
		"DiscNumber":  discNumberOf(item),
		"AlbumTitle":  group.Title,
		// Year comes from MusicRelease.Date, not Group.Year — Group.Year is
		// never populated by Persister (always 0); using it here would
		// silently produce a template that never shows a year.
		"Year":       year,
		"DiscCount":  release.MediumCount,
		"ArtistName": artist.Name,
	}, nil
}

// trackNumber best-effort parses item.Sequence (MusicBrainz's own
// track-position string) as an int for the default template's
// `printf "%02d" .TrackNumber`. Vinyl/cassette releases legitimately carry
// non-numeric positions ("A1", "B3") — falls back to 0 on parse failure,
// the same best-effort convention discNumberOf/parseMBDate already use for
// MusicBrainz's own sparse/irregular data.
func trackNumber(sequence string) int {
	n, err := strconv.Atoi(sequence)
	if err != nil {
		return 0
	}
	return n
}
