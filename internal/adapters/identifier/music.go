package identifier

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strconv"
	"strings"
)

var musicContentTypes = []domain.ContentType{domain.ContentTypeMusic}

type musicIdentifier struct {
	extIDs   ports.ExternalIDRepository
	items    ports.ItemRepository
	sources  []ports.MetadataSource
	acoustid *acoustidClient
}

var _ ports.FileIdentifier = (*musicIdentifier)(nil)

// NewMusicIdentifier returns a FileIdentifier for music content.
// acoustidKey is optional; when empty the AcoustID strategy is skipped for all files.
func NewMusicIdentifier(
	extIDs ports.ExternalIDRepository,
	items ports.ItemRepository,
	sources []ports.MetadataSource,
	acoustidKey string,
) ports.FileIdentifier {
	var ac *acoustidClient
	if acoustidKey != "" {
		ac = newAcoustIDClient(acoustidKey)
	}
	return &musicIdentifier{
		extIDs:   extIDs,
		items:    items,
		sources:  sources,
		acoustid: ac,
	}
}

func (m *musicIdentifier) ContentTypes() []domain.ContentType {
	return musicContentTypes
}

func (m *musicIdentifier) Identify(ctx context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error) {
	if !slices.Contains(m.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := normalizeFingerprint(f.Fingerprint)

	// Strategy 1: Embedded MusicBrainz Track ID
	if mbzID := fp.EmbeddedTags["musicbrainz_track_id"]; mbzID != "" {
		candidates, err := m.mbzTrackIDStrategy(ctx, mbzID)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			return candidates, nil
		}
	}

	// Strategy 2: AcoustID lookup (skipped when no API key or fingerprint absent)
	if candidates, err := m.acoustidStrategy(ctx, fp); err != nil {
		return nil, err
	} else if aboveThreshold(candidates) {
		return candidates, nil
	}

	// Strategy 3: Full tag-set fuzzy match
	if hasFullMusicTagSet(fp) {
		candidates, err := m.tagFuzzyStrategy(ctx, fp)
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
	}

	// Strategy 4: Filename parse
	return m.filenameStrategy(ctx, f.Path)
}

func (m *musicIdentifier) mbzTrackIDStrategy(ctx context.Context, mbzID string) ([]domain.MatchCandidate, error) {
	entityID, err := m.extIDs.FindEntity(ctx, "item", "musicbrainz", mbzID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("mbz track id lookup: %w", err)
	}
	item, err := m.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by mbz id: %w", err)
	}
	return []domain.MatchCandidate{{Item: item, Confidence: 0.99, Source: "musicbrainz_track_id"}}, nil
}

func (m *musicIdentifier) acoustidStrategy(ctx context.Context, fp *domain.Fingerprint) ([]domain.MatchCandidate, error) {
	if m.acoustid == nil || fp.AcoustID == "" {
		return nil, nil //nolint:nilnil
	}
	durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	mbids, err := m.acoustid.Lookup(ctx, fp.AcoustID, durMS/1000)
	if err != nil {
		slog.WarnContext(ctx, "acoustid lookup failed", "err", err)
		return nil, nil //nolint:nilnil,nilerr // non-fatal; fall through to next strategy
	}
	for _, mbid := range mbids {
		entityID, err := m.extIDs.FindEntity(ctx, "item", "musicbrainz", mbid)
		if errs.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("acoustid extid lookup: %w", err)
		}
		item, err := m.items.Get(ctx, entityID)
		if errs.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get item by acoustid mbid: %w", err)
		}
		return []domain.MatchCandidate{{Item: item, Confidence: 0.95, Source: "acoustid"}}, nil
	}
	return nil, nil //nolint:nilnil
}

func (m *musicIdentifier) tagFuzzyStrategy(ctx context.Context, fp *domain.Fingerprint) ([]domain.MatchCandidate, error) {
	title := fp.EmbeddedTags["title"]
	durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	durSecs := durMS / 1000

	items, _, err := m.items.List(ctx, ports.ItemFilter{
		ContentTypes: []domain.ContentType{domain.ContentTypeMusic},
		Search:       title,
		Limit:        10,
	})
	if err != nil {
		return nil, fmt.Errorf("tag fuzzy search: %w", err)
	}

	candidates := make([]domain.MatchCandidate, 0, len(items))
	for _, item := range items {
		if durSecs > 0 && item.RuntimeSeconds > 0 {
			diff := item.RuntimeSeconds - durSecs
			if diff < -5 || diff > 5 {
				continue
			}
		}
		candidates = append(candidates, domain.MatchCandidate{
			Item:       item,
			Confidence: 0.75,
			Source:     "tags",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}

func (m *musicIdentifier) filenameStrategy(ctx context.Context, path string) ([]domain.MatchCandidate, error) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	_, title := ParseTrackFilename(base)
	if title == "" {
		return nil, nil //nolint:nilnil
	}

	items, _, err := m.items.List(ctx, ports.ItemFilter{
		ContentTypes: []domain.ContentType{domain.ContentTypeMusic},
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
			Confidence: 0.40,
			Source:     "filename",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}

// hasFullMusicTagSet reports whether fp has all three tags needed for fuzzy matching.
func hasFullMusicTagSet(fp *domain.Fingerprint) bool {
	return fp.EmbeddedTags["title"] != "" &&
		fp.EmbeddedTags["artist"] != "" &&
		fp.EmbeddedTags["album"] != ""
}
