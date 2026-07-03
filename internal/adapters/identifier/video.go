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

var videoContentTypes = []domain.ContentType{
	domain.ContentTypeMovie,
	domain.ContentTypeTV,
}

type videoIdentifier struct {
	mediaFiles ports.MediaFileRepository
	items      ports.ItemRepository
	sources    []ports.MetadataSource
}

var _ ports.FileIdentifier = (*videoIdentifier)(nil)

// NewVideoIdentifier returns a FileIdentifier for movie and TV content.
func NewVideoIdentifier(
	mediaFiles ports.MediaFileRepository,
	items ports.ItemRepository,
	sources []ports.MetadataSource,
) ports.FileIdentifier {
	return &videoIdentifier{
		mediaFiles: mediaFiles,
		items:      items,
		sources:    sources,
	}
}

func (v *videoIdentifier) ContentTypes() []domain.ContentType {
	return videoContentTypes
}

func (v *videoIdentifier) Identify(ctx context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	if !slices.Contains(v.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := normalizeFingerprint(f.Fingerprint)

	// Strategy 1: OSHash local lookup
	if fp.OSHash != "" {
		candidates, err := v.oshashStrategy(ctx, f.ContentType, fp.OSHash)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 2: Provider hash lookup
	if fp.OSHash != "" {
		candidates, err := v.providerHashStrategy(ctx, f.ContentType, fp.OSHash)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 3: Filename parse
	return v.filenameStrategy(ctx, f.ContentType, f.Path)
}

func (v *videoIdentifier) oshashStrategy(ctx context.Context, _ domain.ContentType, hash string) ([]domain.MatchCandidate, error) {
	mf, err := v.mediaFiles.GetByOSHash(ctx, hash)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("oshash lookup: %w", err)
	}
	if mf.ItemID == "" {
		return nil, nil //nolint:nilnil
	}
	item, err := v.items.Get(ctx, mf.ItemID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by oshash: %w", err)
	}
	return []domain.MatchCandidate{{Item: item, Confidence: 0.97, Source: "oshash"}}, nil
}

func (v *videoIdentifier) providerHashStrategy(ctx context.Context, ct domain.ContentType, hash string) ([]domain.MatchCandidate, error) {
	for _, src := range v.sources {
		if !slices.Contains(src.ContentTypes(), ct) {
			continue
		}
		hs, ok := src.(ports.HashLookupSource)
		if !ok {
			continue
		}
		ext, err := hs.FindByHash(ctx, hash)
		if err != nil || ext == nil {
			continue
		}
		if ext.Title == "" {
			continue
		}
		items, _, err := v.items.List(ctx, ports.ItemFilter{
			ContentTypes: []domain.ContentType{ct},
			Search:       ext.Title,
			Limit:        1,
		})
		if err != nil || len(items) == 0 {
			continue
		}
		return []domain.MatchCandidate{{Item: items[0], Confidence: 0.97, Source: "provider_hash"}}, nil
	}
	return nil, nil //nolint:nilnil
}

func (v *videoIdentifier) filenameStrategy(ctx context.Context, ct domain.ContentType, path string) ([]domain.MatchCandidate, error) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	title, year, _, _ := ParseVideoFilename(base)
	if title == "" {
		return nil, nil //nolint:nilnil
	}

	items, _, err := v.items.List(ctx, ports.ItemFilter{
		ContentTypes: []domain.ContentType{ct},
		Search:       title,
		Limit:        10,
	})
	if err != nil {
		return nil, fmt.Errorf("filename search: %w", err)
	}

	candidates := make([]domain.MatchCandidate, 0, len(items))
	for _, item := range items {
		confidence := 0.45
		if year > 0 && item.Date.Year() == year {
			confidence = 0.65
		}
		candidates = append(candidates, domain.MatchCandidate{
			Item:       item,
			Confidence: confidence,
			Source:     "filename",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}
