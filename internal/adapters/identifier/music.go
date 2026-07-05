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
	"sort"
	"strconv"
	"strings"
)

var musicContentTypes = []domain.ContentType{domain.ContentTypeMusic}

const (
	// baseMBZTrackID is the starting confidence when a MusicBrainz recording ID
	// is embedded in the file's tags. A precise machine-readable identifier —
	// stronger than any combination of human-readable tags.
	baseMBZTrackID = 0.95

	// Tag-quality base scores reflect how completely the embedded tags identify
	// the file. Embedded tags are the primary evidence; external lookups verify.
	tagBaseFullWithTrack = 0.75 // title + artist + album + track number
	tagBaseFullSet       = 0.70 // title + artist + album
	tagBaseArtist        = 0.50 // title + artist only
	tagBaseTitle         = 0.35 // title only
	tagBaseNone          = 0.20 // no useful embedded tags

	// Agreement bonuses are added to the base, scaled by the verification score.
	// Total possible: 0.10+0.05+0.10+0.03 = 0.28
	bonusTitleAgreement    = 0.10
	bonusArtistAgreement   = 0.05
	bonusAlbumAgreement    = 0.10
	bonusDurationAgreement = 0.03

	// acoustidNoTagCeiling caps confidence when the only evidence is a fingerprint
	// match and the file has no useful embedded tags. Keeps it below the
	// auto-import threshold so a human reviews before import.
	acoustidNoTagCeiling = 0.60

	// Tag-fuzzy and filename strategy bases.
	baseTagFuzzy1 = 0.70 // single local-library match — confident
	baseTagFuzzyN = 0.60 // multiple local-library matches — ambiguous
	baseFilename  = 0.35

	// AcoustID result filtering.
	minAcoustIDScore      = 0.50 // discard results below this score
	maxAcoustIDCandidates = 4    // cap on candidates emitted by AcoustID strategy
)

// tagQualityBase returns a base confidence score from embedded tag completeness.
// A richer tag set represents stronger evidence of deliberate identification.
func tagQualityBase(fp *domain.Fingerprint) float64 {
	hasTitle := fp.EmbeddedTags["title"] != ""
	hasArtist := fp.EmbeddedTags["artist"] != ""
	hasAlbum := fp.EmbeddedTags["album"] != ""
	hasTrack := fp.EmbeddedTags["tracknumber"] != "" || fp.EmbeddedTags["track"] != ""
	switch {
	case hasTitle && hasArtist && hasAlbum && hasTrack:
		return tagBaseFullWithTrack
	case hasTitle && hasArtist && hasAlbum:
		return tagBaseFullSet
	case hasTitle && hasArtist:
		return tagBaseArtist
	case hasTitle:
		return tagBaseTitle
	default:
		return tagBaseNone
	}
}

// tagAgreementBonus returns the sum of per-field agreement bonuses between an
// external candidate and the embedded tags of the file.
func tagAgreementBonus(item *domain.ExternalItem, fp *domain.Fingerprint) float64 {
	var bonus float64
	if title := fp.EmbeddedTags["title"]; title != "" && strings.EqualFold(item.Title, title) {
		bonus += bonusTitleAgreement
	}
	if artist := fp.EmbeddedTags["artist"]; artist != "" {
		if item.Studio != nil && strings.EqualFold(item.Studio.Name, artist) {
			bonus += bonusArtistAgreement
		}
	}
	if album := fp.EmbeddedTags["album"]; album != "" && strings.EqualFold(item.GroupTitle, album) {
		bonus += bonusAlbumAgreement
	}
	if durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"]); durMS > 0 && item.RuntimeSecs > 0 {
		diff := item.RuntimeSecs - durMS/1000
		if diff >= -5 && diff <= 5 {
			bonus += bonusDurationAgreement
		}
	}
	return bonus
}

