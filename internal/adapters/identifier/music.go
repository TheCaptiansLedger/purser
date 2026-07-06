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

	// Agreement bonuses are added to the recording base, scaled by the acoustid score.
	// Total possible: 0.10+0.05+0.03 = 0.18 per unit of acoustidScore.
	bonusTitleAgreement    = 0.10
	bonusArtistAgreement   = 0.05
	bonusDurationAgreement = 0.03

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
				score += 0.10 // album bonus — same weight as bonusTitleAgreement
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

	// Strategy 1: Embedded MusicBrainz Track ID — always wins when present.
	// A precise recording MBID is the strongest possible evidence for recording identity;
	// combined confidence may be low when edition data is unavailable, but recording
	// identity is established. No need to fall through to weaker strategies.
	if mbzID := fp.EmbeddedTags["musicbrainz_track_id"]; mbzID != "" {
		candidates, err := m.mbzTrackIDStrategy(ctx, mbzID, fp)
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			enrichFromTags(candidates, fp)
			return candidates, nil
		}
	}

	// Strategy 2: AcoustID lookup (skipped when no API key or fingerprint absent)
	acoustidCandidates, err := m.acoustidStrategy(ctx, fp, f.Path)
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
	artistID := fp.EmbeddedTags["musicbrainz_album_artist_id"]

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
	var item *domain.Item
	if err == nil {
		item, err = m.items.Get(ctx, entityID)
		if errs.IsNotFound(err) {
			item = nil
		} else if err != nil {
			return nil, fmt.Errorf("get item by mbz id: %w", err)
		}
	} else if !errs.IsNotFound(err) {
		return nil, fmt.Errorf("mbz track id lookup: %w", err)
	}
	ext := m.fetchExternalByMBID(ctx, mbzID, fp.EmbeddedTags["album"])
	c := m.scoreMBZTrackID(ext, mbzID, fp)
	c.Item = item
	return []domain.MatchCandidate{c}, nil
}

