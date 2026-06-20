package metadata

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"

	"github.com/google/uuid"
)

// Aggregator fans out metadata queries to all enabled providers for a content
// type, merges results ordered by provider priority, and persists images.
// sources[0] is the primary; all source failures are treated equally — logged
// and skipped. An error is returned only when no sources produced a result.
type Aggregator struct {
	sources   []ports.MetadataSource
	imageRepo ports.ImageRepository
}

// NewAggregator constructs an Aggregator. sources must be ordered by descending
// priority (index 0 = primary).
func NewAggregator(sources []ports.MetadataSource, imageRepo ports.ImageRepository) *Aggregator {
	return &Aggregator{sources: sources, imageRepo: imageRepo}
}

// FindByExternalID fans out to all sources that support contentType, merges
// results ordered by source priority, persists images, and returns the merged
// item. externalID is the provider-specific identifier passed to each source;
// entityID is the internal library_entry UUID used when persisting images.
// All source failures are logged and skipped; an error is returned only if no
// sources returned a result.
func (a *Aggregator) FindByExternalID(ctx context.Context, contentType domain.ContentType, externalID, entityID string) (*domain.ExternalItem, error) {
	matching := a.sourcesFor(contentType)
	if len(matching) == 0 {
		return nil, fmt.Errorf("metadata aggregator: no sources configured for %q", contentType)
	}

	type fanResult struct {
		priority int
		source   string
		item     *domain.ExternalItem
		err      error
	}
	ch := make(chan fanResult, len(matching))
	var wg sync.WaitGroup
	for i, src := range matching {
		wg.Add(1)
		go func(priority int, src ports.MetadataSource) {
			defer wg.Done()
			item, err := src.FindByExternalID(ctx, contentType, externalID)
			ch <- fanResult{priority: priority, source: src.Name(), item: item, err: err}
		}(i, src)
	}
	go func() { wg.Wait(); close(ch) }()

	results := make([]fanResult, len(matching))
	for r := range ch {
		results[r.priority] = r
	}

	var sourced []domain.SourcedExternalItem
	for _, r := range results {
		if r.err != nil {
			slog.Warn("metadata aggregator: source failed", "source", r.source, "error", r.err)
			continue
		}
		if r.item != nil {
			sourced = append(sourced, domain.SourcedExternalItem{Source: r.source, Item: r.item})
		}
	}

	merged := domain.MergeExternalItems(sourced)
	if merged == nil {
		return nil, fmt.Errorf("metadata aggregator: all sources failed for %q in %q", externalID, contentType)
	}

	a.persistImages(ctx, "library_entry", entityID, merged.Images)
	return merged, nil
}

// SearchItems fans out to all sources that support contentType and returns the
// combined results. Errors from individual sources are logged and skipped.
func (a *Aggregator) SearchItems(ctx context.Context, contentType domain.ContentType, title string, limit int) ([]*domain.ExternalItem, error) {
	matching := a.sourcesFor(contentType)

	type result struct{ items []*domain.ExternalItem }
	ch := make(chan result, len(matching))
	var wg sync.WaitGroup
	for _, src := range matching {
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			items, err := src.SearchItems(ctx, contentType, title, limit)
			if err != nil {
				slog.Warn("metadata aggregator: SearchItems failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{items}
		}(src)
	}
	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalItem
	for r := range ch {
		all = append(all, r.items...)
	}
	return all, nil
}

// FetchEntryContent fans out to all sources that support contentType and returns
// the combined item set (first page, up to 100 per source). Errors are logged
// and skipped.
func (a *Aggregator) FetchEntryContent(ctx context.Context, contentType domain.ContentType, entryID string) ([]*domain.ExternalItem, error) {
	matching := a.sourcesFor(contentType)

	type result struct{ items []*domain.ExternalItem }
	ch := make(chan result, len(matching))
	var wg sync.WaitGroup
	for _, src := range matching {
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			_, items, _, err := src.FetchEntryContent(ctx, contentType, entryID, 1, 100)
			if err != nil {
				slog.Warn("metadata aggregator: FetchEntryContent failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{items}
		}(src)
	}
	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalItem
	for r := range ch {
		all = append(all, r.items...)
	}
	return all, nil
}

func (a *Aggregator) persistImages(ctx context.Context, entityType, entityID string, images []domain.ExternalImage) {
	if a.imageRepo == nil || len(images) == 0 {
		return
	}
	stored := make([]domain.StoredImage, 0, len(images))
	for _, img := range images {
		stored = append(stored, domain.StoredImage{
			ID:         uuid.New().String(),
			EntityType: entityType,
			EntityID:   entityID,
			ImageType:  img.Type,
			URL:        img.URL,
			Source:     img.Source,
			Width:      img.Width,
			Height:     img.Height,
		})
	}
	if err := a.imageRepo.Upsert(ctx, stored); err != nil {
		slog.Warn("metadata aggregator: persist images failed", "entity_type", entityType, "entity_id", entityID, "error", err)
	}
}

func (a *Aggregator) sourcesFor(contentType domain.ContentType) []ports.MetadataSource {
	var result []ports.MetadataSource
	for _, src := range a.sources {
		if servesContentType(src, contentType) {
			result = append(result, src)
		}
	}
	return result
}
