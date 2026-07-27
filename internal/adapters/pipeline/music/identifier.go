package music

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Provenance values recorded in a candidate's Metadata["sources"] — see
// docs/technical/pipeline-music-identifier.md's "Provenance, not a score
// cap" section. Purely informational; never fed back into the signal math
// in step 3.
const (
	sourceDirectID = "direct_id"
	sourceBarcode  = "barcode"
	sourceISRC     = "isrc"
	sourceTagFuzzy = "tag_fuzzy"
	sourceFilename = "filename"
	sourceAcoustic = "acoustic"
)

const (
	// durationToleranceSeconds is how far a candidate track's duration may
	// differ from the group's and still count as "within tolerance" for
	// duration_match — a starting number, not validated against real
	// fixtures yet, same caveat every other numeric constant in this
	// design carries (docs/technical/music-identification.md).
	durationToleranceSeconds = 3.0

	// titleMatchThreshold/titlePartialThreshold bucket a per-track title
	// similarity (see nameSimilarity) into a full or partial match credit
	// for title_set_overlap — worked example 2 counts a title differing
	// only in minor formatting ("Edge of Seventeen (Live)" vs "Edge Of
	// 17") as a lower-confidence probable match rather than a miss. Plain
	// Levenshtein similarity won't always catch that specific case; the
	// exact fuzzy algorithm is explicitly not pinned by
	// docs/technical/music-identification.md, so these thresholds are a
	// starting point for the M8 fixture suite to validate, not asserted
	// correct.
	titleMatchThreshold   = 0.85
	titlePartialThreshold = 0.6
	titlePartialCredit    = 0.5

	// isrcConsensusMajorityThreshold is the isrc_consensus_fraction value
	// above which a candidate is considered to have "consensus reached
	// across most files in the group" for unique_id tier purposes.
	isrcConsensusMajorityThreshold = 0.5

	// releaseTrackCountToleranceFraction is the direct-ID sanity check's
	// tolerance band ("roughly matches") — a fraction of the candidate
	// release's own track count, floored at 1 track so a tiny release
	// isn't held to an impossible zero-tolerance bar.
	releaseTrackCountToleranceFraction = 0.10
)

