// Package videohash computes a perceptual video hash ("PHash") close enough
// to Stash's own algorithm (stashapp/stash's pkg/hash/videophash) to be
// lookup-compatible against StashDB/ThePornDB's crowd-sourced fingerprint
// indexes — see docs/technical/afterdark-data_model.md §5.2. It samples 25
// frames evenly spaced across the middle 90% of a video, tiles them into one
// montage image, and runs a DCT-based perceptual hash over the montage.
//
// This package deliberately depends on the same third-party hash library
// Stash itself uses (github.com/corona10/goimagehash) rather than
// reimplementing the DCT hash by hand — a hand-rolled reimplementation could
// silently drift from Stash's exact bit layout, defeating the entire point
// of "lookup-compatible." github.com/disintegration/imaging assembles the
// frame montage, matching Stash's own choice there too.
//
// Kept generic (pkg/, not internal/adapters/pipeline/afterdark/) per
// docs/adr/0024-pipeline-core.md — perceptual video hashing isn't
// AfterDark-specific, any future content type doing scene/video
// identification can reuse it, same reasoning as pkg/filehash. Like
// pkg/filehash, this package is a pure algorithm library: no OpenTelemetry
// tracing or slog logging here, per docs/adr/0007-telemetry.md /
// docs/adr/0008-structured-logging.md — that instrumentation belongs at the
// adapter layer that wires this into the pipeline (the content-type's
// FileFingerprinter), the same split internal/adapters/pipeline/music's
// FileFingerprinter already establishes around pkg/filehash's OSHash.
package videohash

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os/exec"
	"strconv"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/disintegration/imaging"

	// Registers BMP decoding with image.Decode — ffmpeg's screenshot output
	// below is raw BMP and the standard library doesn't register a BMP
	// decoder on its own.
	_ "golang.org/x/image/bmp"
)

// ffmpegBinary is the external binary PHash shells out to, resolved from
// PATH — same assumption internal/adapters/pipeline/music's ffprobe usage
// and cmd/purser's startup preflight check make for ffprobe.
const ffmpegBinary = "ffmpeg"

const (
	// screenshotWidth is the width (in pixels) each sampled frame is scaled
	// to before hashing; height is derived to preserve aspect ratio.
	// Matches Stash's own sprite screenshot size.
	screenshotWidth = 160
	// gridColumns/gridRows lay out the sampled frames into one montage
	// image — 25 frames total, matching Stash's own sprite grid.
	gridColumns = 5
	gridRows    = 5
	frameCount  = gridColumns * gridRows

	// edgeTrimFraction is how much of the video's start and end is skipped
	// before sampling begins, to avoid intro/outro sequences skewing the
	// hash away from the scene's actual content.
	edgeTrimFraction = 0.05
	// sampleSpanFraction is the fraction of total duration the frameCount
	// samples are spread evenly across, starting after edgeTrimFraction.
	sampleSpanFraction = 1 - 2*edgeTrimFraction
)

// ErrInvalidDuration is returned by PHash when duration is zero or negative
// — sampling frames requires a known, positive video duration.
var ErrInvalidDuration = errors.New("videohash: duration must be positive")

// PHash computes the Stash-compatible perceptual video hash of the video at
// path. duration is the video's total playback duration — callers that
// already extract it (e.g. via ffprobe, as AfterDark's FileFingerprinter
// does) pass it in directly rather than this package re-probing it itself,
// keeping PHash's only external dependency ffmpeg.
//
// Frame extraction defaults to ffmpeg's fast (keyframe) seek; if any frame
// fails to extract, PHash falls back to slow/accurate seeking for that and
// every remaining frame in this call — a sticky fallback, matching Stash's
// own behavior for containers whose fast seek is inaccurate or unsupported.
func PHash(ctx context.Context, path string, duration time.Duration) (uint64, error) {
	if duration <= 0 {
		return 0, ErrInvalidDuration
	}

	offsets := frameOffsets(duration)
	frames := make([]image.Image, 0, len(offsets))
	slowSeek := false
	for _, offset := range offsets {
		img, err := extractFrame(ctx, path, offset, slowSeek)
		if err != nil && !slowSeek {
			slowSeek = true
			img, err = extractFrame(ctx, path, offset, slowSeek)
		}
		if err != nil {
			return 0, fmt.Errorf("videohash: extracting frame at %.3fs from %s: %w", offset, path, err)
		}
		frames = append(frames, img)
	}

	hash, err := combineAndHash(frames)
	if err != nil {
		return 0, fmt.Errorf("videohash: hashing %s: %w", path, err)
	}
	return hash, nil
}

