package metadata

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
)

// Aggregator fans out metadata queries to all enabled providers for a content
// type and merges results ordered by provider priority.
// sources[0] is the primary; all source failures are treated equally — logged
// and skipped. An error is returned only when no sources produced a result.
type Aggregator struct {
	sources []ports.MetadataSource
}

// NewAggregator constructs an Aggregator. sources must be ordered by descending
// priority (index 0 = primary).
func NewAggregator(sources []ports.MetadataSource) *Aggregator {
	return &Aggregator{sources: sources}
}

// FindByExternalID fans out to all sources that support contentType, merges
// results ordered by source priority, and returns the merged item.
// All source failures are logged and skipped; an error is returned only if no
// sources returned a result.
func (a *Aggregator) FindByExternalID(ctx context.Context, contentType domain.ContentType, externalID, entityID string) (*domain.ExternalItem, error) {
	matching := a.sourcesFor(contentType)
	if len(matching) == 0 {
		slog.Warn("metadata aggregator: no sources configured", "content_type", contentType, "external_id", externalID)
		return nil, fmt.Errorf("metadata aggregator: no sources configured for %q", contentType)
	}

	sourceNames := make([]string, len(matching))
	for i, src := range matching {
		sourceNames[i] = src.Name()
	}
	slog.Info("metadata aggregator: fan-out", "content_type", contentType, "external_id", externalID, "entity_id", entityID, "sources", sourceNames)

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

	slog.Info("metadata aggregator: merged", "content_type", contentType, "external_id", externalID, "images", len(merged.Images))
	return merged, nil
}

// SearchItems fans out to all sources that support contentType and returns the
// combined results. Errors from individual sources are logged and skipped.
func (a *Aggregator) SearchItems(ctx context.Context, contentType domain.ContentType, title string, limit int) ([]*domain.ExternalItem, error) {
	slog.Info("metadata aggregator: SearchItems", "content_type", contentType, "title", title, "limit", limit)
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

func (a *Aggregator) sourcesFor(contentType domain.ContentType) []ports.MetadataSource {
	var result []ports.MetadataSource
	for _, src := range a.sources {
		if servesContentType(src, contentType) {
			result = append(result, src)
		}
	}
	return result
}