// Identifier implements ports.Identifier for domain.ContentTypeMusic: the
// M7 candidate-generation cascade — direct-ID short-circuit, then unioned
// candidate collection (barcode/ISRC/fuzzy-tag/filename-fallback/AcoustID),
// then per-candidate signal scoring. Sets Tier/Signals/Metadata only; never
// Score, per the M7/M8 boundary in
// docs/adr/0025-music-identification-confidence-scoring.md and
// docs/technical/pipeline-music-identifier.md.
type Identifier struct {
	mb             ports.MusicBrainzClient
	acoustID       ports.AcoustIDClient
	filenameParser ports.FilenameParser

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.Identifier = (*Identifier)(nil)

// NewIdentifier constructs an Identifier backed by mb, acoustID, and
// filenameParser. Reuses the same Option/WithLogger/WithTracerProvider
// used by New (FileFingerprinter's constructor) — both types share the
// same {logger, tracerProvider} shape, so a second declaration would be
// pure duplication.
func NewIdentifier(mb ports.MusicBrainzClient, acoustID ports.AcoustIDClient, filenameParser ports.FilenameParser, opts ...Option) *Identifier {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Identifier{
		mb:             mb,
		acoustID:       acoustID,
		filenameParser: filenameParser,
		logger:         o.logger.With("component", "adapters.pipeline.music.identifier"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.Identifier.
func (id *Identifier) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// Identify implements ports.Identifier, per
// docs/technical/pipeline-music-identifier.md's cascade: a resolving
// direct-ID short-circuit returns immediately with one Tier=direct_id
// candidate; otherwise every applicable source is collected together
// (never a strict try-one-then-next cascade), deduped by release MBID, and
// every distinct candidate is scored against every signal that applies to
// it regardless of which source first surfaced it.
func (id *Identifier) Identify(ctx context.Context, fp domain.Fingerprint, paths []string, groupPath, scanRoot string) ([]domain.MatchCandidate, error) {
	ctx, span := id.tracer.Start(ctx, "music.identifier.identify", trace.WithAttributes(
		attribute.String("pipeline.group_path", groupPath),
	))
	defer span.End()

	if candidate, ok, err := id.directID(ctx, fp); err != nil {
		return nil, err
	} else if ok {
		span.SetAttributes(attribute.Bool("pipeline.direct_id", true))
		return []domain.MatchCandidate{candidate}, nil
	}

	collected, err := id.collectCandidates(ctx, fp, paths, groupPath, scanRoot)
	if err != nil {
		return nil, err
	}
	if len(collected.releaseMBIDs) == 0 {
		return nil, nil
	}

	nameArtist, nameAlbum, haveNameQuery := id.resolveNameQuery(ctx, fp, groupPath, scanRoot)

	mbids := make([]string, 0, len(collected.releaseMBIDs))
	for mbid := range collected.releaseMBIDs {
		mbids = append(mbids, mbid)
	}
	sort.Strings(mbids)

	candidates := make([]domain.MatchCandidate, 0, len(mbids))
	for _, mbid := range mbids {
		release, err := id.mb.LookupRelease(ctx, mbid)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("adapters/pipeline/music: scoring lookup release %s: %w", mbid, err)
		}
		candidates = append(candidates, id.scoreCandidate(fp, release, collected.sources[mbid], collected.acoustic, collected.acoustidRan, nameArtist, nameAlbum, haveNameQuery))
	}

	id.logger.DebugContext(ctx, "identified candidates", "group_path", groupPath, "candidate_count", len(candidates))
	return candidates, nil
}

// directID implements step 1 of the cascade: MUSICBRAINZ_ALBUMID first,
// falling back to MUSICBRAINZ_RELEASEGROUPID only if the first didn't
// resolve. Either branch's sanity-check failure falls through to
// candidate collection — never accepts anyway, never returns empty outright
// — per the issue's resolved open question.
func (id *Identifier) directID(ctx context.Context, fp domain.Fingerprint) (domain.MatchCandidate, bool, error) {
	candidate, ok, err := id.directIDFromRelease(ctx, fp)
	if err != nil || ok {
		return candidate, ok, err
	}
	return id.directIDFromReleaseGroup(ctx, fp)
}

// directIDFromRelease checks Fingerprint.Metadata["embedded_release_mbids"]
// — if it holds exactly one value, looks it up and sanity-checks its track
// count against the group's.
func (id *Identifier) directIDFromRelease(ctx context.Context, fp domain.Fingerprint) (domain.MatchCandidate, bool, error) {
	mbids, _ := fp.Metadata["embedded_release_mbids"].([]string)
	if len(mbids) != 1 {
		return domain.MatchCandidate{}, false, nil
	}

	release, err := id.mb.LookupRelease(ctx, mbids[0])
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return domain.MatchCandidate{}, false, nil
		}
		return domain.MatchCandidate{}, false, fmt.Errorf("adapters/pipeline/music: direct-ID lookup release %s: %w", mbids[0], err)
	}

	groupCount, _ := fp.Metadata["track_count"].(int)
	if !releaseTrackCountSane(groupCount, releaseTrackCount(release)) {
		return domain.MatchCandidate{}, false, nil
	}
	return newDirectIDCandidate(release), true, nil
}

// directIDFromReleaseGroup checks
// Fingerprint.Metadata["embedded_releasegroup_mbids"] the same way, but a
// release group isn't itself a valid MatchCandidate.ExternalRef (M9's
// Persister cascade needs a release MBID) — so a resolving release group is
// narrowed to whichever one of its releases has a track count that
// sanity-checks against the group's. More than one release passing that
// check is treated as ambiguous at this stage (no way to prefer one release
// over another from track count alone) and falls through to collection,
// same as a straightforward sanity-check failure.
func (id *Identifier) directIDFromReleaseGroup(ctx context.Context, fp domain.Fingerprint) (domain.MatchCandidate, bool, error) {
	rgMBIDs, _ := fp.Metadata["embedded_releasegroup_mbids"].([]string)
	if len(rgMBIDs) != 1 {
		return domain.MatchCandidate{}, false, nil
	}
	rgMBID := rgMBIDs[0]

	if _, err := id.mb.LookupReleaseGroup(ctx, rgMBID); err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return domain.MatchCandidate{}, false, nil
		}
		return domain.MatchCandidate{}, false, fmt.Errorf("adapters/pipeline/music: direct-ID lookup release group %s: %w", rgMBID, err)
	}

	releases, err := id.mb.ListReleasesForReleaseGroup(ctx, rgMBID)
	if err != nil {
		return domain.MatchCandidate{}, false, fmt.Errorf("adapters/pipeline/music: listing releases for release group %s: %w", rgMBID, err)
	}

	groupCount, _ := fp.Metadata["track_count"].(int)
	var sane string
	matches := 0
	for _, r := range releases {
		if releaseTrackCountSane(groupCount, releaseTrackCount(&r)) {
			sane = r.ID
			matches++
		}
	}
	if matches != 1 {
		return domain.MatchCandidate{}, false, nil
	}

	release, err := id.mb.LookupRelease(ctx, sane)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return domain.MatchCandidate{}, false, nil
		}
		return domain.MatchCandidate{}, false, fmt.Errorf("adapters/pipeline/music: direct-ID lookup release %s: %w", sane, err)
	}
	return newDirectIDCandidate(release), true, nil
}

