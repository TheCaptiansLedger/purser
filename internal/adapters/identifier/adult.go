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

var adultContentTypes = []domain.ContentType{
	domain.ContentTypeAdult,
	domain.ContentTypeJAV,
}

type adultIdentifier struct {
	mediaFiles ports.MediaFileRepository
	items      ports.ItemRepository
	sources    []ports.MetadataSource
}

var _ ports.FileIdentifier = (*adultIdentifier)(nil)

// NewAdultIdentifier returns a FileIdentifier for AfterDark and JAV content.
func NewAdultIdentifier(
	mediaFiles ports.MediaFileRepository,
	items ports.ItemRepository,
	sources []ports.MetadataSource,
) ports.FileIdentifier {
	return &adultIdentifier{
		mediaFiles: mediaFiles,
		items:      items,
		sources:    sources,
	}
}

func (a *adultIdentifier) ContentTypes() []domain.ContentType {
	return adultContentTypes
}

func (a *adultIdentifier) Identify(ctx context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	if !slices.Contains(a.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := normalizeFingerprint(f.Fingerprint)

	// Strategy 1: OSHash local lookup
	if fp.OSHash != "" {
		candidates, err := a.oShashStrategy(ctx, f.ContentType, fp.OSHash)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 2: Provider hash lookup
	if fp.OSHash != "" {
		candidates, err := a.providerHashStrategy(ctx, f.ContentType, fp.OSHash)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 3: Filename parser
	return a.filenameStrategy(ctx, f.ContentType, f.Path)
}

func (a *adultIdentifier) oShashStrategy(ctx context.Context, _ domain.ContentType, hash string) ([]domain.MatchCandidate, error) {
	mf, err := a.mediaFiles.GetByOSHash(ctx, hash)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("oshash lookup: %w", err)
	}
	if mf.ItemID == "" {
		return nil, nil //nolint:nilnil
	}
	item, err := a.items.Get(ctx, mf.ItemID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by oshash: %w", err)
	}
	return []domain.MatchCandidate{{Item: item, Confidence: 0.97, Source: "oshash"}}, nil
}

func (a *adultIdentifier) providerHashStrategy(ctx context.Context, ct domain.ContentType, hash string) ([]domain.MatchCandidate, error) {
	for _, src := range a.sources {
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
		items, _, err := a.items.List(ctx, ports.ItemFilter{
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

func (a *adultIdentifier) filenameStrategy(ctx context.Context, ct domain.ContentType, path string) ([]domain.MatchCandidate, error) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	_, title, _, _, fieldCount := ParseAdultFilename(base)
	if title == "" {
		return nil, nil //nolint:nilnil
	}

	confidence := 0.40
	if fieldCount >= 3 {
		confidence = 0.60
	}

	items, _, err := a.items.List(ctx, ports.ItemFilter{
		ContentTypes: []domain.ContentType{ct},
		Search:       title,
		Limit:        10,
	})
	if err != nil {
		return nil, fmt.Errorf("filename search: %w", err)
	}

	candidates := make([]domain.MatchCandidate, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, domain.MatchCandidate{
			Item:       item,
			Confidence: confidence,
			Source:     "filename",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}
