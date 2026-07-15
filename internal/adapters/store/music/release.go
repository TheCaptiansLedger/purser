// Package music is the datastore-backed adapter for the
// ports.MusicReleaseRepository port. It is a hand-written translator
// directly against datastore.Datastore — not an instantiation of
// store.Repository[T]/store.FilteredRepository[T] — because later
// sub-issues in the Music Release API epic need independent filtered
// listing (by Group, by LibraryEntry) plus two unique point-lookups (MBID,
// Barcode) neither generic shape covers in one type. This walking-skeleton
// pass implements only Create/Get and writes no secondary index yet — see
// docs/adr/0021-music-domain-model.md and
// docs/adr/0012-datastore-persistence.md.
package music

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/domain/music"
	"purser/internal/ports"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	collection          = "music_release"
	instrumentationName = "purser/internal/adapters/store/music"
)

// Option customizes a Repository constructed via New.
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

// Repository is the datastore-backed ports.MusicReleaseRepository adapter.
type Repository struct {
	name string
	ds   datastore.Datastore

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
}

var _ ports.MusicReleaseRepository = (*Repository)(nil)

// New constructs a named ports.MusicReleaseRepository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/store/music: name must not be empty")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/store/music: ds must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		ds:     ds,
		logger: o.logger.With("component", "adapters.store."+collection, "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store/music: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter(collection+"_repository.gets", metric.WithDescription(collection+" Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/store/music: creating gets counter: %w", err)
	}

	r.logger.Info(collection + " repository created")
	return r, nil
}

// Create implements ports.MusicReleaseRepository. No Document.Index is
// written yet — no filtered/point-lookup query exists to serve until a
// later sub-issue adds one, per docs/adr/0021-music-domain-model.md.
func (r *Repository) Create(ctx context.Context, rel *music.Release) error {
	ctx, span := r.tracer.Start(ctx, collection+"_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("music_release.id", rel.ID)))
	defer span.End()

	data, err := json.Marshal(rel)
	if err != nil {
		return fmt.Errorf("adapters/store/music: marshal music release %s: %w", rel.ID, err)
	}

	if err := r.ds.Create(ctx, datastore.Document{Collection: collection, ID: rel.ID, Data: data}); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "music release created", "music_release.id", rel.ID)
	return nil
}

// Get implements ports.MusicReleaseRepository.
func (r *Repository) Get(ctx context.Context, id string) (*music.Release, error) {
	ctx, span := r.tracer.Start(ctx, collection+"_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("music_release.id", id)))
	defer span.End()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	doc, err := r.ds.Get(ctx, collection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			span.SetAttributes(attribute.Bool("music_release.found", false))
		}
		return nil, err
	}

	var rel music.Release
	if err := json.Unmarshal(doc.Data, &rel); err != nil {
		return nil, fmt.Errorf("adapters/store/music: unmarshal music release %s: %w", id, err)
	}
	return &rel, nil
}