// Distance returns the Hamming distance between two PHash values — the
// number of differing bits, lower meaning more visually similar. Downstream
// fuzzy-matching (e.g. AfterDark's confidence scoring against StashDB/
// ThePornDB fingerprint candidates) compares hashes this way rather than
// requiring an exact match.
func Distance(a, b uint64) int {
	diff := a ^ b
	count := 0
	for diff != 0 {
		count++
		diff &= diff - 1
	}
	return count
}

// String formats a PHash value as 16 lowercase hex characters — the same
// string form StashDB/ThePornDB report their PHASH fingerprints in.
func String(h uint64) string {
	return fmt.Sprintf("%016x", h)
}

// frameOffsets returns the frameCount evenly spaced sample times (in
// seconds from the start of the video) PHash extracts frames at, trimming
// edgeTrimFraction off each end of duration so intro/outro sequences don't
// skew the hash. Pure function — no I/O, testable without ffmpeg.
func frameOffsets(duration time.Duration) []float64 {
	total := duration.Seconds()
	start := edgeTrimFraction * total
	step := (sampleSpanFraction * total) / frameCount

	offsets := make([]float64, frameCount)
	for i := range offsets {
		offsets[i] = start + float64(i)*step
	}
	return offsets
}

// extractFrame runs ffmpeg to decode a single frame from path at
// offsetSeconds, scaled to screenshotWidth, and returns it decoded as an
// image.Image.
func extractFrame(ctx context.Context, path string, offsetSeconds float64, slowSeek bool) (image.Image, error) {
	cmd := exec.CommandContext(ctx, ffmpegBinary, ffmpegArgs(path, offsetSeconds, slowSeek)...) //nolint:gosec // path is caller-supplied by design; hashing arbitrary discovered files is this package's purpose
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running ffmpeg: %w: %s", err, stderr.String())
	}

	img, _, err := image.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("decoding ffmpeg output: %w", err)
	}
	return img, nil
}

// ffmpegArgs builds the ffmpeg argument list for extracting one frame at
// offsetSeconds from path, scaled to screenshotWidth and written as BMP to
// stdout. slowSeek places -ss after -i (accurate but slower); the default
// places it before -i (fast, keyframe-based seek) — same two modes and
// argument placement Stash's own screenshot generation uses. Pure function
// — no I/O, testable without ffmpeg.
func ffmpegArgs(path string, offsetSeconds float64, slowSeek bool) []string {
	t := strconv.FormatFloat(offsetSeconds, 'f', 6, 64)

	args := []string{"-loglevel", "error", "-y"}
	if !slowSeek {
		args = append(args, "-ss", t)
	}
	args = append(args, "-i", path)
	if slowSeek {
		args = append(args, "-ss", t)
	}
	args = append(args,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-2", screenshotWidth),
		"-c:v", "bmp",
		"-f", "rawvideo",
		"-",
	)
	return args
}

// combineAndHash tiles frames into one gridColumns x gridRows montage (row-
// major, in the order frames was built — the same order frameOffsets
// produces) and returns the montage's perceptual hash. Assumes every frame
// shares the same dimensions, true by construction since every frame in one
// PHash call is scaled to the same screenshotWidth from the same source
// video.
func combineAndHash(frames []image.Image) (uint64, error) {
	if len(frames) == 0 {
		return 0, errors.New("no frames to hash")
	}

	width := frames[0].Bounds().Dx()
	height := frames[0].Bounds().Dy()
	montage := imaging.New(width*gridColumns, height*gridRows, color.NRGBA{})
	for i, frame := range frames {
		x := width * (i % gridColumns)
		y := height * (i / gridColumns)
		montage = imaging.Paste(montage, frame, image.Pt(x, y))
	}

	hash, err := goimagehash.PerceptionHash(montage)
	if err != nil {
		return 0, fmt.Errorf("computing perception hash: %w", err)
	}
	return hash.GetHash(), nil
}
