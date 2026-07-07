package identifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Signal weights combine the seven identification signals into an overall
// confidence. Each weight is the maximum contribution of a signal at full
// strength; the stored signal score (0.0–1.0) scales it. The weights mirror the
// table in docs/technical/music_organization.md §Step 7. A barcode hit alone is
// sufficient to auto-import, so it short-circuits the weighted sum.
const (
	weightBarcode       = 1.00
	weightISRC          = 0.95
	weightRGNameFuzzy   = 0.60
	weightTrackCount    = 0.20
	weightTrackTitleSet = 0.25
	weightDuration      = 0.30
	weightAcoustID      = 0.35
)

// maxFuzzyReleaseGroups caps how many name-matched Release Groups from an
// artist's discography are expanded into candidates, bounding MBZ calls when a
// prolific artist has many similarly named albums.
const maxFuzzyReleaseGroups = 5

// musicAlbumIdentifier assembles the full album-level identification pipeline.
// It consumes the tag summary for a scanned folder, resolves candidate Release
// Groups through the barcode, ISRC, and fuzzy-name cascade, scores every signal,
// and either triggers an auto-import (confidence ≥ threshold) or persists the
// ranked candidates to the queue for manual resolution.
//
// It runs after MusicGroupQueueWriter, updating the pending entry that writer
// seeded rather than creating a duplicate.
type musicAlbumIdentifier struct {
	queue     ports.MusicScanGroupRepository
	releases  ports.MusicReleaseRepository
	sources   []ports.MetadataSource
	importer  ports.AlbumImporter
	threshold float64
}

var _ ports.GroupIdentifier = (*musicAlbumIdentifier)(nil)

// NewMusicAlbumIdentifier returns the album-level GroupIdentifier. Wire it after
// NewMusicGroupQueueWriter so the pending queue entry exists before this
// identifier updates it with candidates or flips it to matched on auto-import.
func NewMusicAlbumIdentifier(
	queue ports.MusicScanGroupRepository,
	releases ports.MusicReleaseRepository,
	sources []ports.MetadataSource,
	importer ports.AlbumImporter,
	threshold float64,
) ports.GroupIdentifier {
	return &musicAlbumIdentifier{
		queue:     queue,
		releases:  releases,
		sources:   sources,
		importer:  importer,
		threshold: threshold,
	}
}

// ContentTypes declares that this identifier handles music files only.
func (a *musicAlbumIdentifier) ContentTypes() []domain.ContentType {
	return musicContentTypes
}

// Identify runs the full cascade for one scanned folder.
func (a *musicAlbumIdentifier) Identify(ctx context.Context, group ports.ScannedFileGroup) error {
	root := filepath.Clean(group.RootPath)
	tags := ExtractMusicTagSummary(ctx, group)

	slog.InfoContext(ctx, "album scan start",
		"root", root,
		"tracks", len(group.Files),
		"has_barcode", tags.Barcode != "",
		"has_isrcs", tagsHaveISRCs(tags),
		"has_mbz_id", tags.MBZReleaseID != "",
	)

	if a.reScanShortcut(ctx, root, tags) {
		return nil
	}

	candidates := a.assembleCandidates(ctx, root, tags)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].OverallConfidence > candidates[j].OverallConfidence
	})

	var top float64
	if len(candidates) > 0 {
		top = candidates[0].OverallConfidence
	}

	action := "queue"
	if top >= a.threshold && len(candidates) > 0 {
		action = "auto_import"
	}
	slog.InfoContext(ctx, "album confidence",
		"root", root, "overall", top, "threshold", a.threshold, "action", action)

	if action == "auto_import" {
		return a.autoImport(ctx, root, group, candidates[0])
	}
	return a.saveToQueue(ctx, root, group, tags, candidates)
}

