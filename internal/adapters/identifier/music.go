package identifier

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/cache"
	"slices"
	"strconv"
	"strings"
)

var musicContentTypes = []domain.ContentType{domain.ContentTypeMusic}

const (
	baseMBZTrackID = 0.95
	baseAcoustID   = 0.80
	baseTagFuzzy1  = 0.70
	baseTagFuzzyN  = 0.60
	baseFilename   = 0.35
	bonusTitle     = 0.05
	bonusArtist    = 0.05
	bonusAlbum     = 0.05
	bonusDuration  = 0.05
)

func computeConfidence(base float64, c domain.MatchCandidate, fp *domain.Fingerprint) float64 {
	score := base
	if c.ExternalItem != nil {
		title := fp.EmbeddedTags["title"]
		if title != "" && strings.EqualFold(c.ExternalItem.Title, title) {
			score += bonusTitle
		}
		artist := fp.EmbeddedTags["artist"]
		if artist != "" && c.ExternalItem.Studio != nil && strings.EqualFold(c.ExternalItem.Studio.Name, artist) {
			score += bonusArtist
		}
		album := fp.EmbeddedTags["album"]
		if album != "" && strings.EqualFold(c.ExternalItem.GroupTitle, album) {
			score += bonusAlbum
		}
		durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
		durSecs := durMS / 1000
		if durSecs > 0 && c.ExternalItem.RuntimeSecs > 0 {
			diff := c.ExternalItem.RuntimeSecs - durSecs
			if diff >= -5 && diff <= 5 {
				score += bonusDuration
			}
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

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
	acoustidCache *cache.Cache,
) ports.FileIdentifier {
	var ac *acoustidClient
	if acoustidKey != "" {
		ac = newAcoustIDClient(acoustidKey, acoustidCache)
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
		candidates, err := m.mbzTrackIDStrategy(ctx, mbzID, fp)
		if err != nil {
			return nil, err
		}
		if aboveThreshold(candidates) {
			enrichFromTags(candidates, fp)
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
		enrichFromTags(acoustidCandidates, fp)
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
		enrichFromTags(acoustidCandidates, fp)
		return acoustidCandidates, nil
	}
	return m.filenameStrategy(ctx, f.Path)
}

// enrichFromTags fills in missing metadata on candidates whose ExternalItem was
// built from a MBID stub (recording endpoint not available in MBZ adapter).
// Embedded tags are the fallback source for title, runtime, and artist info.
// Studio is only populated when a musicbrainz_artist_id tag is present so that
// the Import & Create dialog can call importEntry with a valid external ID.
func enrichFromTags(candidates []domain.MatchCandidate, fp *domain.Fingerprint) {
	durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	artist := fp.EmbeddedTags["artist"]
	artistID := fp.EmbeddedTags["musicbrainz_artist_id"]

	for i := range candidates {
		ext := candidates[i].ExternalItem
		if ext == nil {
			continue
		}
		if ext.Title == "" {
			ext.Title = fp.EmbeddedTags["title"]
		}
		if ext.RuntimeSecs == 0 && durMS > 0 {
			ext.RuntimeSecs = durMS / 1000
		}
		if ext.GroupTitle == "" {
			ext.GroupTitle = fp.EmbeddedTags["album"]
		}
		if ext.Studio == nil && artist != "" && artistID != "" {
			ext.Studio = &domain.ExternalStudio{
				Name:       artist,
				Source:     domain.SourceMusicBrainz,
				ExternalID: artistID,
			}
		}
	}
}

func (m *musicIdentifier) mbzTrackIDStrategy(ctx context.Context, mbzID string, fp *domain.Fingerprint) ([]domain.MatchCandidate, error) {
	entityID, err := m.extIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), mbzID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID)
		c := domain.MatchCandidate{ExternalItem: ext, Source: "musicbrainz_track_id"}
		c.Confidence = computeConfidence(baseMBZTrackID, c, fp)
		return []domain.MatchCandidate{c}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mbz track id lookup: %w", err)
	}
	item, err := m.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID)
		c := domain.MatchCandidate{ExternalItem: ext, Source: "musicbrainz_track_id"}
		c.Confidence = computeConfidence(baseMBZTrackID, c, fp)
		return []domain.MatchCandidate{c}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by mbz id: %w", err)
	}
	ext := m.fetchExternalByMBID(ctx, mbzID)
	c := domain.MatchCandidate{Item: item, ExternalItem: ext, Source: "musicbrainz_track_id"}
	c.Confidence = computeConfidence(baseMBZTrackID, c, fp)
	return []domain.MatchCandidate{c}, nil
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
		c.Confidence = computeConfidence(baseAcoustID, c, fp)
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
			Item:   item,
			Source: "tags",
		})
	}
	// Unique title match with full tag context (artist + album + title) is more
	// reliable than a title-only search that might span multiple albums.
	base := baseTagFuzzyN
	if len(candidates) == 1 {
		base = baseTagFuzzy1
	}
	for i := range candidates {
		candidates[i].Confidence = computeConfidence(base, candidates[i], fp)
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
			Confidence: baseFilename,
			Source:     "filename",
		})
	}
	sortCandidates(candidates)
	return candidates, nil
}

// recordingLookup is the narrow interface for fetching a recording by MBID.
// The MBZ adapter implements this; other sources need not.
type recordingLookup interface {
	FetchRecordingByID(ctx context.Context, mbid string) (*domain.ExternalItem, error)
}

// fetchExternalByMBID fetches full recording metadata for a MusicBrainz recording
// ID. It prefers the dedicated recording endpoint (title + artist credits) over
// the generic FindByExternalID, which does an artist lookup for MBZ and always
// 404s on recording MBIDs. Returns a minimal stub if no source can hydrate it.
func (m *musicIdentifier) fetchExternalByMBID(ctx context.Context, mbid string) *domain.ExternalItem {
	for _, src := range m.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeMusic) {
			continue
		}
		if rl, ok := src.(recordingLookup); ok {
			if ext, err := rl.FetchRecordingByID(ctx, mbid); err == nil && ext != nil {
				return ext
			}
		}
	}
	// Fallback for non-MBZ sources that implement FindByExternalID.
	for _, src := range m.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeMusic) {
			continue
		}
		if es, ok := src.(ports.ExternalIDSource); ok {
			if ext, err := es.FindByExternalID(ctx, domain.ContentTypeMusic, mbid); err == nil && ext != nil {
				return ext
			}
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
