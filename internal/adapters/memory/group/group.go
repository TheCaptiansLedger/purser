// Package group is the in-memory adapter for the ports.GroupRepository
// port. See internal/adapters/memory/person for the convention this
// follows.
package group

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

const instrumentationName = "purser/internal/adapters/memory/group"

// Repository is the in-memory ports.GroupRepository adapter. Safe for
// concurrent use.
type Repository struct {
	name string

	mu   sync.RWMutex
	byID map[string]*domain.Group

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
		return nil, fmt.Errorf("adapters/memory/group: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byID:   make(map[string]*domain.Group),
		logger: o.logger.With("component", "adapters.memory.group", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("group_repository.creates", metric.WithDescription("Group records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("group_repository.gets", metric.WithDescription("Group Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("group_repository.updates", metric.WithDescription("Group records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("group_repository.deletes", metric.WithDescription("Group records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("group_repository.lists", metric.WithDescription("Group List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("group_repository.items",
		metric.WithDescription("current number of Group records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byID)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/group: creating items gauge: %w", err)
	}

	r.logger.Info("group repository created")
	return r, nil
}

// Create implements ports.GroupRepository.
func (r *Repository) Create(ctx context.Context, g *domain.Group) error {
	ctx, span := r.tracer.Start(ctx, "group_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("group.id", g.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[g.ID]; exists {
		return ports.ErrConflict
	}

	stored := *g
	r.byID[g.ID] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "group created", "group.id", g.ID)
	return nil
}

// Get implements ports.GroupRepository.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Group, error) {
	ctx, span := r.tracer.Start(ctx, "group_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("group.id", id)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	g, ok := r.byID[id]
	if !ok {
		span.SetAttributes(attribute.Bool("group.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *g
	return &stored, nil
}

// Update implements ports.GroupRepository.
func (r *Repository) Update(ctx context.Context, g *domain.Group) error {
	ctx, span := r.tracer.Start(ctx, "group_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("group.id", g.ID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[g.ID]; !ok {
		return ports.ErrNotFound
	}

	stored := *g
	r.byID[g.ID] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "group updated", "group.id", g.ID)
	return nil
}

// Delete implements ports.GroupRepository.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "group_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("group.id", id)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byID, id)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "group deleted", "group.id", id)
	return nil
}

// List implements ports.GroupRepository. See
// internal/adapters/memory/person for the pagination convention.
func (r *Repository) List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Group, string, error) {
	ctx, span := r.tracer.Start(ctx, "group_repository.list",
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

	groups := make([]*domain.Group, 0, end-start)
	for _, id := range ids[start:end] {
		stored := *r.byID[id]
		groups = append(groups, &stored)
	}
	r.mu.RUnlock()

	var nextToken string
	if end < len(ids) {
		nextToken = ids[end-1]
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "group list", "count", len(groups), "next_page_token", nextToken)
	return groups, nextToken, nil
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