// computeConfidence returns the final confidence for a candidate.
//
// base is the starting score for the identification strategy.
// verificationScore scales the tag-agreement bonuses: 1.0 for a direct MBZ
// lookup (the MBID was in the file's tags), or the AcoustID match score for
// fingerprint-based matches.
//
// When there are no useful embedded tags and the candidate is from AcoustID,
// the score is capped at acoustidNoTagCeiling so the file goes to the queue.
func computeConfidence(base, verificationScore float64, c domain.MatchCandidate, fp *domain.Fingerprint) float64 {
	if c.ExternalItem == nil {
		return base
	}

	agreement := tagAgreementBonus(c.ExternalItem, fp)

	// No useful tags: fall back to a scaled ceiling so the file requires review.
	if base <= tagBaseNone && agreement == 0 {
		return verificationScore * acoustidNoTagCeiling
	}

	score := base + agreement*verificationScore
	if score > 1.0 {
		return 1.0
	}
	return score
}

// inlineTagScore scores an AcoustIDRecording against embedded tags using only
// the data returned by AcoustID — no MusicBrainz lookup required. Used to rank
// candidate MBIDs before deciding which ones to fetch from MusicBrainz.
func inlineTagScore(rec AcoustIDRecording, fp *domain.Fingerprint) float64 {
	var score float64
	if title := fp.EmbeddedTags["title"]; title != "" && strings.EqualFold(rec.Title, title) {
		score += bonusTitleAgreement
	}
	if artist := fp.EmbeddedTags["artist"]; artist != "" && strings.EqualFold(rec.Artist, artist) {
		score += bonusArtistAgreement
	}
	if album := fp.EmbeddedTags["album"]; album != "" {
		for _, rel := range rec.Albums {
			if strings.EqualFold(rel, album) {
				score += bonusAlbumAgreement
				break
			}
		}
	}
	if durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"]); durMS > 0 && rec.Duration > 0 {
		diff := rec.Duration - durMS/1000
		if diff >= -5 && diff <= 5 {
			score += bonusDurationAgreement
		}
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
	// threshold but item=nil, fall through so the tag strategy can auto-match.
	if aboveThreshold(acoustidCandidates) && anyItemLinked(acoustidCandidates) {
		enrichFromTags(acoustidCandidates, fp)
		return acoustidCandidates, nil
	}

	// Strategy 3: Full tag-set fuzzy match against local library
	if hasFullMusicTagSet(fp) {
		candidates, err := m.tagFuzzyStrategy(ctx, fp)
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
	}

	// Strategy 4: Prefer AcoustID candidates (even without library item) over
	// low-confidence filename guesses so the queue entry carries MBIDs.
	if len(acoustidCandidates) > 0 {
		enrichFromTags(acoustidCandidates, fp)
		return acoustidCandidates, nil
	}
	return m.filenameStrategy(ctx, f.Path)
}

// enrichFromTags fills in missing metadata on candidates whose ExternalItem was
// built from a MBID stub. Embedded tags are the fallback source for title,
// runtime, and artist info. Studio is only populated when a
// musicbrainz_artist_id tag is present so that the Import & Create dialog can
// call importEntry with a valid external ID.
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
	albumHint := fp.EmbeddedTags["album"]
	entityID, err := m.extIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), mbzID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID, albumHint)
		c := domain.MatchCandidate{ExternalItem: ext, Source: "musicbrainz_track_id"}
		c.Confidence = computeConfidence(baseMBZTrackID, 1.0, c, fp)
		return []domain.MatchCandidate{c}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mbz track id lookup: %w", err)
	}
	item, err := m.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		ext := m.fetchExternalByMBID(ctx, mbzID, albumHint)
		c := domain.MatchCandidate{ExternalItem: ext, Source: "musicbrainz_track_id"}
		c.Confidence = computeConfidence(baseMBZTrackID, 1.0, c, fp)
		return []domain.MatchCandidate{c}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by mbz id: %w", err)
	}
	ext := m.fetchExternalByMBID(ctx, mbzID, albumHint)
	c := domain.MatchCandidate{Item: item, ExternalItem: ext, Source: "musicbrainz_track_id"}
	c.Confidence = computeConfidence(baseMBZTrackID, 1.0, c, fp)
	return []domain.MatchCandidate{c}, nil
}

type rankedMBID struct {
	mbid          string
	acoustidScore float64
}

