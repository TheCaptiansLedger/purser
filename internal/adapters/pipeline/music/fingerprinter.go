package music

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/pipeline/music"

// ffprobeBinary is the external binary Fingerprint shells out to — see
// docs/technical/pipeline-music-fingerprinter.md. Not configurable: it's
// resolved from PATH, the same assumption cmd/purser's startup preflight
// check verifies.
const ffprobeBinary = "ffprobe"

// canonicalTagAliases maps every tag Fingerprint extracts into
// domain.Fingerprint.Tags to the raw ffprobe key names known to carry it
// (beyond the canonical name itself), matched case-insensitively against
// ffprobe's reported tag map. Verified against real ffmpeg/ffprobe output:
// FLAC/Vorbis-comment containers normalize ALBUMARTIST/DISCNUMBER/
// TRACKNUMBER to lowercase album_artist/disc/track (libavformat's
// vorbiscomment metadata-conversion table) while leaving ALBUM/DATE/TITLE
// untouched; MP3/ID3v2 does the opposite — ALBUM/DATE/TITLE normalize to
// lowercase (standard frames) while ALBUMARTIST/DISCNUMBER/TRACKNUMBER and
// every custom TXXX frame (LABEL, CATALOGNUMBER, BARCODE, ISRC,
// MUSICBRAINZ_*) stay exactly as written. Every alias below accounts for a
// normalization actually observed against generated fixtures (see
// testdata/); M4A/iTunes freeform atoms may need further aliases once
// verified against real Picard-tagged files — not asserted complete, per
// the technical doc's own caveat that exact tag-key mapping needs
// verification against real files.
var canonicalTagAliases = map[string][]string{
	"ALBUM":                      {"album"},
	"ALBUMARTIST":                {"album_artist"},
	"DATE":                       {"date"},
	"LABEL":                      {"label"},
	"CATALOGNUMBER":              {"catalognumber"},
	"BARCODE":                    {"barcode"},
	"ISRC":                       {"isrc"},
	"TITLE":                      {"title"},
	"MUSICBRAINZ_ALBUMID":        nil,
	"MUSICBRAINZ_RELEASEGROUPID": nil,
	"MUSICBRAINZ_TRACKID":        {"MUSICBRAINZ_RELEASETRACKID"},
}

// discNumberAliases/trackNumberAliases are checked in order against
// ffprobe's tags, case-insensitively — see canonicalTagAliases' doc comment
// for why both the raw and ffmpeg-normalized forms are needed.
var (
	discNumberAliases  = []string{"DISCNUMBER", "disc", "discnumber"}
	trackNumberAliases = []string{"TRACKNUMBER", "track", "tracknumber"}
)

// ffprobeOutput is the subset of `ffprobe -show_format -show_streams -of
// json` this adapter reads. Tags can land on either format or the first
// stream depending on container — both are merged, format taking priority
// on a key collision.
type ffprobeOutput struct {
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
}

// FileFingerprinter implements ports.FileFingerprinter for
// domain.ContentTypeMusic: per-file `ffprobe` tag/duration extraction plus
// group-consensus reduction. See
// docs/technical/pipeline-music-fingerprinter.md,
// docs/adr/0025-music-identification-confidence-scoring.md.
type FileFingerprinter struct {
	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.FileFingerprinter = (*FileFingerprinter)(nil)

// New constructs a FileFingerprinter.
func New(opts ...Option) *FileFingerprinter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &FileFingerprinter{
		logger: o.logger.With("component", "adapters.pipeline.music.fingerprinter"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.FileFingerprinter.
func (f *FileFingerprinter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// Fingerprint implements ports.FileFingerprinter: runs ffprobe against
// path, parses its JSON output, and extracts tags/disc-number/track-number/
// duration per docs/technical/pipeline-music-fingerprinter.md's "Per-file
// extraction" section. discNumberGuess (the grouping implementation's
// folder-derived guess) is overridden by an embedded DISCNUMBER-family tag
// when present and parseable as an int; TRACKNUMBER is stored raw, exactly
// as ffprobe reports it, so vinyl side-lettering ("A1", "B2") survives
// unparsed.
func (f *FileFingerprinter) Fingerprint(ctx context.Context, path string, discNumberGuess int) (domain.Fingerprint, error) {
	ctx, span := f.tracer.Start(ctx, "music.fingerprinter.fingerprint", trace.WithAttributes(
		attribute.String("pipeline.path", path),
	))
	defer span.End()

	cmd := exec.CommandContext(ctx, ffprobeBinary, "-v", "quiet", "-show_format", "-show_streams", "-of", "json", path) //nolint:gosec // path is caller-supplied by design; fingerprinting arbitrary discovered files is this package's purpose
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		f.logger.ErrorContext(ctx, "ffprobe failed", "path", path, "error", err, "stderr", stderr.String())
		return domain.Fingerprint{}, fmt.Errorf("adapters/pipeline/music: running ffprobe on %s: %w", path, err)
	}

	var out ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return domain.Fingerprint{}, fmt.Errorf("adapters/pipeline/music: parsing ffprobe output for %s: %w", path, err)
	}

	idx := mergedTagIndex(out)

	tags := make(map[string]string, len(canonicalTagAliases))
	for canonical, aliases := range canonicalTagAliases {
		if v, ok := findTag(idx, append([]string{canonical}, aliases...)...); ok && v != "" {
			tags[canonical] = v
		}
	}

	discNumber := discNumberGuess
	if v, ok := findTag(idx, discNumberAliases...); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			discNumber = parsed
		}
	}
	trackNumber, _ := findTag(idx, trackNumberAliases...)

	metadata := map[string]any{
		"disc_number":  discNumber,
		"track_number": trackNumber,
	}
	if out.Format.Duration != "" {
		if seconds, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
			metadata["duration_seconds"] = seconds
		}
	}

	f.logger.DebugContext(ctx, "fingerprinted file", "path", path, "tag_count", len(tags))
	return domain.Fingerprint{Tags: tags, Metadata: metadata}, nil
}