func newDirectIDCandidate(release *ports.Release) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: release.ID,
		Title:       release.Title,
		Tier:        domain.MatchTierDirectID,
		Signals:     map[string]float64{},
		Metadata: map[string]any{
			"sources":            []string{sourceDirectID},
			"release_group_mbid": releaseGroupMBID(release),
		},
	}
}

// collectionResult accumulates step 2's union of discovered release MBIDs,
// per-MBID provenance, and per-file AcoustID results (cached here for reuse
// by step 3's acoustic_agreement scoring, since AcoustID is deliberately
// run at most once per file for the whole Identify call — fpcalc/network
// calls are the most expensive step in the cascade).
type collectionResult struct {
	releaseMBIDs map[string]struct{}
	sources      map[string]map[string]struct{}
	acoustic     map[string][]ports.AcoustIDMatch
	acoustidRan  bool
}

func newCollectionResult() collectionResult {
	return collectionResult{
		releaseMBIDs: map[string]struct{}{},
		sources:      map[string]map[string]struct{}{},
		acoustic:     map[string][]ports.AcoustIDMatch{},
	}
}

func (c *collectionResult) add(mbid, source string) {
	if mbid == "" {
		return
	}
	c.releaseMBIDs[mbid] = struct{}{}
	if c.sources[mbid] == nil {
		c.sources[mbid] = map[string]struct{}{}
	}
	c.sources[mbid][source] = struct{}{}
}

// collectCandidates implements step 2: every applicable source gathered
// together, not a strict try-one-then-next cascade, unioned into one
// deduplicated release-MBID set. A source returning ports.ErrNotFound (an
// unknown ISRC/MBID) simply contributes nothing from that one lookup and
// collection continues; any other MusicBrainz error is treated as a hard
// failure and propagated — MusicBrainz being unreachable isn't a per-file
// phenomenon the way a corrupt audio file is, so it shouldn't be silently
// absorbed into "found nothing". AcoustID's own per-file calls (fpcalc,
// then the API) are the one exception: a single unreadable/unrecognized
// file there is tolerated and skipped, the same "one bad file doesn't stop
// the group" treatment M4's Fingerprint step gets.
func (id *Identifier) collectCandidates(ctx context.Context, fp domain.Fingerprint, paths []string, groupPath, scanRoot string) (collectionResult, error) {
	result := newCollectionResult()

	barcodeFound, err := id.collectBarcode(ctx, &result, fp)
	if err != nil {
		return result, err
	}
	isrcFound, err := id.collectISRCConsensus(ctx, &result, fp)
	if err != nil {
		return result, err
	}
	uniqueIDFound := barcodeFound || isrcFound

	albumArtist, album := fp.Tags["ALBUMARTIST"], fp.Tags["ALBUM"]
	tagFuzzyFound := false
	if albumArtist != "" || album != "" {
		found, err := id.searchAndUnionReleaseGroups(ctx, &result, albumArtist, album, sourceTagFuzzy)
		if err != nil {
			return result, err
		}
		tagFuzzyFound = found
	}

	if !tagFuzzyFound {
		if parsedArtist, parsedAlbum, ok := id.filenameParser.Parse(ctx, groupPath, scanRoot); ok {
			if _, err := id.searchAndUnionReleaseGroups(ctx, &result, parsedArtist, parsedAlbum, sourceFilename); err != nil {
				return result, err
			}
		}
	}

	if !uniqueIDFound {
		if err := id.collectAcoustID(ctx, &result, paths); err != nil {
			return result, err
		}
	}

	return result, nil
}

