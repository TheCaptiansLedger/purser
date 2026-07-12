// Package itemperson is the in-memory adapter for the
// ports.ItemPersonRepository port. See
// internal/adapters/memory/entryperson for the composite-key convention
// this follows.
package itemperson

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

const instrumentationName = "purser/internal/adapters/memory/itemperson"

func key(itemID, personID, role string) string {
	return itemID + "\x00" + personID + "\x00" + role
}

// Repository is the in-memory ports.ItemPersonRepository adapter. Safe
// for concurrent use.
type Repository struct {
	name string

	mu     sync.RWMutex
	byKey  map[string]*domain.ItemPerson
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
		return nil, fmt.Errorf("adapters/memory/itemperson: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byKey:  make(map[string]*domain.ItemPerson),
		logger: o.logger.With("component", "adapters.memory.itemperson", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("item_person_repository.creates", metric.WithDescription("ItemPerson records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("item_person_repository.gets", metric.WithDescription("ItemPerson Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("item_person_repository.updates", metric.WithDescription("ItemPerson records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("item_person_repository.deletes", metric.WithDescription("ItemPerson records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("item_person_repository.lists", metric.WithDescription("ItemPerson List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("item_person_repository.items",
		metric.WithDescription("current number of ItemPerson records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byKey)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/itemperson: creating items gauge: %w", err)
	}

	r.logger.Info("item person repository created")
	return r, nil
}

// Create implements ports.ItemPersonRepository.
func (r *Repository) Create(ctx context.Context, ip *domain.ItemPerson) error {
	k := key(ip.ItemID, ip.PersonID, ip.Role)
	ctx, span := r.tracer.Start(ctx, "item_person_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("item_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byKey[k]; exists {
		return ports.ErrConflict
	}

	stored := *ip
	r.byKey[k] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "item person created", "item_person.key", k)
	return nil
}

// Get implements ports.ItemPersonRepository.
func (r *Repository) Get(ctx context.Context, itemID, personID, role string) (*domain.ItemPerson, error) {
	k := key(itemID, personID, role)
	ctx, span := r.tracer.Start(ctx, "item_person_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("item_person.key", k)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	ip, ok := r.byKey[k]
	if !ok {
		span.SetAttributes(attribute.Bool("item_person.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *ip
	return &stored, nil
}

// Update implements ports.ItemPersonRepository.
func (r *Repository) Update(ctx context.Context, ip *domain.ItemPerson) error {
	k := key(ip.ItemID, ip.PersonID, ip.Role)
	ctx, span := r.tracer.Start(ctx, "item_person_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("item_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byKey[k]; !ok {
		return ports.ErrNotFound
	}

	stored := *ip
	r.byKey[k] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "item person updated", "item_person.key", k)
	return nil
}

// Delete implements ports.ItemPersonRepository.
func (r *Repository) Delete(ctx context.Context, itemID, personID, role string) error {
	k := key(itemID, personID, role)
	ctx, span := r.tracer.Start(ctx, "item_person_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("item_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byKey, k)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "item person deleted", "item_person.key", k)
	return nil
}

// List implements ports.ItemPersonRepository. itemID and personID are
// independent, optional filters. See internal/adapters/memory/person for
// the pagination convention.
func (r *Repository) List(ctx context.Context, itemID, personID string, pageSize int, pageToken string) ([]*domain.ItemPerson, string, error) {
	ctx, span := r.tracer.Start(ctx, "item_person_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	r.mu.RLock()
	keys := make([]string, 0, len(r.byKey))
	for k, ip := range r.byKey {
		if itemID != "" && ip.ItemID != itemID {
			continue
		}
		if personID != "" && ip.PersonID != personID {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	start := 0
	if pageToken != "" {
		start = sort.SearchStrings(keys, pageToken)
		if start < len(keys) && keys[start] == pageToken {
			start++
		}
	}

	end := min(start+pageSize, len(keys))

	rows := make([]*domain.ItemPerson, 0, end-start)
	for _, k := range keys[start:end] {
		stored := *r.byKey[k]
		rows = append(rows, &stored)
	}
	r.mu.RUnlock()

	var nextToken string
	if end < len(keys) {
		nextToken = keys[end-1]
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "item person list", "count", len(rows), "next_page_token", nextToken)
	return rows, nextToken, nil
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