// mergedTagIndex merges out.Format.Tags and every stream's Tags (format
// taking priority on collision) into a single case-insensitive lookup index
// keyed by lowercased tag name.
func mergedTagIndex(out ffprobeOutput) map[string]string {
	merged := make(map[string]string, len(out.Format.Tags))
	for k, v := range out.Format.Tags {
		merged[k] = v
	}
	for _, s := range out.Streams {
		for k, v := range s.Tags {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}
	idx := make(map[string]string, len(merged))
	for k, v := range merged {
		idx[strings.ToLower(k)] = v
	}
	return idx
}

// findTag returns the first non-empty match for any of names (checked in
// order, case-insensitively) against idx.
func findTag(idx map[string]string, names ...string) (string, bool) {
	for _, name := range names {
		if v, ok := idx[strings.ToLower(name)]; ok {
			return v, true
		}
	}
	return "", false
}

// Consensus implements ports.FileFingerprinter: reduces every file's
// Fingerprint in one identification group into the single Fingerprint
// written onto every domain.UnmatchedFile row in that group, per
// docs/technical/pipeline-music-fingerprinter.md's "Group consensus"
// section. Pure in-memory reduction — no I/O, so unlike Fingerprint above
// it carries no tracing (same treatment Grouping.GroupKeys gets).
func (f *FileFingerprinter) Consensus(_ context.Context, fingerprints []domain.Fingerprint) (domain.Fingerprint, error) {
	tags := majorityVoteTags(fingerprints)

	entries := make([]trackEntry, 0, len(fingerprints))
	discCount := 0
	mbidSet := make(map[string]struct{})
	for _, fp := range fingerprints {
		disc, _ := fp.Metadata["disc_number"].(int)
		if disc > discCount {
			discCount = disc
		}
		track, _ := fp.Metadata["track_number"].(string)
		duration, _ := fp.Metadata["duration_seconds"].(float64)
		entries = append(entries, trackEntry{
			disc:     disc,
			track:    track,
			title:    fp.Tags["TITLE"],
			duration: duration,
			isrc:     fp.Tags["ISRC"],
		})
		if mbid := fp.Tags["MUSICBRAINZ_ALBUMID"]; mbid != "" {
			mbidSet[mbid] = struct{}{}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].disc != entries[j].disc {
			return entries[i].disc < entries[j].disc
		}
		return trackNumberLess(entries[i].track, entries[j].track)
	})

	titles := make([]string, len(entries))
	durations := make([]float64, len(entries))
	isrcs := make([]string, len(entries))
	for i, e := range entries {
		titles[i] = e.title
		durations[i] = e.duration
		isrcs[i] = e.isrc
	}

	metadata := map[string]any{
		"track_count":     len(fingerprints),
		"disc_count":      discCount,
		"track_titles":    titles,
		"track_durations": durations,
		"track_isrcs":     isrcs,
	}
	if len(mbidSet) > 0 {
		mbids := make([]string, 0, len(mbidSet))
		for mbid := range mbidSet {
			mbids = append(mbids, mbid)
		}
		sort.Strings(mbids)
		metadata["embedded_release_mbids"] = mbids
	}

	return domain.Fingerprint{Tags: tags, Metadata: metadata}, nil
}

// trackEntry is one file's contribution to Consensus' ordered per-track
// lists, keyed by (disc, track) — the same pair shape MusicBrainz's own
// medium.position/track.number fields use.
type trackEntry struct {
	disc     int
	track    string
	title    string
	duration float64
	isrc     string
}

// trackNumberLess orders two raw TRACKNUMBER strings: numerically when both
// parse as plain integers (the common CD case, correct past single digits),
// falling back to lexicographic comparison otherwise — which orders vinyl
// side-lettered values ("A1" < "A2" < "B1") correctly for the track counts
// a single vinyl side actually has. TRACKNUMBER stays a string throughout;
// this comparator never mutates or persists a parsed value.
func trackNumberLess(a, b string) bool {
	ai, aErr := strconv.Atoi(a)
	bi, bErr := strconv.Atoi(b)
	if aErr == nil && bErr == nil {
		return ai < bi
	}
	return a < b
}

// majorityVoteTags picks, for every tag key seen across fingerprints' Tags,
// the most common non-empty value — a typo'd outlier in one file's tag
// doesn't win over the rest of the group agreeing. Ties break toward
// whichever value was seen first across the group, for deterministic
// output.
func majorityVoteTags(fingerprints []domain.Fingerprint) map[string]string {
	counts := make(map[string]map[string]int)
	firstSeenOrder := make(map[string][]string)
	for _, fp := range fingerprints {
		for k, v := range fp.Tags {
			if v == "" {
				continue
			}
			if counts[k] == nil {
				counts[k] = make(map[string]int)
			}
			if counts[k][v] == 0 {
				firstSeenOrder[k] = append(firstSeenOrder[k], v)
			}
			counts[k][v]++
		}
	}

	result := make(map[string]string, len(counts))
	for k, valueCounts := range counts {
		best := ""
		bestCount := 0
		for _, v := range firstSeenOrder[k] {
			if valueCounts[v] > bestCount {
				best = v
				bestCount = valueCounts[v]
			}
		}
		result[k] = best
	}
	return result
}

// Option customizes a FileFingerprinter constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
	}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithTracerProvider overrides the default (global) TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}
