// Package entryperson is the in-memory adapter for the
// ports.EntryPersonRepository port. See internal/adapters/memory/person
// for the general convention this follows — the one difference is the
// composite (libraryEntryID, personID, role) key in place of a single ID.
package entryperson

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

const instrumentationName = "purser/internal/adapters/memory/entryperson"

func key(libraryEntryID, personID, role string) string {
	return libraryEntryID + "\x00" + personID + "\x00" + role
}

// Repository is the in-memory ports.EntryPersonRepository adapter. Safe
// for concurrent use.
type Repository struct {
	name string

	mu     sync.RWMutex
	byKey  map[string]*domain.EntryPerson
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
		return nil, fmt.Errorf("adapters/memory/entryperson: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byKey:  make(map[string]*domain.EntryPerson),
		logger: o.logger.With("component", "adapters.memory.entryperson", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("entry_person_repository.creates", metric.WithDescription("EntryPerson records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("entry_person_repository.gets", metric.WithDescription("EntryPerson Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("entry_person_repository.updates", metric.WithDescription("EntryPerson records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("entry_person_repository.deletes", metric.WithDescription("EntryPerson records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("entry_person_repository.lists", metric.WithDescription("EntryPerson List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("entry_person_repository.items",
		metric.WithDescription("current number of EntryPerson records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byKey)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/entryperson: creating items gauge: %w", err)
	}

	r.logger.Info("entry person repository created")
	return r, nil
}

// Create implements ports.EntryPersonRepository.
func (r *Repository) Create(ctx context.Context, ep *domain.EntryPerson) error {
	k := key(ep.LibraryEntryID, ep.PersonID, ep.Role)
	ctx, span := r.tracer.Start(ctx, "entry_person_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("entry_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byKey[k]; exists {
		return ports.ErrConflict
	}

	stored := *ep
	r.byKey[k] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "entry person created", "entry_person.key", k)
	return nil
}

// Get implements ports.EntryPersonRepository.
func (r *Repository) Get(ctx context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error) {
	k := key(libraryEntryID, personID, role)
	ctx, span := r.tracer.Start(ctx, "entry_person_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("entry_person.key", k)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	ep, ok := r.byKey[k]
	if !ok {
		span.SetAttributes(attribute.Bool("entry_person.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *ep
	return &stored, nil
}

// Update implements ports.EntryPersonRepository.
func (r *Repository) Update(ctx context.Context, ep *domain.EntryPerson) error {
	k := key(ep.LibraryEntryID, ep.PersonID, ep.Role)
	ctx, span := r.tracer.Start(ctx, "entry_person_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("entry_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byKey[k]; !ok {
		return ports.ErrNotFound
	}

	stored := *ep
	r.byKey[k] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "entry person updated", "entry_person.key", k)
	return nil
}

// Delete implements ports.EntryPersonRepository.
func (r *Repository) Delete(ctx context.Context, libraryEntryID, personID, role string) error {
	k := key(libraryEntryID, personID, role)
	ctx, span := r.tracer.Start(ctx, "entry_person_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("entry_person.key", k)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byKey, k)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "entry person deleted", "entry_person.key", k)
	return nil
}

// List implements ports.EntryPersonRepository. libraryEntryID and
// personID are independent, optional filters. See
// internal/adapters/memory/person for the pagination convention.
func (r *Repository) List(ctx context.Context, libraryEntryID, personID string, pageSize int, pageToken string) ([]*domain.EntryPerson, string, error) {
	ctx, span := r.tracer.Start(ctx, "entry_person_repository.list",
		trace.WithAttributes(attribute.String("repository.name", r.name)))
	defer span.End()

	if pageSize <= 0 {
		pageSize = 50
	}

	r.mu.RLock()
	keys := make([]string, 0, len(r.byKey))
	for k, ep := range r.byKey {
		if libraryEntryID != "" && ep.LibraryEntryID != libraryEntryID {
			continue
		}
		if personID != "" && ep.PersonID != personID {
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

	rows := make([]*domain.EntryPerson, 0, end-start)
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
	r.logger.DebugContext(ctx, "entry person list", "count", len(rows), "next_page_token", nextToken)
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
