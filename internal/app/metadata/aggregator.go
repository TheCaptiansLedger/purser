package metadata

import (
	"context"
	"errors"
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

// SearchStudios fans out to all sources that support contentType, combines
// the results, and deduplicates by ExternalID. When two sources return the
// same MBID, the TheAudioDB result is preferred over any other source.
// Errors from individual sources are logged and skipped.
func (a *Aggregator) SearchStudios(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalStudio, error) {
	matching := a.sourcesFor(contentType)

	type result struct{ studios []*domain.ExternalStudio }
	ch := make(chan result, len(matching))
	var wg sync.WaitGroup
	for _, src := range matching {
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			studios, err := src.SearchStudios(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata aggregator: SearchStudios failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{studios}
		}(src)
	}
	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalStudio
	for r := range ch {
		all = append(all, r.studios...)
	}
	return a.deduplicateStudios(all), nil
}

// deduplicateStudios removes studios that share an ExternalID.
//
// When two sources return the same MBID, the lower-ImagePriority source is kept
// as canonical (it typically supports richer data fetching, e.g. MBZ for album
// lists) and the best available image URL is filled in from the highest-priority
// source that has one. Studios with an empty ExternalID are never deduplicated.
// Insertion order is stable.
func (a *Aggregator) deduplicateStudios(studios []*domain.ExternalStudio) []*domain.ExternalStudio {
	priority := make(map[string]int, len(a.sources))
	for _, src := range a.sources {
		priority[src.Name()] = src.ImagePriority()
	}

	// First pass: record the best-priority image URL for each MBID.
	type bestEntry struct {
		url string
		pri int
	}
	bestImages := make(map[string]bestEntry)
	for _, s := range studios {
		if s.ExternalID == "" || s.ImageURL == "" {
			continue
		}
		p := priority[string(s.Source)]
		if e, ok := bestImages[s.ExternalID]; !ok || p > e.pri {
			bestImages[s.ExternalID] = bestEntry{s.ImageURL, p}
		}
	}

	seen := make(map[string]int) // ExternalID → index in out
	out := make([]*domain.ExternalStudio, 0, len(studios))
	for _, s := range studios {
		if s.ExternalID == "" {
			out = append(out, s)
			continue
		}
		idx, ok := seen[s.ExternalID]
		if !ok {
			entry := *s
			if entry.ImageURL == "" {
				entry.ImageURL = bestImages[entry.ExternalID].url
			}
			seen[s.ExternalID] = len(out)
			out = append(out, &entry)
			continue
		}
		// Duplicate MBID: prefer the lower-priority source as canonical so that
		// sources with richer data-fetching support (e.g. MBZ) win the slot.
		existingPri := priority[string(out[idx].Source)]
		incomingPri := priority[string(s.Source)]
		if incomingPri < existingPri {
			entry := *s
			entry.ImageURL = bestImages[entry.ExternalID].url
			if entry.ImageURL == "" {
				entry.ImageURL = out[idx].ImageURL
			}
			out[idx] = &entry
		}
	}
	return out
}

// SearchPeople fans out to all sources that support contentType and returns
// the combined people results. Errors from individual sources are logged and skipped.
func (a *Aggregator) SearchPeople(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalPerson, error) {
	matching := a.sourcesFor(contentType)

	type result struct{ people []*domain.ExternalPerson }
	ch := make(chan result, len(matching))
	var wg sync.WaitGroup
	for _, src := range matching {
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			people, err := src.SearchPeople(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata aggregator: SearchPeople failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{people}
		}(src)
	}
	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalPerson
	for r := range ch {
		all = append(all, r.people...)
	}
	return all, nil
}

// FetchGroupImages fans out FindGroupImages to all sources supporting contentType,
// pairing IDs by source name (with first-available fallback for sources that share
// an ID namespace with another source, e.g. fanart using MBZ MBIDs). Results are
// deduplicated by URL. ErrNotSupported and ErrNotFound are silently skipped.
func (a *Aggregator) FetchGroupImages(ctx context.Context, contentType domain.ContentType, entryExtIDs, groupExtIDs []domain.ExternalID) []domain.ExternalImage {
	seen := make(map[string]bool)
	var all []domain.ExternalImage

	for _, src := range a.sourcesFor(contentType) {
		parentExtID := pickExtID(src.Name(), entryExtIDs)
		groupExtID := pickExtID(src.Name(), groupExtIDs)
		if parentExtID == "" || groupExtID == "" {
			continue
		}

		item, err := src.FindGroupImages(ctx, contentType, parentExtID, groupExtID)
		if err != nil {
			if !errors.Is(err, ports.ErrNotSupported) && !errors.Is(err, ports.ErrNotFound) {
				slog.Warn("metadata aggregator: FindGroupImages failed", "source", src.Name(), "error", err)
			}
			continue
		}
		for _, img := range item.Images {
			if !seen[img.URL] {
				seen[img.URL] = true
				all = append(all, img)
			}
		}
	}
	return all
}

// pickExtID returns the external ID value for the given source name.
// Prefers an exact source name match; falls back to the first available value
// so that adapters sharing an ID namespace with another source (e.g. fanart
// using MBZ MBIDs) can be called even when no source-native ExternalID is stored.
func pickExtID(sourceName string, extIDs []domain.ExternalID) string {
	for _, extID := range extIDs {
		if string(extID.Source) == sourceName {
			return extID.Value
		}
	}
	if len(extIDs) > 0 {
		return extIDs[0].Value
	}
	return ""
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
