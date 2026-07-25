// Package local is the local-filesystem adapter for the ports.FileWalker
// port. See docs/adr/0024-pipeline-core.md.
package local

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"purser/internal/ports"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/filewalker/local"

// Walker is the local-filesystem ports.FileWalker adapter. Unlike
// imagestore/local's Store, it is stateless with respect to which
// directory it walks — root is a per-call Walk argument, not constructor
// state, since a scan can target any root.
type Walker struct {
	logger *slog.Logger
	tracer trace.Tracer

	filesWalked metric.Int64Counter
}

var _ ports.FileWalker = (*Walker)(nil)

// New constructs a Walker.
func New(opts ...Option) (*Walker, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	w := &Walker{
		logger: o.logger.With("component", "adapters.filewalker.local"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if w.filesWalked, err = meter.Int64Counter("filewalker_local.files_walked", metric.WithDescription("Regular files discovered by Walk")); err != nil {
		return nil, fmt.Errorf("adapters/filewalker/local: creating files_walked counter: %w", err)
	}

	return w, nil
}

// Walk implements ports.FileWalker.
func (w *Walker) Walk(ctx context.Context, root string) ([]ports.DiscoveredFile, error) {
	ctx, span := w.tracer.Start(ctx, "filewalker_local.walk", trace.WithAttributes(attribute.String("root", root)))
	defer span.End()

	var files []ports.DiscoveredFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("adapters/filewalker/local: stat %s: %w", p, err)
		}
		files = append(files, ports.DiscoveredFile{Path: p, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("adapters/filewalker/local: walking %s: %w", root, err)
	}

	w.filesWalked.Add(ctx, int64(len(files)))
	w.logger.DebugContext(ctx, "walk complete", "root", root, "files", len(files))
	return files, nil
}

// Option customizes a Walker constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
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

// WithMeterProvider overrides the default (global) MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.meterProvider = mp }
}