// reScanShortcut returns true when the folder was already imported: a MBZ
// Release ID is present in the tags and the matching release is imported. This
// skips the whole identification pipeline on re-scans after renames or moves.
func (a *musicAlbumIdentifier) reScanShortcut(ctx context.Context, root string, tags domain.MusicTagSummary) bool {
	if tags.MBZReleaseID == "" {
		return false
	}
	rel, err := a.releases.GetByMBID(ctx, tags.MBZReleaseID)
	if err != nil {
		if !errs.IsNotFound(err) {
			slog.WarnContext(ctx, "re-scan release lookup failed", "root", root, "err", err)
		}
		return false
	}
	if rel == nil || rel.Status != domain.ReleaseStatusImported {
		return false
	}
	slog.InfoContext(ctx, "album re-scan shortcut", "root", root, "release_id", rel.ID)
	return true
}

// autoImport calls the release importer for the winning candidate and flips the
// pending queue entry to matched so it leaves the pending list.
func (a *musicAlbumIdentifier) autoImport(ctx context.Context, root string, group ports.ScannedFileGroup, top domain.MusicReleaseCandidate) error {
	slog.InfoContext(ctx, "album auto-importing",
		"root", root, "release_mbid", top.ReleaseMBID, "confidence", top.OverallConfidence)
	if err := a.importer.ImportRelease(ctx, top, group); err != nil {
		return fmt.Errorf("import release: %w", err)
	}
	entry := a.entryFor(ctx, root, group)
	entry.Candidates = []domain.MusicReleaseCandidate{top}
	entry.Status = domain.UnmatchedMatched
	if err := a.queue.Save(ctx, entry); err != nil {
		return fmt.Errorf("mark music scan group matched: %w", err)
	}
	return nil
}

// saveToQueue persists the ranked candidates and signal breakdown onto the
// pending queue entry so the user can resolve the folder manually.
func (a *musicAlbumIdentifier) saveToQueue(ctx context.Context, root string, group ports.ScannedFileGroup, tags domain.MusicTagSummary, candidates []domain.MusicReleaseCandidate) error {
	entry := a.entryFor(ctx, root, group)
	entry.Tags = tags
	entry.Candidates = candidates
	entry.Status = domain.UnmatchedPending
	if err := a.queue.Save(ctx, entry); err != nil {
		return fmt.Errorf("save music scan group candidates: %w", err)
	}

	var topTitle string
	var topScore float64
	if len(candidates) > 0 {
		topTitle = candidates[0].ReleaseTitle
		if topTitle == "" {
			topTitle = candidates[0].ReleaseGroupTitle
		}
		topScore = candidates[0].OverallConfidence
	}
	slog.InfoContext(ctx, "album queued",
		"root", root, "queue_id", entry.ID, "top_candidate", topTitle, "top_score", topScore)
	return nil
}

// entryFor returns the pending queue entry for the folder (seeded by
// MusicGroupQueueWriter), or a freshly built entry when none exists — e.g. when
// the writer is not registered ahead of this identifier.
func (a *musicAlbumIdentifier) entryFor(ctx context.Context, root string, group ports.ScannedFileGroup) *domain.MusicScanGroup {
	pending, err := a.queue.List(ctx, domain.UnmatchedPending)
	if err != nil {
		slog.WarnContext(ctx, "list pending music scan groups", "root", root, "err", err)
	}
	for _, g := range pending {
		if filepath.Clean(g.FolderPath) == root {
			return g
		}
	}
	return &domain.MusicScanGroup{
		ID:           uuid.New().String(),
		FolderPath:   root,
		Files:        group.Files,
		TotalTracks:  len(group.Files),
		TotalDiscs:   countDiscs(group),
		Status:       domain.UnmatchedPending,
		DiscoveredAt: time.Now().UTC(),
	}
}

// releaseGroupRef is a Release Group resolved by the ISRC or fuzzy-name stage,
// carrying the file ISRCs that resolved to it so the ISRC signal can be scored.
type releaseGroupRef struct {
	mbid         string
	title        string
	artistMBID   string
	artistName   string
	matchedISRCs []string
}

