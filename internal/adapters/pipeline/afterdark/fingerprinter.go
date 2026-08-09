// Package afterdark implements AfterDark's content-type-scoped pipeline
// capabilities — see docs/adr/0024-pipeline-core.md,
// docs/technical/afterdark-data_model.md §5.2. This file is the
// ports.FileFingerprinter implementation for domain.ContentTypeAdult:
// single-pass OSHash + PHash + basic ffprobe metadata per file, mirroring
// Music's internal/adapters/pipeline/music FileFingerprinter shape. Unlike
// Music, there is no group-consensus pass — AfterDark reuses
// IdentityGrouping as-is (a group is always exactly one file), so
// Consensus below is a passthrough, not a reduction.
package afterdark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/filehash"
	"purser/pkg/videohash"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/pipeline/afterdark"

// ffprobeBinary is the external binary Fingerprint shells out to for
// duration/codec — resolved from PATH, same assumption
// internal/adapters/pipeline/music's ffprobe usage and cmd/purser's startup
// preflight check make.
const ffprobeBinary = "ffprobe"

// ffprobeOutput is the subset of `ffprobe -show_format -show_streams -of
// json` this adapter reads: the container's duration and the first stream's
// codec.
type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

// FileFingerprinter implements ports.FileFingerprinter for
// domain.ContentTypeAdult: per-file OSHash + PHash + basic ffprobe
// metadata. See docs/adr/0024-pipeline-core.md,
// docs/technical/afterdark-data_model.md §5.2.
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
		logger: o.logger.With("component", "adapters.pipeline.afterdark.fingerprinter"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.FileFingerprinter.
func (f *FileFingerprinter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// Fingerprint implements ports.FileFingerprinter: runs ffprobe against path
// for duration/codec, computes pkg/filehash.OSHash and pkg/videohash.PHash,
// and returns them all in the result's Metadata bag — the same shape
// StashDB's fingerprints[]/ThePornDB's hashes[] compare against
// ({hash, algorithm, duration}), per docs/technical/afterdark-data_model.md
// §5.2. discNumberGuess is accepted only to satisfy ports.FileFingerprinter;
// it has no meaning here — AfterDark has no disc concept and reuses
// IdentityGrouping, which never produces a folder-derived guess. Tags is
// always nil: AfterDark has no embedded-tag identification path, unlike
// Music.
//
// A file under pkg/filehash's 64 KiB OSHash minimum
// (filehash.ErrFileTooSmall) is not a hard failure — Metadata["os_hash"] is
// simply omitted, since PHash/duration/codec remain valid identification
// signals on their own.
func (f *FileFingerprinter) Fingerprint(ctx context.Context, path string, _ int) (domain.Fingerprint, error) {
	ctx, span := f.tracer.Start(ctx, "afterdark.fingerprinter.fingerprint", trace.WithAttributes(
		attribute.String("pipeline.path", path),
	))
	defer span.End()

	duration, codec, err := probe(ctx, path)
	if err != nil {
		f.logger.ErrorContext(ctx, "ffprobe failed", "path", path, "error", err)
		return domain.Fingerprint{}, fmt.Errorf("adapters/pipeline/afterdark: probing %s: %w", path, err)
	}

	metadata := map[string]any{
		"duration_seconds": duration.Seconds(),
		"codec":            codec,
	}

	osHash, err := filehash.OSHash(path)
	switch {
	case err == nil:
		metadata["os_hash"] = osHash
	case errors.Is(err, filehash.ErrFileTooSmall):
		f.logger.DebugContext(ctx, "file too small for OSHash, omitting", "path", path)
	default:
		f.logger.ErrorContext(ctx, "OSHash failed", "path", path, "error", err)
		return domain.Fingerprint{}, fmt.Errorf("adapters/pipeline/afterdark: hashing %s: %w", path, err)
	}

	pHash, err := videohash.PHash(ctx, path, duration)
	if err != nil {
		f.logger.ErrorContext(ctx, "PHash failed", "path", path, "error", err)
		return domain.Fingerprint{}, fmt.Errorf("adapters/pipeline/afterdark: perceptual-hashing %s: %w", path, err)
	}
	metadata["phash"] = videohash.String(pHash)

	f.logger.DebugContext(ctx, "fingerprinted file", "path", path)
	return domain.Fingerprint{Metadata: metadata}, nil
}

// probe runs ffprobe against path and returns its duration and first
// stream's codec name.
func probe(ctx context.Context, path string) (time.Duration, string, error) {
	cmd := exec.CommandContext(ctx, ffprobeBinary, "-v", "quiet", "-show_format", "-show_streams", "-of", "json", path) //nolint:gosec // G204: path is caller-supplied by design; fingerprinting arbitrary discovered files is this package's purpose
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, "", fmt.Errorf("running ffprobe: %w: %s", err, stderr.String())
	}

	var out ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return 0, "", fmt.Errorf("parsing ffprobe output: %w", err)
	}

	seconds, err := strconv.ParseFloat(out.Format.Duration, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parsing ffprobe duration %q: %w", out.Format.Duration, err)
	}

	var codec string
	if len(out.Streams) > 0 {
		codec = out.Streams[0].CodecName
	}
	return time.Duration(seconds * float64(time.Second)), codec, nil
}

// Consensus implements ports.FileFingerprinter: a passthrough, not a
// reduction. AfterDark reuses IdentityGrouping as-is (docs/adr/0024-
// pipeline-core.md) — a group is always exactly one file, so there is
// nothing to vote or merge across, unlike Music's majority-vote Consensus.
// Returns fingerprints[0] unchanged when non-empty, an empty
// domain.Fingerprint otherwise. Pure in-memory — no I/O, so like Music's
// Consensus it carries no tracing.
func (f *FileFingerprinter) Consensus(_ context.Context, fingerprints []domain.Fingerprint) (domain.Fingerprint, error) {
	if len(fingerprints) == 0 {
		return domain.Fingerprint{}, nil
	}
	return fingerprints[0], nil
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