// scoreMBZTrackID builds a two-tier scored MatchCandidate for a direct MBZ recording
// MBID lookup. acoustidScore is 1.0 — a precise machine-readable ID is the strongest
// possible evidence for recording identity.
func (m *musicIdentifier) scoreMBZTrackID(ext *domain.ExternalItem, recordingMBID string, fp *domain.Fingerprint) domain.MatchCandidate {
	title, artist, durationSecs := "", "", 0
	if ext != nil {
		title = ext.Title
		durationSecs = ext.RuntimeSecs
		if ext.Studio != nil {
			artist = ext.Studio.Name
		}
	}
	recConf, recReasons := computeRecordingConfidence(baseMBZTrackID, 1.0, title, artist, durationSecs, fp)

	var rd *domain.ExternalReleaseDetail
	if ext != nil {
		rd = ext.ReleaseDetail
	}
	releaseTitle, releaseDate := "", ""
	if rd != nil {
		releaseTitle = rd.ReleaseTitle
		releaseDate = rd.ReleaseDate
	}
	relConf, relReasons := computeReleaseConfidence(fp.EmbeddedTags["album"], embeddedYearFromTags(fp.EmbeddedTags), releaseTitle, releaseDate)
	comb := combinedConfidence(recConf, relConf)

	reasons := domain.MatchReasons{
		Fingerprint: recReasons.Fingerprint,
		Duration:    recReasons.Duration,
		TitleTag:    recReasons.TitleTag,
		ArtistTag:   recReasons.ArtistTag,
		AlbumTag:    relReasons.AlbumTag,
	}

	rgMBID := ""
	if ext != nil {
		rgMBID = ext.GroupExternalID
	}
	detail := &domain.MusicMatchDetail{
		RecordingMBID:       recordingMBID,
		RecordingTitle:      title,
		RecordingConfidence: recConf,
		ReleaseGroupMBID:    rgMBID,
		ReleaseConfidence:   relConf,
		MatchReasons:        reasons,
	}
	if rd != nil {
		detail.ReleaseMBID = rd.ReleaseMBID
		detail.ReleaseTitle = rd.ReleaseTitle
		detail.ReleaseDate = rd.ReleaseDate
		detail.ReleaseLabel = rd.ReleaseLabel
		detail.ReleaseCountry = rd.ReleaseCountry
		detail.ReleaseCatalog = rd.ReleaseCatalog
		detail.ReleaseBarcode = rd.ReleaseBarcode
	}

	return domain.MatchCandidate{
		ExternalItem:        ext,
		Confidence:          comb,
		RecordingConfidence: recConf,
		ReleaseConfidence:   relConf,
		Source:              "musicbrainz_track_id",
		MusicDetail:         detail,
	}
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

// allReleasesLookup is the narrow interface for fetching every release of a recording.
// Implemented by the MBZ adapter; used by acoustidStrategy to generate one candidate
// per unique release group rather than one per recording MBID.
type allReleasesLookup interface {
	FetchAllRecordingReleases(ctx context.Context, mbid string) ([]*domain.ExternalItem, error)
}

// fetchAllReleasesByMBID returns all releases for a recording as separate ExternalItems.
// Falls back to a single-item slice from the existing single-release path when no source
// implements allReleasesLookup (e.g. in tests with minimal stubs).
func (m *musicIdentifier) fetchAllReleasesByMBID(ctx context.Context, mbid string) []*domain.ExternalItem {
	for _, src := range m.sources {
		if !slices.Contains(src.ContentTypes(), domain.ContentTypeMusic) {
			continue
		}
		if arl, ok := src.(allReleasesLookup); ok {
			items, err := arl.FetchAllRecordingReleases(ctx, mbid)
			if err == nil && len(items) > 0 {
				return items
			}
		}
	}
	if ext := m.fetchExternalByMBID(ctx, mbid, ""); ext != nil {
		return []*domain.ExternalItem{ext}
	}
	return nil
}

// embeddedYearFromTags parses a four-digit year from the "year" or "date" embedded tag.
// Returns 0 when the tag is absent or not parseable.
func embeddedYearFromTags(tags map[string]string) int {
	for _, key := range []string{"year", "date"} {
		if v := tags[key]; len(v) >= 4 {
			if y, err := strconv.Atoi(v[:4]); err == nil && y > 0 {
				return y
			}
		}
	}
	return 0
}

type groupEntry struct {
	candidate   domain.MatchCandidate
	releaseMBID string
}

func (m *musicIdentifier) acoustidStrategy(ctx context.Context, fp *domain.Fingerprint, filePath string) ([]domain.MatchCandidate, error) {
	if m.acoustid == nil || fp.AcoustID == "" {
		return nil, nil //nolint:nilnil
	}
	durMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	matches, err := m.acoustid.Lookup(ctx, fp.AcoustID, durMS/1000)
	if err != nil {
		slog.WarnContext(ctx, "music: acoustid lookup failed",
			"file", filePath,
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

	embeddedAlbum := fp.EmbeddedTags["album"]
	embeddedYear := embeddedYearFromTags(fp.EmbeddedTags)
	base := tagQualityBase(fp)
	bestPerGroup := make(map[string]groupEntry)

	for _, r := range ranked {
		item, err := m.resolveLibraryItem(ctx, r.mbid)
		if err != nil {
			return nil, err
		}
		for _, ext := range m.fetchAllReleasesByMBID(ctx, r.mbid) {
			scoreAndDedup(ctx, ext, item, fp, r.acoustidScore, base, embeddedAlbum, embeddedYear, filePath, bestPerGroup)
		}
	}

	candidates := make([]domain.MatchCandidate, 0, len(bestPerGroup))
	for _, entry := range bestPerGroup {
		candidates = append(candidates, entry.candidate)
	}
	sortCandidates(candidates)
	if len(candidates) > maxAcoustIDCandidates {
		candidates = candidates[:maxAcoustIDCandidates]
	}
	return candidates, nil
}

// scoreAndDedup scores one release candidate and updates bestPerGroup, keeping the
// highest combined-confidence release per release group.
func scoreAndDedup(
	ctx context.Context,
	ext *domain.ExternalItem,
	item *domain.Item,
	fp *domain.Fingerprint,
	acoustidScore, base float64,
	embeddedAlbum string,
	embeddedYear int,
	filePath string,
	bestPerGroup map[string]groupEntry,
) {
	if ext == nil || ext.GroupExternalID == "" || ext.ReleaseDetail == nil {
		return
	}
	artist := ""
	if ext.Studio != nil {
		artist = ext.Studio.Name
	}
	recConf, recReasons := computeRecordingConfidence(base, acoustidScore, ext.Title, artist, ext.RuntimeSecs, fp)
	relConf, relReasons := computeReleaseConfidence(embeddedAlbum, embeddedYear, ext.ReleaseDetail.ReleaseTitle, ext.ReleaseDetail.ReleaseDate)
	comb := combinedConfidence(recConf, relConf)

	reasons := domain.MatchReasons{
		Fingerprint: recReasons.Fingerprint,
		Duration:    recReasons.Duration,
		TitleTag:    recReasons.TitleTag,
		ArtistTag:   recReasons.ArtistTag,
		AlbumTag:    relReasons.AlbumTag,
	}

	slog.DebugContext(ctx, "music: candidate scored",
		"file", filePath,
		"recording_mbid", ext.ExternalID,
		"recording_title", ext.Title,
		"recording_confidence", recConf,
		"release_group_mbid", ext.GroupExternalID,
		"release_title", ext.ReleaseDetail.ReleaseTitle,
		"release_confidence", relConf,
		"combined_confidence", comb,
		"album_tag_similarity", relReasons.AlbumTag,
		"source", "acoustid",
	)

	detail := &domain.MusicMatchDetail{
		RecordingMBID:       ext.ExternalID,
		RecordingTitle:      ext.Title,
		RecordingConfidence: recConf,
		ReleaseGroupMBID:    ext.GroupExternalID,
		ReleaseMBID:         ext.ReleaseDetail.ReleaseMBID,
		ReleaseTitle:        ext.ReleaseDetail.ReleaseTitle,
		ReleaseDate:         ext.ReleaseDetail.ReleaseDate,
		ReleaseLabel:        ext.ReleaseDetail.ReleaseLabel,
		ReleaseCountry:      ext.ReleaseDetail.ReleaseCountry,
		ReleaseCatalog:      ext.ReleaseDetail.ReleaseCatalog,
		ReleaseBarcode:      ext.ReleaseDetail.ReleaseBarcode,
		ReleaseConfidence:   relConf,
		MatchReasons:        reasons,
	}

	rgMBID := ext.GroupExternalID
	prev, exists := bestPerGroup[rgMBID]
	if !exists || comb > prev.candidate.Confidence {
		logDedup(ctx, filePath, rgMBID, ext.ReleaseDetail.ReleaseMBID, prev.releaseMBID, exists)
		bestPerGroup[rgMBID] = groupEntry{
			candidate: domain.MatchCandidate{
				Item:                item,
				ExternalItem:        ext,
				Confidence:          comb,
				RecordingConfidence: recConf,
				ReleaseConfidence:   relConf,
				Source:              "acoustid",
				MusicDetail:         detail,
			},
			releaseMBID: ext.ReleaseDetail.ReleaseMBID,
		}
	} else {
		logDedup(ctx, filePath, rgMBID, prev.releaseMBID, ext.ReleaseDetail.ReleaseMBID, true)
	}
}

func logDedup(ctx context.Context, filePath, rgMBID, kept, discarded string, displaced bool) {
	if !displaced {
		return
	}
	slog.DebugContext(ctx, "music: candidate deduplicated",
		"file", filePath,
		"release_group_mbid", rgMBID,
		"kept_release", kept,
		"discarded_release", discarded,
	)
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

// recordingLookup is the narrow interface for fetching the best release of a recording.
// albumHint is matched against MBZ release titles to select the best release for
// cover art and grouping; pass empty string when no hint is available.
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
