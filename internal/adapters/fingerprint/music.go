package fingerprint

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

var musicContentTypes = []domain.ContentType{domain.ContentTypeMusic}

type musicFingerprinter struct {
	fpcalcAvail bool
}

var _ ports.FileFingerprinter = (*musicFingerprinter)(nil)

// NewMusicFingerprinter returns a FileFingerprinter for music content.
// It checks for fpcalc availability at construction; if absent, AcoustID is skipped for all files.
func NewMusicFingerprinter() ports.FileFingerprinter {
	_, err := exec.LookPath("fpcalc")
	avail := err == nil
	if !avail {
		slog.Warn("fpcalc not found, acoustid disabled")
	}
	return &musicFingerprinter{fpcalcAvail: avail}
}

func (m *musicFingerprinter) ContentTypes() []domain.ContentType {
	return musicContentTypes
}

func (m *musicFingerprinter) Fingerprint(ctx context.Context, f domain.ScannedFile) (*domain.Fingerprint, error) {
	if !slices.Contains(m.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := &domain.Fingerprint{EmbeddedTags: make(map[string]string)}

	if err := m.readTags(f.Path, fp); err != nil {
		slog.WarnContext(ctx, "tag read skipped", "path", f.Path, "err", err)
	}

	if m.fpcalcAvail {
		if err := m.populateFromFpcalc(ctx, f.Path, fp); err != nil {
			slog.WarnContext(ctx, "acoustid skipped", "path", f.Path, "err", err)
		}
	}

	slog.DebugContext(ctx, "music fingerprint",
		"path", f.Path,
		"barcode", tagOrNotSet(fp.EmbeddedTags, "barcode"),
		"isrc", tagOrNotSet(fp.EmbeddedTags, "isrc"),
		"mbz_album_id", tagOrNotSet(fp.EmbeddedTags, "musicbrainz_album_id"),
		"tag_count", len(fp.EmbeddedTags),
	)

	return fp, nil
}

func tagOrNotSet(tags map[string]string, key string) string {
	if v := tags[key]; v != "" {
		return v
	}
	return "not_set"
}

func (m *musicFingerprinter) readTags(path string, fp *domain.Fingerprint) error {
	f, err := os.Open(path) //nolint:gosec // path comes from the internal scanner, not from user HTTP input
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		return fmt.Errorf("read tags: %w", err)
	}

	setTag := func(k, v string) {
		if v != "" {
			fp.EmbeddedTags[k] = v
		}
	}

	setTag("artist", meta.Artist())
	setTag("album_artist", meta.AlbumArtist())
	setTag("album", meta.Album())
	setTag("title", meta.Title())

	trackNum, trackTotal := meta.Track()
	if trackNum > 0 {
		fp.EmbeddedTags["track_number"] = strconv.Itoa(trackNum)
	}
	if trackTotal > 0 {
		fp.EmbeddedTags["track_total"] = strconv.Itoa(trackTotal)
	}

	discNum, discTotal := meta.Disc()
	if discNum > 0 {
		fp.EmbeddedTags["disc_number"] = strconv.Itoa(discNum)
	}
	if discTotal > 0 {
		fp.EmbeddedTags["disc_total"] = strconv.Itoa(discTotal)
	}

	if y := meta.Year(); y > 0 {
		fp.EmbeddedTags["date"] = strconv.Itoa(y)
	}

	m.applyRawTags(meta.Raw(), fp)
	return nil
}

func (m *musicFingerprinter) applyRawTags(raw map[string]interface{}, fp *domain.Fingerprint) {
	for rawKey, rawVal := range raw {
		switch v := rawVal.(type) {
		case *tag.Comm:
			// ID3v2 TXXX frame: the user-defined description is the semantic key
			if norm, ok := normalizeRawKey(v.Description); ok {
				fp.EmbeddedTags[norm] = v.Text
			}
		case string:
			// VORBISCOMMENT keys are lowercased by dhowden/tag; ID3v2 standard frame IDs are uppercase
			if norm, ok := normalizeRawKey(rawKey); ok {
				fp.EmbeddedTags[norm] = v
			}
			// Duration in milliseconds: TLEN (ID3v2) or length (VORBISCOMMENT, lowercased)
			if rawKey == "TLEN" || rawKey == "length" {
				fp.EmbeddedTags["duration_ms"] = v
			}
		}
	}
}

