package identifier

import (
	"context"
	"fmt"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strings"
)

var bookContentTypes = []domain.ContentType{domain.ContentTypeBook}

type bookIdentifier struct {
	extIDs  ports.ExternalIDRepository
	items   ports.ItemRepository
	sources []ports.MetadataSource
}

var _ ports.FileIdentifier = (*bookIdentifier)(nil)

// NewBookIdentifier returns a FileIdentifier for book content (EPUB, PDF, etc.).
func NewBookIdentifier(
	extIDs ports.ExternalIDRepository,
	items ports.ItemRepository,
	sources []ports.MetadataSource,
) ports.FileIdentifier {
	return &bookIdentifier{
		extIDs:  extIDs,
		items:   items,
		sources: sources,
	}
}

func (b *bookIdentifier) ContentTypes() []domain.ContentType {
	return bookContentTypes
}

func (b *bookIdentifier) Identify(ctx context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	if !slices.Contains(b.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := normalizeFingerprint(f.Fingerprint)

	// Strategy 1: ISBN local lookup
	if fp.ISBN != "" {
		candidates, err := b.isbnLocalStrategy(ctx, fp.ISBN)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 2: Provider ISBN lookup
	if fp.ISBN != "" {
		candidates, err := b.providerISBNStrategy(ctx, fp.ISBN)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 3: Title + author from embedded tags, or filename fallback
	return b.titleStrategy(ctx, fp, f.Path)
}

func (b *bookIdentifier) isbnLocalStrategy(ctx context.Context, isbn string) ([]domain.MatchCandidate, error) {
	entityID, err := b.extIDs.FindEntity(ctx, "item", "openlibrary", isbn)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("isbn local lookup: %w", err)
	}
	item, err := b.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by isbn: %w", err)
	}
	return []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "isbn_local"}}, nil
}

func (b *bookIdentifier) providerISBNStrategy(ctx context.Context, isbn string) ([]domain.MatchCandidate, error) {
	var candidates []domain.MatchCandidate
	for _, src := range b.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeBook) {
			continue
		}
		es, ok := src.(ports.ExternalIDSource)
		if !ok {
			continue
		}
		ext, err := es.FindByExternalID(ctx, domain.ContentTypeBook, isbn)
		if err != nil || ext == nil || ext.Title == "" {
			continue
		}
		c := domain.MatchCandidate{ExternalItem: ext, Confidence: 0.92, Source: "isbn_provider"}
		items, _, err := b.items.List(ctx, ports.ItemFilter{
			ContentTypes: []domain.ContentType{domain.ContentTypeBook},
			Search:       ext.Title,
			Limit:        1,
		})
		if err == nil && len(items) > 0 {
			c.Item = items[0]
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

func (b *bookIdentifier) titleStrategy(ctx context.Context, fp *domain.Fingerprint, path string) ([]domain.MatchCandidate, error) {
	title := fp.EmbeddedTags["title"]
	creator := fp.EmbeddedTags["creator"]

	if title == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		title = strings.TrimSpace(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(base))
	}
	if title == "" {
		return nil, nil //nolint:nilnil
	}

	items, _, err := b.items.List(ctx, ports.ItemFilter{
		ContentTypes: []domain.ContentType{domain.ContentTypeBook},
		Search:       title,
		Limit:        10,
	})
	if err != nil {
		return nil, fmt.Errorf("title search: %w", err)
	}

	hasCreator := creator != ""
	candidates := make([]domain.MatchCandidate, 0, len(items))
	for _, item := range items {
		confidence := 0.40
		if hasCreator {
			confidence = 0.65
		}
		candidates = append(candidates, domain.MatchCandidate{
			Item:       item,
			Confidence: confidence,
			Source:     "title",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}
