// Package local is the local-filesystem adapter for the ports.ImageStore
// port. See docs/adr/0013-image-blob-storage.md for the layout,
// atomicity, size-limit, and path-traversal decisions this implements.
package local

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"purser/internal/ports"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/imagestore/local"

// defaultMaxBytes bounds a single Put — sane for one image, prevents an
// unbounded caller-supplied reader from writing without limit.
const defaultMaxBytes = 32 << 20 // 32 MiB

// tempFilePattern names the temp file Put writes to before atomically
// renaming it into place.
const tempFilePattern = ".imagestore-*"

// Store is the local-filesystem ports.ImageStore adapter. Safe for
// concurrent use — writes are atomic (temp file + rename) and reads open
// the file directly, so concurrent Put/Get/Delete calls never observe a
// partially written file.
type Store struct {
	name     string
	root     string
	maxBytes int64

	logger *slog.Logger
	tracer trace.Tracer

	puts    metric.Int64Counter
	gets    metric.Int64Counter
	deletes metric.Int64Counter
}

var _ ports.ImageStore = (*Store)(nil)

// New constructs a named Store rooted at root, creating it if necessary.
func New(name, root string, opts ...Option) (*Store, error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/imagestore/local: name must not be empty")
	}
	if root == "" {
		return nil, fmt.Errorf("adapters/imagestore/local: root must not be empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: resolving root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: creating root: %w", err)
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Store{
		name:     name,
		root:     absRoot,
		maxBytes: o.maxBytes,
		logger:   o.logger.With("component", "adapters.imagestore.local", "imagestore.name", name),
		tracer:   o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	if s.puts, err = meter.Int64Counter("imagestore_local.puts", metric.WithDescription("Images written")); err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: creating puts counter: %w", err)
	}
	if s.gets, err = meter.Int64Counter("imagestore_local.gets", metric.WithDescription("Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: creating gets counter: %w", err)
	}
	if s.deletes, err = meter.Int64Counter("imagestore_local.deletes", metric.WithDescription("Images deleted")); err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: creating deletes counter: %w", err)
	}

	s.logger.Info("local imagestore created", "root", absRoot)
	return s, nil
}

// Put implements ports.ImageStore.
func (s *Store) Put(ctx context.Context, ownerType, id string, r io.Reader) (string, error) {
	ctx, span := s.tracer.Start(ctx, "imagestore_local.put", trace.WithAttributes(
		attribute.String("imagestore.name", s.name),
		attribute.String("owner_type", ownerType),
		attribute.String("id", id),
	))
	defer span.End()

	if ownerType == "" {
		return "", fmt.Errorf("adapters/imagestore/local: ownerType must not be empty")
	}
	if id == "" {
		return "", fmt.Errorf("adapters/imagestore/local: id must not be empty")
	}

	br := bufio.NewReaderSize(r, 512)
	// Peek's error is deliberately ignored: a short read (including EOF
	// on an input smaller than 512 bytes) still yields whatever bytes
	// were available, which is all http.DetectContentType needs — it
	// accepts fewer than 512 bytes and always returns a valid MIME type.
	sniff, _ := br.Peek(512)
	ext := extFromContentType(http.DetectContentType(sniff))

	key := path.Join(ownerType, shard(id), id+ext)
	dest := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", fmt.Errorf("adapters/imagestore/local: creating shard dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), tempFilePattern)
	if err != nil {
		return "", fmt.Errorf("adapters/imagestore/local: creating temp file: %w", err)
	}
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmp.Name())
		}
	}()

	written, err := io.Copy(tmp, io.LimitReader(br, s.maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("adapters/imagestore/local: writing %s: %w", key, err)
	}
	if written > s.maxBytes {
		return "", fmt.Errorf("adapters/imagestore/local: %s exceeds max size of %d bytes", key, s.maxBytes)
	}

	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("adapters/imagestore/local: closing temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return "", fmt.Errorf("adapters/imagestore/local: renaming into place: %w", err)
	}
	committed = true

	// A second Put for the same id with a different content type produces
	// a different extension, and therefore a different filename — without
	// this, the previous file would be silently orphaned on disk instead
	// of overwritten. Best-effort: the new blob is already safely in
	// place, so a cleanup failure here is logged, not returned.
	if err := removeStaleSiblings(filepath.Dir(dest), id, dest); err != nil {
		s.logger.WarnContext(ctx, "failed to remove stale sibling blob", "key", key, "error", err)
	}

	s.puts.Add(ctx, 1, metric.WithAttributes(attribute.String("imagestore.name", s.name)))
	s.logger.DebugContext(ctx, "image put", "key", key, "bytes", written)
	return key, nil
}