// rawTagKeyMap maps normalised tag names to canonical EmbeddedTags keys.
// Covers ID3v2 TXXX frame descriptions, ID3v2 standard frame IDs (e.g. TSRC, TPUB),
// and VORBISCOMMENT keys (lowercased by dhowden/tag). Lookup is case-insensitive via normalizeRawKey.
var rawTagKeyMap = map[string]string{
	// MusicBrainz IDs — TXXX descriptions and VORBISCOMMENT keys
	"musicbrainz track id":         "musicbrainz_track_id",
	"musicbrainz album id":         "musicbrainz_album_id",
	"musicbrainz release group id": "musicbrainz_release_group_id",
	"musicbrainz album artist id":  "musicbrainz_album_artist_id",
	"musicbrainz_trackid":          "musicbrainz_track_id",
	"musicbrainz_albumid":          "musicbrainz_album_id",
	"musicbrainz_releasegroupid":   "musicbrainz_release_group_id",
	"musicbrainz_albumartistid":    "musicbrainz_album_artist_id",
	// Barcode — UPC and BARCODE are synonyms
	"barcode": "barcode",
	"upc":     "barcode",
	// Recording identifier
	"isrc": "isrc",
	"tsrc": "isrc", // ID3v2 standard ISRC frame
	// Label / publisher
	"label": "label",
	"tpub":  "label", // ID3v2 publisher frame
	// Catalog number
	"catalognumber":  "catalog_number",
	"catalog_number": "catalog_number",
	// Track / disc totals (VORBISCOMMENT — also handled via meta.Track()/Disc() for ID3v2 N/total format)
	"tracktotal":  "track_total",
	"totaltracks": "track_total",
	"disctotal":   "disc_total",
	"totaldiscs":  "disc_total",
}

func normalizeRawKey(key string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	mapped, ok := rawTagKeyMap[k]
	return mapped, ok
}

// populateFromFpcalc runs fpcalc and writes both the AcoustID fingerprint and
// the duration into fp. fpcalc outputs FILE=, DURATION=, and FINGERPRINT= lines;
// the FINGERPRINT value is the base64url-encoded Chromaprint string that the
// AcoustID /v2/lookup API requires.
func (m *musicFingerprinter) populateFromFpcalc(ctx context.Context, path string, fp *domain.Fingerprint) error {
	out, err := exec.CommandContext(ctx, "fpcalc", path).Output() //nolint:gosec // intentional external tool invocation
	if err != nil {
		return fmt.Errorf("fpcalc: %w", err)
	}
	return ApplyFpcalcOutput(string(out), fp)
}

// ApplyFpcalcOutput parses fpcalc stdout into fp. Package-level so tests
// can verify parsing without invoking fpcalc.
func ApplyFpcalcOutput(output string, fp *domain.Fingerprint) error {
	for _, line := range strings.Split(output, "\n") {
		if after, ok := strings.CutPrefix(line, "FINGERPRINT="); ok {
			fp.AcoustID = strings.TrimSpace(after)
		}
		if after, ok := strings.CutPrefix(line, "DURATION="); ok {
			secs, parseErr := strconv.ParseFloat(strings.TrimSpace(after), 64)
			if parseErr == nil && secs > 0 {
				fp.EmbeddedTags["duration_ms"] = strconv.Itoa(int(secs * 1000))
			}
		}
	}
	if fp.AcoustID == "" {
		return fmt.Errorf("no fingerprint in fpcalc output")
	}
	return nil
}
