// Package person is the in-memory adapter for the ports.PersonRepository
// port. See ADR 0001 for why this lives behind the port rather than being
// used directly, and ADR 0011 for why an in-memory backend comes first — a
// persistent adapter (sqlite/badger) implements the same port later,
// proven against the same contract test in internal/ports/persontest.
package person

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

const instrumentationName = "purser/internal/adapters/memory/person"

// Repository is the in-memory ports.PersonRepository adapter. Safe for
// concurrent use.
type Repository struct {
	name string

	mu   sync.RWMutex
	byID map[string]*domain.Person

	logger *slog.Logger
	tracer trace.Tracer

	creates metric.Int64Counter
	gets    metric.Int64Counter
	updates metric.Int64Counter
	deletes metric.Int64Counter
	lists   metric.Int64Counter
}

// New constructs a named in-memory Repository. name identifies this
// instance in logs, traces, and metrics, same convention as
// pkg/cache/memory.
func New(name string, opts ...Option) (*Repository, error) {
	if name == "" {
		return nil, fmt.Errorf("adapters/memory/person: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byID:   make(map[string]*domain.Person),
		logger: o.logger.With("component", "adapters.memory.person", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("person_repository.creates", metric.WithDescription("Person records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("person_repository.gets", metric.WithDescription("Person Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("person_repository.updates", metric.WithDescription("Person records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("person_repository.deletes", metric.WithDescription("Person records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("person_repository.lists", metric.WithDescription("Person List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("person_repository.items",
		metric.WithDescription("current number of Person records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byID)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/person: creating items gauge: %w", err)
	}

	r.logger.Info("person repository created")
	return r, nil
}

// Create implements ports.PersonRepository.
func (r *Repository) Create(ctx context.Context, p *domain.Person) error {
	ctx, span := r.tracer.Start(ctx, "person_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("person.id", p.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[p.ID]; exists {
		return ports.ErrConflict
	}

	stored := *p
	r.byID[p.ID] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "person created", "person.id", p.ID)
	return nil
}

// Get implements ports.PersonRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Person, error) {
	ctx, span := r.tracer.Start(ctx, "person_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("person.id", id)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	p, ok := r.byID[id]
	if !ok {
		span.SetAttributes(attribute.Bool("person.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *p
	return &stored, nil
}

// Update implements ports.PersonRepository.
func (r *Repository) Update(ctx context.Context, p *domain.Person) error {
	ctx, span := r.tracer.Start(ctx, "person_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("person.id", p.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[p.ID]; !ok {
		return ports.ErrNotFound
	}

	stored := *p
	r.byID[p.ID] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "person updated", "person.id", p.ID)
	return nil
}

// Delete implements ports.PersonRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "person_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("person.id", id)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byID, id)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "person deleted", "person.id", id)
	return nil
}

// List implements ports.PersonRepository. Pagination is a stable
// lexicographic sort over IDs — deterministic under an in-memory map's
// unordered iteration, and simple enough not to need a separate insertion-
// order index. The opaque page token is the last ID returned on the
// previous page; a real backend's token format may differ, callers must
// not assume a shape.
func (r *Repository) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Person, string, error) {
	ctx, span := r.tracer.Start(ctx, "person_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	r.mu.RLock()
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
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

	people := make([]*domain.Person, 0, end-start)
	for _, id := range ids[start:end] {
		stored := *r.byID[id]
		people = append(people, &stored)
	}
	r.mu.RUnlock()

	var nextToken string
	if end < len(ids) {
		nextToken = ids[end-1]
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "person list", "count", len(people), "next_page_token", nextToken)
	return people, nextToken, nil
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