// collectBarcode implements the barcode-search half of step 2. Reports
// whether it found anything, feeding the AcoustID structural gate below.
func (id *Identifier) collectBarcode(ctx context.Context, result *collectionResult, fp domain.Fingerprint) (bool, error) {
	barcode := strings.TrimSpace(fp.Tags["BARCODE"])
	if barcode == "" {
		return false, nil
	}
	releases, err := id.mb.SearchReleaseByBarcode(ctx, barcode)
	if err != nil {
		return false, fmt.Errorf("adapters/pipeline/music: searching release by barcode: %w", err)
	}
	for _, r := range releases {
		result.add(r.ID, sourceBarcode)
	}
	return len(releases) > 0, nil
}

// collectISRCConsensus implements the ISRC-consensus half of step 2:
// looking up every non-empty ISRC in the group and unioning every release
// any of them resolves to. Reports whether it found anything, feeding the
// AcoustID structural gate below.
func (id *Identifier) collectISRCConsensus(ctx context.Context, result *collectionResult, fp domain.Fingerprint) (bool, error) {
	isrcs, _ := fp.Metadata["track_isrcs"].([]string)
	found := false
	for _, isrc := range isrcs {
		if isrc == "" {
			continue
		}
		recordings, err := id.mb.LookupRecordingByISRC(ctx, isrc)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return found, fmt.Errorf("adapters/pipeline/music: looking up recording by ISRC %s: %w", isrc, err)
		}
		for _, rec := range recordings {
			for _, rel := range rec.Releases {
				result.add(rel.ID, sourceISRC)
				found = true
			}
		}
	}
	return found, nil
}

// searchAndUnionReleaseGroups searches release groups by artistName/album,
// resolves each hit to its specific releases, and unions every discovered
// release MBID into result under source. Reports whether the search itself
// found any release group — the trigger the filename fallback and unique-ID
// tier both need, distinct from whether resolving those release groups down
// to releases actually produced anything.
func (id *Identifier) searchAndUnionReleaseGroups(ctx context.Context, result *collectionResult, artistName, album, source string) (bool, error) {
	rgs, err := id.mb.SearchReleaseGroups(ctx, artistName, album)
	if err != nil {
		return false, fmt.Errorf("adapters/pipeline/music: searching release groups: %w", err)
	}
	for _, rg := range rgs {
		releases, err := id.mb.ListReleasesForReleaseGroup(ctx, rg.ID)
		if err != nil {
			return len(rgs) > 0, fmt.Errorf("adapters/pipeline/music: listing releases for release group %s: %w", rg.ID, err)
		}
		for _, r := range releases {
			result.add(r.ID, source)
		}
	}
	return len(rgs) > 0, nil
}

// collectAcoustID runs Fingerprint+Lookup for every file in the group (not
// a sample — this path is already the less-common case, and completeness
// matters more here than the extra cost, per
// docs/technical/pipeline-music-identifier.md), resolving every matched
// recording's release groups down to specific releases the same way the
// fuzzy-search path does.
func (id *Identifier) collectAcoustID(ctx context.Context, result *collectionResult, paths []string) error {
	result.acoustidRan = true

	for _, path := range paths {
		fingerprint, duration, err := id.acoustID.Fingerprint(ctx, path)
		if err != nil {
			id.logger.WarnContext(ctx, "acoustid fingerprint failed, skipping file", "path", path, "error", err)
			continue
		}
		matches, err := id.acoustID.Lookup(ctx, fingerprint, duration)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			id.logger.WarnContext(ctx, "acoustid lookup failed, skipping file", "path", path, "error", err)
			continue
		}
		if len(matches) == 0 {
			continue
		}
		result.acoustic[path] = matches

		rgSeen := map[string]struct{}{}
		for _, m := range matches {
			for _, rec := range m.Recordings {
				for _, rg := range rec.ReleaseGroups {
					rgSeen[rg.MBID] = struct{}{}
				}
			}
		}
		for rgMBID := range rgSeen {
			releases, err := id.mb.ListReleasesForReleaseGroup(ctx, rgMBID)
			if err != nil {
				return fmt.Errorf("adapters/pipeline/music: listing releases for AcoustID release group %s: %w", rgMBID, err)
			}
			for _, r := range releases {
				result.add(r.ID, sourceAcoustic)
			}
		}
	}
	return nil
}

