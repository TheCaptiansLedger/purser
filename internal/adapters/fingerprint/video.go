package fingerprint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"log/slog"
	"os/exec"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strconv"
	"strings"

	"github.com/corona10/goimagehash"
)

var videoContentTypes = []domain.ContentType{
	domain.ContentTypeMovie,
	domain.ContentTypeTV,
	domain.ContentTypeAdult,
	domain.ContentTypeJAV,
}

type videoFingerprinter struct {
	fs         ports.FileSystem
	pHashAvail bool
}

var _ ports.FileFingerprinter = (*videoFingerprinter)(nil)

// NewVideoFingerprinter returns a FileFingerprinter for movie, tv, adult, and jav content.
// It checks for ffprobe availability at construction; if absent, PHash is skipped for all files.
func NewVideoFingerprinter(fs ports.FileSystem) ports.FileFingerprinter {
	_, err := exec.LookPath("ffprobe")
	avail := err == nil
	if !avail {
		slog.Warn("ffprobe not found; PHash computation will be skipped for video files")
	}
	return &videoFingerprinter{fs: fs, pHashAvail: avail}
}

func (v *videoFingerprinter) ContentTypes() []domain.ContentType {
	return videoContentTypes
}

func (v *videoFingerprinter) Fingerprint(ctx context.Context, f domain.ScannedFile) (*domain.Fingerprint, error) {
	if !slices.Contains(v.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	hash, err := v.fs.OSHash(ctx, f.Path)
	if err != nil {
		return nil, fmt.Errorf("oshash %s: %w", f.Path, err)
	}

	fp := &domain.Fingerprint{OSHash: hash}

	if v.pHashAvail {
		phash, err := v.computePHash(ctx, f.Path)
		if err != nil {
			slog.WarnContext(ctx, "phash skipped", "path", f.Path, "err", err)
		} else {
			fp.PHash = phash
		}
	}

	return fp, nil
}

// computePHash samples frames at 10%, 25%, and 50% of the video duration and
// returns a 48-char hex string (three concatenated 16-char dHash values).
func (v *videoFingerprinter) computePHash(ctx context.Context, path string) (string, error) {
	dur, err := v.probeDuration(ctx, path)
	if err != nil {
		return "", fmt.Errorf("probe duration: %w", err)
	}
	if dur <= 0 {
		return "", fmt.Errorf("invalid duration %.3f", dur)
	}

	var sb strings.Builder
	for _, frac := range []float64{0.10, 0.25, 0.50} {
		h, err := v.frameHash(ctx, path, dur*frac)
		if err != nil {
			return "", fmt.Errorf("frame at %.0f%%: %w", frac*100, err)
		}
		fmt.Fprintf(&sb, "%016x", h)
	}
	return sb.String(), nil
}

func (v *videoFingerprinter) probeDuration(ctx context.Context, path string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", //nolint:gosec // intentional external tool invocation
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, fmt.Errorf("parse ffprobe output: %w", err)
	}
	return strconv.ParseFloat(result.Format.Duration, 64)
}

// frameHash extracts one PNG frame via ffmpeg at offsetSec and returns its 64-bit dHash.
func (v *videoFingerprinter) frameHash(ctx context.Context, path string, offsetSec float64) (uint64, error) {
	out, err := exec.CommandContext(ctx, "ffmpeg", //nolint:gosec // intentional external tool invocation
		"-ss", fmt.Sprintf("%.3f", offsetSec),
		"-i", path,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"pipe:1",
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return 0, fmt.Errorf("decode frame: %w", err)
	}
	h, err := goimagehash.DifferenceHash(img)
	if err != nil {
		return 0, fmt.Errorf("dhash: %w", err)
	}
	return h.GetHash(), nil
}
