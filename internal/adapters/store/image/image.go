// Package image is the datastore-backed adapter for the
// ports.ImageRepository port. Image doesn't fit either shared generic in
// internal/adapters/store: Get/Delete key on its own ID, but List filters
// independently on OwnerType/OwnerID — two fields that aren't part of the
// key. Building a generic for exactly this one occurrence would be
// premature abstraction, so this is a small, hand-written translator
// against datastore.Datastore directly — see
// docs/adr/0012-datastore-persistence.md.
package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/domain"
	"purser/internal/ports"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "purser/internal/adapters/store/image"
	collection          = "image"
)

// Repository is the datastore-backed ports.ImageRepository adapter.
// Concurrency safety is the injected datastore.Datastore's responsibility,
// not this translation layer's.
type Repository struct {
	name string
	ds   datastore.Datastore

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

var _ ports.ImageRepository = (*Repository)(nil)

// New constructs a named Repository backed by ds.
func New(name string, ds datastore.Datastore, opts ...Option) (*Repository, error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/store/image: name must not be empty")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/store/image: ds must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		ds:     ds,
		logger: o.logger.With("component", "adapters.store.image", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if r.creates, err = meter.Int64Counter("image_repository.creates", metric.WithDescription("Image records created")); err != nil {
		return nil, fmt.Errorf("adapters/store/image: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("image_repository.gets", metric.WithDescription("Image Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/store/image: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("image_repository.updates", metric.WithDescription("Image records updated")); err != nil {
		return nil, fmt.Errorf("adapters/store/image: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("image_repository.deletes", metric.WithDescription("Image records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/store/image: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("image_repository.lists", metric.WithDescription("Image List calls")); err != nil {
		return nil, fmt.Errorf("adapters/store/image: creating lists counter: %w", err)
	}

	r.logger.Info("image repository created")
	return r, nil
}

// Create implements ports.ImageRepository.
func (r *Repository) Create(ctx context.Context, img *domain.Image) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", img.ID)))
	defer span.End()

	data, err := json.Marshal(img)
	if err != nil {
		return fmt.Errorf("adapters/store/image: marshal %s: %w", img.ID, err)
	}

	doc := datastore.Document{Collection: collection, ID: img.ID, Data: data, Index: indexOf(img)}
	if err := r.ds.Create(ctx, doc); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image created", "image.id", img.ID)
	return nil
}

// Get implements ports.ImageRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Image, error) {
	ctx, span := r.tracer.Start(ctx, "image_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", id)))
	defer span.End()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	doc, err := r.ds.Get(ctx, collection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			span.SetAttributes(attribute.Bool("image.found", false))
		}
		return nil, err
	}

	var img domain.Image
	if err := json.Unmarshal(doc.Data, &img); err != nil {
		return nil, fmt.Errorf("adapters/store/image: unmarshal %s: %w", id, err)
	}
	return &img, nil
}

// Update implements ports.ImageRepository.
func (r *Repository) Update(ctx context.Context, img *domain.Image) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", img.ID)))
	defer span.End()

	data, err := json.Marshal(img)
	if err != nil {
		return fmt.Errorf("adapters/store/image: marshal %s: %w", img.ID, err)
	}

	doc := datastore.Document{Collection: collection, ID: img.ID, Data: data, Index: indexOf(img)}
	if err := r.ds.Update(ctx, doc); err != nil {
		return err
	}

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image updated", "image.id", img.ID)
	return nil
}

// Delete implements ports.ImageRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "image_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("image.id", id)))
	defer span.End()

	if err := r.ds.Delete(ctx, collection, id); err != nil {
		return err
	}

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image deleted", "image.id", id)
	return nil
}

// List implements ports.ImageRepository. ownerType and ownerID are
// independent, optional filters — an empty string means "no filter on
// this field."
func (r *Repository) List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	ctx, span := r.tracer.Start(ctx, "image_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	filter := map[string]string{}
	if ownerType != "" {
		filter["owner_type"] = ownerType
	}
	if ownerID != "" {
		filter["owner_id"] = ownerID
	}
	if len(filter) == 0 {
		filter = nil
	}

	docs, nextToken, err := r.ds.List(ctx, collection, filter, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}

	images := make([]*domain.Image, 0, len(docs))
	for _, doc := range docs {
		var img domain.Image
		if err := json.Unmarshal(doc.Data, &img); err != nil {
			return nil, "", fmt.Errorf("adapters/store/image: unmarshal %s: %w", doc.ID, err)
		}
		images = append(images, &img)
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "image list", "count", len(images), "next_page_token", nextToken)
	return images, nextToken, nil
}

func indexOf(img *domain.Image) map[string]string {
	return map[string]string{"owner_type": img.OwnerType, "owner_id": img.OwnerID}
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