// resolveNameQuery returns the name data available for name_fuzzy_score
// comparison: ALBUMARTIST/ALBUM tags when either is non-empty, otherwise
// the filename/folder-name fallback guess — the same tags-first,
// filename-fallback-when-genuinely-untagged policy the collection cascade
// uses, but evaluated independently since scoring needs it for every
// candidate regardless of which collection step actually ran.
func (id *Identifier) resolveNameQuery(ctx context.Context, fp domain.Fingerprint, groupPath, scanRoot string) (artist, album string, ok bool) {
	if artist, album := fp.Tags["ALBUMARTIST"], fp.Tags["ALBUM"]; artist != "" || album != "" {
		return artist, album, true
	}
	return id.filenameParser.Parse(ctx, groupPath, scanRoot)
}

// scoreCandidate implements step 3: every signal that applies to release,
// regardless of which source in collectCandidates first surfaced it. A
// signal is present in the returned map only when real data existed on
// both our side and the candidate's — absent, never zero, when it couldn't
// be evaluated, per the M7->M8 signal key contract in
// docs/technical/pipeline-music-confidence-score.md.
func (id *Identifier) scoreCandidate(fp domain.Fingerprint, release *ports.Release, sources map[string]struct{}, acoustic map[string][]ports.AcoustIDMatch, acoustidRan bool, nameArtist, nameAlbum string, haveNameQuery bool) domain.MatchCandidate {
	signals := map[string]float64{}

	if v, ok := barcodeMatchSignal(fp, release); ok {
		signals["barcode_match"] = v
	}
	if v, ok := isrcConsensusSignal(fp, release); ok {
		signals["isrc_consensus_fraction"] = v
	}
	if v, ok := nameFuzzySignal(nameArtist, nameAlbum, haveNameQuery, release); ok {
		signals["name_fuzzy_score"] = v
	}
	if v, ok := trackCountSignal(fp, release); ok {
		signals["track_count_match"] = v
	}
	if v, ok := titleSetSignal(fp, release); ok {
		signals["title_set_overlap"] = v
	}
	if v, ok := durationSignal(fp, release); ok {
		signals["duration_match"] = v
	}
	rgMBID := releaseGroupMBID(release)
	if acoustidRan {
		if v, ok := acousticAgreementSignal(acoustic, rgMBID); ok {
			signals["acoustic_agreement"] = v
		}
	}

	return domain.MatchCandidate{
		ExternalRef: release.ID,
		Title:       release.Title,
		Tier:        classifyTier(signals),
		Signals:     signals,
		Metadata: map[string]any{
			"sources":            sortedSources(sources),
			"release_group_mbid": rgMBID,
		},
	}
}

// classifyTier assigns Tier from the strongest evidence that actually
// applies to this candidate — never from which source first discovered it,
// per docs/technical/pipeline-music-identifier.md's "Score every distinct
// candidate against every applicable signal" section. This is a structural
// classification only (which evidence class applies); the degree of
// agreement within that class is M8's job, not this function's.
func classifyTier(signals map[string]float64) domain.MatchTier {
	if v, ok := signals["barcode_match"]; ok && v >= 1.0 {
		return domain.MatchTierUniqueID
	}
	if v, ok := signals["isrc_consensus_fraction"]; ok && v > isrcConsensusMajorityThreshold {
		return domain.MatchTierUniqueID
	}

	agreeing := 0
	for _, key := range [...]string{"name_fuzzy_score", "track_count_match", "title_set_overlap", "duration_match"} {
		if v, ok := signals[key]; ok && v > 0 {
			agreeing++
		}
	}
	if agreeing >= 2 {
		return domain.MatchTierFuzzy
	}
	if _, ok := signals["acoustic_agreement"]; ok {
		return domain.MatchTierAcoustic
	}
	return domain.MatchTierFuzzy
}