// Get implements ports.ImageStore.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	_, span := s.tracer.Start(ctx, "imagestore_local.get", trace.WithAttributes(
		attribute.String("imagestore.name", s.name),
		attribute.String("key", key),
	))
	defer span.End()

	dest, err := s.resolve(key)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(dest) //nolint:gosec // dest is validated by resolve() to stay under s.root
	if errors.Is(err, os.ErrNotExist) {
		span.SetAttributes(attribute.Bool("imagestore.found", false))
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("adapters/imagestore/local: opening %s: %w", key, err)
	}

	s.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("imagestore.name", s.name)))
	return f, nil
}

// Delete implements ports.ImageStore.
func (s *Store) Delete(ctx context.Context, key string) error {
	ctx, span := s.tracer.Start(ctx, "imagestore_local.delete", trace.WithAttributes(
		attribute.String("imagestore.name", s.name),
		attribute.String("key", key),
	))
	defer span.End()

	dest, err := s.resolve(key)
	if err != nil {
		return err
	}

	if err := os.Remove(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ports.ErrNotFound
		}
		return fmt.Errorf("adapters/imagestore/local: removing %s: %w", key, err)
	}

	s.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("imagestore.name", s.name)))
	s.logger.DebugContext(ctx, "image deleted", "key", key)
	return nil
}

// resolve turns key into an absolute path guaranteed to stay under
// s.root, rejecting anything that would escape it — see
// docs/adr/0013-image-blob-storage.md's path-traversal defense. Put
// never generates a key that fails this, but key round-trips through
// domain.Image.URL (a plain string) once a caller starts persisting it,
// so this is defense in depth against a malformed/tampered value, not a
// defense against Put itself.
func (s *Store) resolve(key string) (string, error) {
	if key == "" {
		return "", ports.ErrNotFound
	}
	dest := filepath.Clean(filepath.Join(s.root, filepath.FromSlash(key)))
	if dest != s.root && !strings.HasPrefix(dest, s.root+string(filepath.Separator)) {
		return "", fmt.Errorf("adapters/imagestore/local: key %q resolves outside root", key)
	}
	return dest, nil
}

// removeStaleSiblings deletes every file in shardDir matching id.* other
// than keep — cleaning up a prior Put's file when a later Put for the
// same id sniffed a different content type (and therefore extension).
func removeStaleSiblings(shardDir, id, keep string) error {
	matches, err := filepath.Glob(filepath.Join(shardDir, id+".*"))
	if err != nil {
		return fmt.Errorf("adapters/imagestore/local: globbing siblings of %s: %w", id, err)
	}
	for _, m := range matches {
		if m == keep {
			continue
		}
		if err := os.Remove(m); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("adapters/imagestore/local: removing stale sibling %s: %w", m, err)
		}
	}
	return nil
}

func shard(id string) string {
	if len(id) >= 2 {
		return id[:2]
	}
	return id
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "svg"):
		return ".svg"
	default:
		return ".jpg"
	}
}

// Option customizes a Store constructed via New.
type Option func(*options)

type options struct {
	maxBytes       int64
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

func defaultOptions() *options {
	return &options{
		maxBytes:       defaultMaxBytes,
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
	}
}

// WithMaxBytes overrides the default (32 MiB) per-image size cap.
func WithMaxBytes(n int64) Option {
	return func(o *options) { o.maxBytes = n }
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithTracerProvider overrides the default (global) TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}

// WithMeterProvider overrides the default (global) MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.meterProvider = mp }
}