// assembleCandidates runs the identification cascade and returns scored
// candidates. A barcode hit resolves the release directly and short-circuits the
// remaining stages; otherwise ISRC consensus and fuzzy artist search resolve
// candidate Release Groups whose editions become candidates.
func (a *musicAlbumIdentifier) assembleCandidates(ctx context.Context, root string, tags domain.MusicTagSummary) []domain.MusicReleaseCandidate {
	if tags.Barcode != "" {
		if src := barcodeSource(a.sources); src != nil {
			rel, err := src.GetReleaseByBarcode(ctx, tags.Barcode)
			switch {
			case err == nil && rel != nil:
				c := candidateFromRelease(rel, rgTitleFromTags(tags), "", tags.AlbumArtist, "")
				a.scoreCandidate(ctx, root, tags, &c, nil)
				// A barcode lookup hit is authoritative: the tag barcode resolved
				// to this release. Force the signal to 1.0 even when MBZ returns a
				// normalised barcode string (e.g. without a leading zero).
				c.Signals.Barcode = 1.0
				c.OverallConfidence = overallConfidence(c.Signals)
				return []domain.MusicReleaseCandidate{c}
			case err != nil && !errors.Is(err, ports.ErrNotFound):
				slog.WarnContext(ctx, "barcode lookup failed", "root", root, "err", err)
			}
		}
	}

	rgs := a.resolveReleaseGroups(ctx, root, tags)
	rgSrc := releaseGroupSource(a.sources)

	var candidates []domain.MusicReleaseCandidate
	for _, rg := range rgs {
		var releases []*ports.ExternalMusicRelease
		if rgSrc != nil && rg.mbid != "" {
			rels, err := rgSrc.FetchReleaseGroupReleases(ctx, rg.mbid)
			if err != nil {
				slog.WarnContext(ctx, "fetch rg releases failed", "root", root, "rg_mbid", rg.mbid, "err", err)
			} else {
				releases = rels
			}
		}
		if len(releases) == 0 {
			// No editions available: still surface the Release Group so the fuzzy
			// or ISRC match is visible in the queue.
			c := candidateFromReleaseGroup(rg, rgTitleFromTags(tags))
			a.scoreCandidate(ctx, root, tags, &c, rg.matchedISRCs)
			candidates = append(candidates, c)
			continue
		}
		for _, rel := range releases {
			c := candidateFromRelease(rel, rg.title, rg.mbid, rg.artistName, rg.artistMBID)
			a.scoreCandidate(ctx, root, tags, &c, rg.matchedISRCs)
			candidates = append(candidates, c)
		}
	}
	return candidates
}

// resolveReleaseGroups gathers candidate Release Groups from ISRC consensus and
// fuzzy artist-discography matching, de-duplicated by MBID in resolution order.
func (a *musicAlbumIdentifier) resolveReleaseGroups(ctx context.Context, root string, tags domain.MusicTagSummary) []releaseGroupRef {
	seen := make(map[string]*releaseGroupRef)
	var order []string
	add := func(mbid, title, artistMBID, artistName string) *releaseGroupRef {
		if r, ok := seen[mbid]; ok {
			return r
		}
		r := &releaseGroupRef{mbid: mbid, title: title, artistMBID: artistMBID, artistName: artistName}
		seen[mbid] = r
		order = append(order, mbid)
		return r
	}

	if src := isrcSource(a.sources); src != nil {
		for _, isrc := range tags.ISRCs {
			if isrc == "" {
				continue
			}
			rgMBID, err := src.LookupISRC(ctx, isrc)
			if err != nil {
				slog.WarnContext(ctx, "isrc lookup failed", "root", root, "isrc", isrc, "err", err)
				continue
			}
			if rgMBID == "" {
				continue
			}
			r := add(rgMBID, rgTitleFromTags(tags), "", tags.AlbumArtist)
			r.matchedISRCs = append(r.matchedISRCs, isrc)
		}
	}

	a.resolveFuzzyReleaseGroups(ctx, tags, add)

	out := make([]releaseGroupRef, 0, len(order))
	for _, id := range order {
		out = append(out, *seen[id])
	}
	return out
}