func barcodeMatchSignal(fp domain.Fingerprint, release *ports.Release) (float64, bool) {
	ours := strings.TrimSpace(fp.Tags["BARCODE"])
	theirs := strings.TrimSpace(release.Barcode)
	if ours == "" || theirs == "" {
		return 0, false
	}
	if ours == theirs {
		return 1.0, true
	}
	return 0.0, true
}

func isrcConsensusSignal(fp domain.Fingerprint, release *ports.Release) (float64, bool) {
	isrcs, _ := fp.Metadata["track_isrcs"].([]string)
	total := 0
	for _, isrc := range isrcs {
		if isrc != "" {
			total++
		}
	}
	if total == 0 {
		return 0, false
	}

	theirs := releaseTrackISRCs(release)
	matched := 0
	for _, isrc := range isrcs {
		if isrc == "" {
			continue
		}
		if _, ok := theirs[isrc]; ok {
			matched++
		}
	}
	return float64(matched) / float64(total), true
}

func nameFuzzySignal(nameArtist, nameAlbum string, haveNameQuery bool, release *ports.Release) (float64, bool) {
	if !haveNameQuery {
		return 0, false
	}
	candidateArtist := releaseArtistCreditName(release)
	candidateAlbum := releaseGroupOrReleaseTitle(release)
	if candidateArtist == "" && candidateAlbum == "" {
		return 0, false
	}

	var scores []float64
	if nameArtist != "" && candidateArtist != "" {
		scores = append(scores, nameSimilarity(nameArtist, candidateArtist))
	}
	if nameAlbum != "" && candidateAlbum != "" {
		scores = append(scores, nameSimilarity(nameAlbum, candidateAlbum))
	}
	if len(scores) == 0 {
		return 0, false
	}
	return average(scores), true
}

func trackCountSignal(fp domain.Fingerprint, release *ports.Release) (float64, bool) {
	groupCount, _ := fp.Metadata["track_count"].(int)
	releaseCount := releaseTrackCount(release)
	if groupCount <= 0 || releaseCount <= 0 {
		return 0, false
	}
	lo, hi := groupCount, releaseCount
	if lo > hi {
		lo, hi = hi, lo
	}
	return float64(lo) / float64(hi), true
}

// titleSetSignal compares the group's ordered track_titles (indexed by
// (disc, track) — see docs/technical/pipeline-music-fingerprinter.md)
// against release's tracks flattened in the same (medium, track) position
// order, position-for-position. The denominator is always the group's own
// track count, matching the "10/10", "30/32" coverage-fraction framing in
// docs/technical/music-identification.md's worked examples, not just the
// subset of positions that happened to have a comparable candidate track.
func titleSetSignal(fp domain.Fingerprint, release *ports.Release) (float64, bool) {
	titles, _ := fp.Metadata["track_titles"].([]string)
	if len(titles) == 0 {
		return 0, false
	}
	candidateTracks := flattenReleaseTracks(release)
	if len(candidateTracks) == 0 {
		return 0, false
	}

	credit := 0.0
	for i, title := range titles {
		if title == "" || i >= len(candidateTracks) {
			continue
		}
		switch sim := nameSimilarity(title, candidateTracks[i].Title); {
		case sim >= titleMatchThreshold:
			credit += 1
		case sim >= titlePartialThreshold:
			credit += titlePartialCredit
		}
	}
	return credit / float64(len(titles)), true
}

func durationSignal(fp domain.Fingerprint, release *ports.Release) (float64, bool) {
	durations, _ := fp.Metadata["track_durations"].([]float64)
	if len(durations) == 0 {
		return 0, false
	}
	candidateTracks := flattenReleaseTracks(release)
	if len(candidateTracks) == 0 {
		return 0, false
	}

	within := 0
	for i, d := range durations {
		if d <= 0 || i >= len(candidateTracks) || candidateTracks[i].Length <= 0 {
			continue
		}
		candidateSeconds := float64(candidateTracks[i].Length) / 1000.0
		if math.Abs(d-candidateSeconds) <= durationToleranceSeconds {
			within++
		}
	}
	return float64(within) / float64(len(durations)), true
}