// rankAcoustIDMBIDs deduplicates MBIDs across AcoustID results, filters
// low-quality matches, and returns up to maxAcoustIDCandidates entries ranked
// by combined fingerprint + tag agreement score.
func rankAcoustIDMBIDs(matches []AcoustIDMatch, fp *domain.Fingerprint) []rankedMBID {
	type scored struct {
		acoustidScore float64
		tagScore      float64
	}
	best := make(map[string]scored)
	for _, match := range matches {
		if match.Score < minAcoustIDScore {
			continue
		}
		for _, rec := range match.Recordings {
			if rec.MBID == "" {
				continue
			}
			ts := inlineTagScore(rec, fp)
			if prev, ok := best[rec.MBID]; !ok || match.Score > prev.acoustidScore {
				best[rec.MBID] = scored{acoustidScore: match.Score, tagScore: ts}
			}
		}
	}
	type combined struct {
		rankedMBID
		combined float64
	}
	all := make([]combined, 0, len(best))
	for mbid, s := range best {
		all = append(all, combined{
			rankedMBID: rankedMBID{mbid: mbid, acoustidScore: s.acoustidScore},
			combined:   s.acoustidScore * (1 + s.tagScore),
		})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].combined > all[j].combined })
	if len(all) > maxAcoustIDCandidates {
		all = all[:maxAcoustIDCandidates]
	}
	out := make([]rankedMBID, len(all))
	for i, a := range all {
		out[i] = a.rankedMBID
	}
	return out
}

// resolveLibraryItem looks up a library item linked to the given MusicBrainz
// recording ID. Returns nil (not an error) when no linked item is found.
func (m *musicIdentifier) resolveLibraryItem(ctx context.Context, mbid string) (*domain.Item, error) {
	entityID, err := m.extIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), mbid)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("acoustid extid lookup: %w", err)
	}
	item, err := m.items.Get(ctx, entityID)
	if errs.IsNotFound(err) {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("get item by acoustid mbid: %w", err)
	}
	return item, nil
}

func (m *musicIdentifier) acoustidStrategy(ctx context.Context, fp *domain.Fingerprint) ([]domain.MatchCandidate, error) {
	if m.acoustid == nil || fp.AcoustID == "" {
		return nil, nil //nolint:nilnil
	}
	durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	matches, err := m.acoustid.Lookup(ctx, fp.AcoustID, durMS/1000)
	if err != nil {
		slog.WarnContext(ctx, "acoustid lookup failed",
			"title", fp.EmbeddedTags["title"],
			"artist", fp.EmbeddedTags["artist"],
			"err", err)
		return nil, nil //nolint:nilnil,nilerr // non-fatal; fall through to next strategy
	}

	ranked := rankAcoustIDMBIDs(matches, fp)
	if len(ranked) == 0 {
		slog.DebugContext(ctx, "acoustid: no usable recordings after ranking",
			"title", fp.EmbeddedTags["title"],
			"matches", len(matches))
		return nil, nil //nolint:nilnil
	}

	albumHint := fp.EmbeddedTags["album"]
	base := tagQualityBase(fp)
	candidates := make([]domain.MatchCandidate, 0, len(ranked))
	for _, r := range ranked {
		c := domain.MatchCandidate{
			ExternalItem: m.fetchExternalByMBID(ctx, r.mbid, albumHint),
			Source:       "acoustid",
		}
		item, err := m.resolveLibraryItem(ctx, r.mbid)
		if err != nil {
			return nil, err
		}
		c.Item = item
		c.Confidence = computeConfidence(base, r.acoustidScore, c, fp)
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
		candidates[i].Confidence = base
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
// albumHint is matched against MBZ release titles to select the best release
// for cover art and grouping; pass empty string when no hint is available.
type recordingLookup interface {
	FetchRecordingByID(ctx context.Context, mbid, albumHint string) (*domain.ExternalItem, error)
}

// fetchExternalByMBID fetches full recording metadata for a MusicBrainz recording
// ID. Returns a minimal stub if no source can hydrate it.
func (m *musicIdentifier) fetchExternalByMBID(ctx context.Context, mbid, albumHint string) *domain.ExternalItem {
	for _, src := range m.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeMusic) {
			continue
		}
		if rl, ok := src.(recordingLookup); ok {
			if ext, err := rl.FetchRecordingByID(ctx, mbid, albumHint); err == nil && ext != nil {
				return ext
			}
		}
	}
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