// resolveFuzzyReleaseGroups searches for the album artist, ranks their Release
// Groups by name similarity to the album tag, and registers the best matches.
func (a *musicAlbumIdentifier) resolveFuzzyReleaseGroups(ctx context.Context, tags domain.MusicTagSummary, add func(mbid, title, artistMBID, artistName string) *releaseGroupRef) {
	if tags.AlbumArtist == "" {
		return
	}
	studioSrc := studioSearchSource(a.sources)
	entrySrc := entryContentSource(a.sources)
	if studioSrc == nil || entrySrc == nil {
		return
	}

	artists, err := studioSrc.SearchStudios(ctx, tags.AlbumArtist, 1)
	if err != nil || len(artists) == 0 {
		return
	}
	artist := artists[0]

	groups, _, _, err := entrySrc.FetchEntryContent(ctx, domain.ContentTypeMusic, artist.ExternalID, 1, 100)
	if err != nil {
		return
	}

	stripped := StripAlbumSuffixes(tags.AlbumTitle)
	type scored struct {
		group *domain.ExternalGroup
		score float64
	}
	ranked := make([]scored, 0, len(groups))
	for _, g := range groups {
		s := albumTagSimilarity(stripped, StripAlbumSuffixes(g.Title))
		if s <= 0 {
			continue
		}
		ranked = append(ranked, scored{group: g, score: s})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	for i, sc := range ranked {
		if i >= maxFuzzyReleaseGroups {
			break
		}
		add(sc.group.ExternalID, sc.group.Title, artist.ExternalID, artist.Name)
	}
}

// scoreCandidate computes the seven-signal breakdown and overall confidence for
// one candidate. Barcode, ISRC, fuzzy name, and track count are derived from the
// Release Group / Release metadata available here; duration, track-title set,
// and AcoustID require per-track MBZ detail fetched during import (Task 16) and
// remain 0.0 but are always present so the queue UI renders every signal.
func (a *musicAlbumIdentifier) scoreCandidate(ctx context.Context, root string, tags domain.MusicTagSummary, c *domain.MusicReleaseCandidate, matchedISRCs []string) {
	sig := domain.MusicConfidenceSignals{
		Barcode:     ScoreBarcodeSignal(ctx, root, tags, *c),
		ISRC:        ScoreISRCSignal(ctx, root, tags, matchedISRCs),
		RGNameFuzzy: albumTagSimilarity(StripAlbumSuffixes(tags.AlbumTitle), StripAlbumSuffixes(c.ReleaseGroupTitle)),
	}
	if tags.TotalTracks > 0 && c.ReleaseTrackCount == tags.TotalTracks {
		sig.TrackCount = 1.0
	}
	c.Signals = sig
	c.OverallConfidence = overallConfidence(sig)
}

// overallConfidence combines the signal breakdown into a single 0.0–1.0 score.
// A barcode match alone is authoritative; otherwise the signals contribute a
// weighted sum capped at 1.0.
func overallConfidence(s domain.MusicConfidenceSignals) float64 {
	if s.Barcode >= 1.0 {
		return 1.0
	}
	sum := s.Barcode*weightBarcode +
		s.ISRC*weightISRC +
		s.RGNameFuzzy*weightRGNameFuzzy +
		s.TrackCount*weightTrackCount +
		s.TrackTitleSet*weightTrackTitleSet +
		s.Duration*weightDuration +
		s.AcoustID*weightAcoustID
	if sum > 1.0 {
		return 1.0
	}
	return sum
}

// candidateFromRelease builds a candidate from a specific MBZ Release edition.
func candidateFromRelease(rel *ports.ExternalMusicRelease, rgTitle, rgMBID, artistName, artistMBID string) domain.MusicReleaseCandidate {
	title := rgTitle
	if title == "" {
		title = rel.Title
	}
	return domain.MusicReleaseCandidate{
		ArtistMBID:         artistMBID,
		ArtistName:         artistName,
		ReleaseGroupMBID:   rgMBID,
		ReleaseGroupTitle:  title,
		ReleaseMBID:        rel.MBID,
		ReleaseTitle:       rel.Title,
		ReleaseDate:        rel.Date,
		ReleaseLabel:       rel.Label,
		ReleaseCountry:     rel.Country,
		ReleaseBarcode:     rel.Barcode,
		ReleaseFormat:      rel.Format,
		ReleaseMediumCount: rel.MediumCount,
		ReleaseTrackCount:  rel.TrackCount,
	}
}

// candidateFromReleaseGroup builds a candidate when a Release Group matched but
// no editions could be fetched, so the match still appears in the queue.
func candidateFromReleaseGroup(rg releaseGroupRef, fallbackTitle string) domain.MusicReleaseCandidate {
	title := rg.title
	if title == "" {
		title = fallbackTitle
	}
	return domain.MusicReleaseCandidate{
		ArtistMBID:        rg.artistMBID,
		ArtistName:        rg.artistName,
		ReleaseGroupMBID:  rg.mbid,
		ReleaseGroupTitle: title,
	}
}

// rgTitleFromTags returns the album tag with edition/variant suffixes stripped,
// used as the Release Group title when MBZ does not supply one directly.
func rgTitleFromTags(tags domain.MusicTagSummary) string {
	return StripAlbumSuffixes(tags.AlbumTitle)
}

// tagsHaveISRCs reports whether any track carries a non-empty ISRC.
func tagsHaveISRCs(tags domain.MusicTagSummary) bool {
	for _, isrc := range tags.ISRCs {
		if isrc != "" {
			return true
		}
	}
	return false
}

// ── source capability lookups ─────────────────────────────────────────────────

func barcodeSource(sources []ports.MetadataSource) ports.BarcodeLookupSource {
	for _, s := range sources {
		if src, ok := s.(ports.BarcodeLookupSource); ok {
			return src
		}
	}
	return nil
}

func isrcSource(sources []ports.MetadataSource) ports.ISRCLookupSource {
	for _, s := range sources {
		if src, ok := s.(ports.ISRCLookupSource); ok {
			return src
		}
	}
	return nil
}

func releaseGroupSource(sources []ports.MetadataSource) ports.ReleaseGroupContentSource {
	for _, s := range sources {
		if src, ok := s.(ports.ReleaseGroupContentSource); ok {
			return src
		}
	}
	return nil
}

func studioSearchSource(sources []ports.MetadataSource) ports.StudioSearchSource {
	for _, s := range sources {
		if src, ok := s.(ports.StudioSearchSource); ok {
			return src
		}
	}
	return nil
}

func entryContentSource(sources []ports.MetadataSource) ports.EntryContentSource {
	for _, s := range sources {
		if src, ok := s.(ports.EntryContentSource); ok {
			return src
		}
	}
	return nil
}

// NoopAlbumImporter is a placeholder AlbumImporter used until the concrete
// release import service (Task 16) is wired. It logs that an auto-import
// decision was reached without creating any entities so the pipeline is
// exercisable end to end before import lands.
type NoopAlbumImporter struct{}

var _ ports.AlbumImporter = NoopAlbumImporter{}

// NewNoopAlbumImporter returns a no-op AlbumImporter.
func NewNoopAlbumImporter() ports.AlbumImporter { return NoopAlbumImporter{} }

// ImportRelease logs the skipped import and returns nil.
func (NoopAlbumImporter) ImportRelease(ctx context.Context, candidate domain.MusicReleaseCandidate, group ports.ScannedFileGroup) error {
	slog.InfoContext(ctx, "album import skipped: no importer wired",
		"root", group.RootPath, "release_mbid", candidate.ReleaseMBID)
	return nil
}