func acousticAgreementSignal(acoustic map[string][]ports.AcoustIDMatch, releaseGroupMBID string) (float64, bool) {
	if len(acoustic) == 0 {
		return 0, false
	}
	agree := 0
	for _, matches := range acoustic {
		if acousticMatchesAgree(matches, releaseGroupMBID) {
			agree++
		}
	}
	return float64(agree) / float64(len(acoustic)), true
}

func acousticMatchesAgree(matches []ports.AcoustIDMatch, releaseGroupMBID string) bool {
	for _, m := range matches {
		for _, rec := range m.Recordings {
			for _, rg := range rec.ReleaseGroups {
				if rg.MBID == releaseGroupMBID {
					return true
				}
			}
		}
	}
	return false
}

// releaseTrackCountSane implements the direct-ID short-circuit's "cheap
// sanity check": groupCount must roughly match releaseCount, within a
// tolerance proportional to the release's own size (floored at one track).
// Either count being non-positive (no data to check against) is treated as
// a failed check, not a free pass — a direct-ID short-circuit with nothing
// to sanity-check against is exactly the case this guard exists to catch.
func releaseTrackCountSane(groupCount, releaseCount int) bool {
	if groupCount <= 0 || releaseCount <= 0 {
		return false
	}
	diff := groupCount - releaseCount
	if diff < 0 {
		diff = -diff
	}
	tolerance := int(float64(releaseCount) * releaseTrackCountToleranceFraction)
	if tolerance < 1 {
		tolerance = 1
	}
	return diff <= tolerance
}

func releaseTrackCount(release *ports.Release) int {
	total := 0
	for _, m := range release.Media {
		total += m.TrackCount
	}
	return total
}

func releaseTrackISRCs(release *ports.Release) map[string]struct{} {
	set := map[string]struct{}{}
	for _, m := range release.Media {
		for _, t := range m.Tracks {
			if t.Recording == nil {
				continue
			}
			for _, isrc := range t.Recording.ISRCs {
				if isrc != "" {
					set[isrc] = struct{}{}
				}
			}
		}
	}
	return set
}

// flattenReleaseTracks orders release's tracks by (Medium.Position,
// Track.Position) — the same (disc, track) shape the group's consensus
// ordered lists use, so the two align position-for-position with no
// translation needed, vinyl included.
func flattenReleaseTracks(release *ports.Release) []ports.Track {
	media := append([]ports.Medium(nil), release.Media...)
	sort.SliceStable(media, func(i, j int) bool { return media[i].Position < media[j].Position })

	tracks := make([]ports.Track, 0, len(media))
	for _, m := range media {
		ts := append([]ports.Track(nil), m.Tracks...)
		sort.SliceStable(ts, func(i, j int) bool { return ts[i].Position < ts[j].Position })
		tracks = append(tracks, ts...)
	}
	return tracks
}

func releaseGroupMBID(release *ports.Release) string {
	if release.ReleaseGroup != nil {
		return release.ReleaseGroup.ID
	}
	return ""
}

func releaseArtistCreditName(release *ports.Release) string {
	names := make([]string, 0, len(release.ArtistCredit))
	for _, ac := range release.ArtistCredit {
		if ac.Name != "" {
			names = append(names, ac.Name)
		}
	}
	return strings.Join(names, " ")
}

func releaseGroupOrReleaseTitle(release *ports.Release) string {
	if release.ReleaseGroup != nil && release.ReleaseGroup.Title != "" {
		return release.ReleaseGroup.Title
	}
	return release.Title
}

// nameSimilarity is a case-insensitive, whitespace-trimmed similarity score
// in [0, 1] based on normalized Levenshtein distance. The exact fuzzy
// algorithm is explicitly not pinned by
// docs/technical/music-identification.md — this reuses
// github.com/lithammer/fuzzysearch, already an indirect dependency, rather
// than adding a new one or hand-rolling Levenshtein.
func nameSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	maxLen := len([]rune(a))
	if l := len([]rune(b)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1
	}
	score := 1 - float64(fuzzy.LevenshteinDistance(a, b))/float64(maxLen)
	if score < 0 {
		return 0
	}
	return score
}

func sortedSources(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
