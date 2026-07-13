// Package store implements the shared, generic translation logic every
// single-ID ports.XRepository (Create/Get/Update/Delete/List with no
// filter arguments) uses to sit on top of a datastore.Datastore — see
// docs/adr/0012-datastore-persistence.md's "Repository[T]" addendum.
//
// Repository[T] is deliberately not a ports interface itself: each entity
// (internal/adapters/store/person, .../group, ...) gets its own thin
// wrapper package that pins T, the collection name, and how to read T's
// ID, and returns the result typed as that entity's specific port — the
// service layer only ever sees ports.PersonRepository, ports.GroupRepository,
// etc., never this generic type, per docs/adr/0001-hexagonal-architecture.md.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/store"

// Repository is the datastore-backed translator shared by every
// single-ID entity. Concurrency safety is the injected
// datastore.Datastore's responsibility, not this translation layer's.
type Repository[T any] struct {
	name       string
	collection string
	ds         datastore.Datastore
	idOf       func(*T) string

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

// New constructs a named Repository[T] backed by ds, storing documents
// under collection. idOf extracts T's own ID — injected rather than
// required via a method on T, so this package makes no assumption about a
// domain type's field names or method set.
func New[T any](name, collection string, ds datastore.Datastore, idOf func(*T) string, opts ...Option) (*Repository[T], error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/store: name must not be empty")
	}
	if collection == "" {
		return nil, fmt.Errorf("adapters/store: collection must not be empty")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/store: ds must not be nil")
	}
	if idOf == nil {
		return nil, fmt.Errorf("adapters/store: idOf must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository[T]{
		name:       name,
		collection: collection,
		ds:         ds,
		idOf:       idOf,
		logger:     o.logger.With("component", "adapters.store."+collection, "repository.name", name),
		tracer:     o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if r.creates, err = meter.Int64Counter(collection+"_repository.creates", metric.WithDescription(collection+" records created")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter(collection+"_repository.gets", metric.WithDescription(collection+" Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter(collection+"_repository.updates", metric.WithDescription(collection+" records updated")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter(collection+"_repository.deletes", metric.WithDescription(collection+" records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter(collection+"_repository.lists", metric.WithDescription(collection+" List calls")); err != nil {
		return nil, fmt.Errorf("adapters/store: creating lists counter: %w", err)
	}

	r.logger.Info(collection + " repository created")
	return r, nil
}

// Create stores v under its own ID. Returns ports.ErrConflict if a
// document with that ID already exists in this collection.
func (r *Repository[T]) Create(ctx context.Context, v *T) error {
	id := r.idOf(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Create(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data}); err != nil {
		return err
	}

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" created", r.collection+".id", id)
	return nil
}

// Get returns the record stored under id. Returns ports.ErrNotFound if
// none exists.
func (r *Repository[T]) Get(ctx context.Context, id string) (*T, error) {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	doc, err := r.ds.Get(ctx, r.collection, id)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			span.SetAttributes(attribute.Bool(r.collection+".found", false))
		}
		return nil, err
	}

	var v T
	if err := json.Unmarshal(doc.Data, &v); err != nil {
		return nil, fmt.Errorf("adapters/store: unmarshal %s %s: %w", r.collection, id, err)
	}
	return &v, nil
}

// Update replaces the record stored under v's own ID. Returns
// ports.ErrNotFound if none exists.
func (r *Repository[T]) Update(ctx context.Context, v *T) error {
	id := r.idOf(v)
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("adapters/store: marshal %s %s: %w", r.collection, id, err)
	}

	if err := r.ds.Update(ctx, datastore.Document{Collection: r.collection, ID: id, Data: data}); err != nil {
		return err
	}

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" updated", r.collection+".id", id)
	return nil
}

// Delete removes the record stored under id. Returns ports.ErrNotFound if
// none exists.
func (r *Repository[T]) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String(r.collection+".id", id)))
	defer span.End()

	if err := r.ds.Delete(ctx, r.collection, id); err != nil {
		return err
	}

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" deleted", r.collection+".id", id)
	return nil
}

// List returns records in this collection, cursor-paginated. This
// generic type only serves ports with no filter arguments, so the
// underlying datastore.Datastore.List filter is always nil.
func (r *Repository[T]) List(ctx context.Context, pageSize int, pageToken string) ([]*T, string, error) {
	ctx, span := r.tracer.Start(ctx, r.collection+"_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	docs, nextToken, err := r.ds.List(ctx, r.collection, nil, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}

	records := make([]*T, 0, len(docs))
	for _, doc := range docs {
		var v T
		if err := json.Unmarshal(doc.Data, &v); err != nil {
			return nil, "", fmt.Errorf("adapters/store: unmarshal %s %s: %w", r.collection, doc.ID, err)
		}
		records = append(records, &v)
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, r.collection+" list", "count", len(records), "next_page_token", nextToken)
	return records, nextToken, nil
}

// Option customizes a Repository[T] constructed via New.
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
