// Package image is the in-memory adapter for the ports.ImageRepository
// port. See internal/adapters/memory/person for the convention this
// follows.
package image

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/memory/image"

// Repository is the in-memory ports.ImageRepository adapter. Safe for
// concurrent use.
type Repository struct {
	name string

	mu   sync.RWMutex
	byID map[string]*domain.Image

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

// New constructs a named in-memory Repository.
func New(name string, opts ...Option) (*Repository, error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/memory/image: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byID:   make(map[string]*domain.Image),
		logger: o.logger.With("component", "adapters.memory.image", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("image_repository.creates", metric.WithDescription("Image records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("image_repository.gets", metric.WithDescription("Image Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("image_repository.updates", metric.WithDescription("Image records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("image_repository.deletes", metric.WithDescription("Image records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("image_repository.lists", metric.WithDescription("Image List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("image_repository.items",
		metric.WithDescription("current number of Image records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byID)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/image: creating items gauge: %w", err)
	}

	r.logger.Info("image repository created")
	return r, nil
}

// Create implements ports.ImageRepository.
func (r *Repository) Create(ctx context.Context, img *domain.Image) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", img.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[img.ID]; exists {
		return ports.ErrConflict
	}

	stored := *img
	r.byID[img.ID] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image created", "image.id", img.ID)
	return nil
}

// Get implements ports.ImageRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Image, error) {
	ctx, span := r.tracer.Start(ctx, "image_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", id)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	img, ok := r.byID[id]
	if !ok {
		span.SetAttributes(attribute.Bool("image.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *img
	return &stored, nil
}

// Update implements ports.ImageRepository.
func (r *Repository) Update(ctx context.Context, img *domain.Image) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", img.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[img.ID]; !ok {
		return ports.ErrNotFound
	}

	stored := *img
	r.byID[img.ID] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image updated", "image.id", img.ID)
	return nil
}

// Delete implements ports.ImageRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", id)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byID, id)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image deleted", "image.id", id)
	return nil
}

// List implements ports.ImageRepository. ownerType and ownerID are
// independent, optional filters. See internal/adapters/memory/person for
// the pagination convention.
func (r *Repository) List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	ctx, span := r.tracer.Start(ctx, "image_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	r.mu.RLock()
	ids := make([]string, 0, len(r.byID))
	for id, img := range r.byID {
		if ownerType != "" && img.OwnerType != ownerType {
			continue
		}
		if ownerID != "" && img.OwnerID != ownerID {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	start := 0
	if pageToken != "" {
		start = sort.SearchStrings(ids, pageToken)
		if start < len(ids) && ids[start] == pageToken {
			start++
		}
	}

	end := min(start+pageSize, len(ids))

	images := make([]*domain.Image, 0, end-start)
	for _, id := range ids[start:end] {
		stored := *r.byID[id]
		images = append(images, &stored)
	}
	r.mu.RUnlock()

	var nextToken string
	if end < len(ids) {
		nextToken = ids[end-1]
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image list", "count", len(images), "next_page_token", nextToken)
	return images, nextToken, nil
}

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
