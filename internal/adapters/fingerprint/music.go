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
		slog.Warn("fpcalc not found; AcoustID computation will be skipped for music files")
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
		acoustID, err := m.computeAcoustID(ctx, f.Path)
		if err != nil {
			slog.WarnContext(ctx, "acoustid skipped", "path", f.Path, "err", err)
		} else {
			fp.AcoustID = acoustID
		}
	}

	return fp, nil
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

	if n, _ := meta.Track(); n > 0 {
		fp.EmbeddedTags["track_number"] = strconv.Itoa(n)
	}
	if n, _ := meta.Disc(); n > 0 {
		fp.EmbeddedTags["disc_number"] = strconv.Itoa(n)
	}
	if y := meta.Year(); y > 0 {
		fp.EmbeddedTags["date"] = strconv.Itoa(y)
	}

	for rawKey, rawVal := range meta.Raw() {
		switch v := rawVal.(type) {
		case *tag.Comm:
			// ID3v2 TXXX frame: the user-defined description is the semantic key
			if norm, ok := normalizeMBZKey(v.Description); ok {
				fp.EmbeddedTags[norm] = v.Text
			}
		case string:
			// VORBISCOMMENT keys are lowercased by dhowden/tag; ID3v2 standard frame IDs are uppercase
			if norm, ok := normalizeMBZKey(rawKey); ok {
				fp.EmbeddedTags[norm] = v
			}
			// Duration in milliseconds: TLEN (ID3v2) or length (VORBISCOMMENT, lowercased)
			if rawKey == "TLEN" || rawKey == "length" {
				fp.EmbeddedTags["duration_ms"] = v
			}
		}
	}

	return nil
}

// mbzKeyMap maps normalised MusicBrainz tag names to canonical EmbeddedTags keys.
// Covers both ID3v2 TXXX descriptions and VORBISCOMMENT keys (lowercased by dhowden/tag).
var mbzKeyMap = map[string]string{
	"musicbrainz track id":        "musicbrainz_track_id",
	"musicbrainz album id":        "musicbrainz_album_id",
	"musicbrainz album artist id": "musicbrainz_artist_id",
	"musicbrainz_trackid":         "musicbrainz_track_id",
	"musicbrainz_albumid":         "musicbrainz_album_id",
	"musicbrainz_artistid":        "musicbrainz_artist_id",
}

func normalizeMBZKey(key string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	mapped, ok := mbzKeyMap[k]
	return mapped, ok
}

func (m *musicFingerprinter) computeAcoustID(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, "fpcalc", "-raw", path).Output() //nolint:gosec // intentional external tool invocation
	if err != nil {
		return "", fmt.Errorf("fpcalc: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if after, ok := strings.CutPrefix(line, "FINGERPRINT="); ok {
			return strings.TrimSpace(after), nil
		}
	}
	return "", fmt.Errorf("no fingerprint in fpcalc output")
}
