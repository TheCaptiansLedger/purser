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
	acoustidCandidates, err := m.acoustidStrategy(ctx, fp)
	if err != nil {
		return nil, err
	}
	// Only short-circuit if AcoustID resolved to a library item. When above
	// threshold but item=nil (MBIDs not stored on tracks), fall through so the
	// tag strategy can still auto-match by title.
	if aboveThreshold(acoustidCandidates) && anyItemLinked(acoustidCandidates) {
		return acoustidCandidates, nil
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
	// Prefer acoustid candidates (even without library item) over low-confidence
	// filename guesses so the queue entry carries the MBID for manual review.
	if len(acoustidCandidates) > 0 {
		return acoustidCandidates, nil
	}
	return m.filenameStrategy(ctx, f.Path)
}

func (m *musicIdentifier) mbzTrackIDStrategy(ctx context.Context, mbzID string) ([]domain.MatchCandidate, error) {
	entityID, err := m.extIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), mbzID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID)
		return []domain.MatchCandidate{{ExternalItem: ext, Confidence: 0.99, Source: "musicbrainz_track_id"}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mbz track id lookup: %w", err)
	}
	item, err := m.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID)
		return []domain.MatchCandidate{{ExternalItem: ext, Confidence: 0.99, Source: "musicbrainz_track_id"}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by mbz id: %w", err)
	}
	ext := m.fetchExternalByMBID(ctx, mbzID)
	return []domain.MatchCandidate{{Item: item, ExternalItem: ext, Confidence: 0.99, Source: "musicbrainz_track_id"}}, nil
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
	var candidates []domain.MatchCandidate
	for _, mbid := range mbids {
		c := domain.MatchCandidate{
			ExternalItem: m.fetchExternalByMBID(ctx, mbid),
			Confidence:   0.95,
			Source:       "acoustid",
		}
		entityID, err := m.extIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), mbid)
		if err != nil && !errs.IsNotFound(err) {
			return nil, fmt.Errorf("acoustid extid lookup: %w", err)
		}
		if entityID != "" {
			item, err := m.items.Get(ctx, entityID)
			if err != nil && !errs.IsNotFound(err) {
				return nil, fmt.Errorf("get item by acoustid mbid: %w", err)
			}
			if err == nil {
				c.Item = item
			}
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

func (m *musicIdentifier) tagFuzzyStrategy(ctx context.Context, fp *domain.Fingerprint) ([]domain.MatchCandidate, error) {
	title := normalizeQuotes(fp.EmbeddedTags["title"])
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
	// Unique title match with full tag context (artist + album + title) is more
	// reliable than a title-only search that might span multiple albums.
	if len(candidates) == 1 {
		candidates[0].Confidence = 0.92
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

// fetchExternalByMBID asks each registered source for the ExternalItem for a
// MusicBrainz recording ID. Returns a stub with just the ID set when no source
// can hydrate it, so the queue entry always carries the MBID for the UI.
func (m *musicIdentifier) fetchExternalByMBID(ctx context.Context, mbid string) *domain.ExternalItem {
	for _, src := range m.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeMusic) {
			continue
		}
		es, ok := src.(ports.ExternalIDSource)
		if !ok {
			continue
		}
		ext, err := es.FindByExternalID(ctx, domain.ContentTypeMusic, mbid)
		if err == nil && ext != nil {
			return ext
		}
	}
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  mbid,
		ContentType: domain.ContentTypeMusic,
	}
}

// hasFullMusicTagSet reports whether fp has all three tags needed for fuzzy matching.
func hasFullMusicTagSet(fp *domain.Fingerprint) bool {
	return fp.EmbeddedTags["title"] != "" &&
		fp.EmbeddedTags["artist"] != "" &&
		fp.EmbeddedTags["album"] != ""
}
