// Package performerprofile is the in-memory adapter for the
// ports.PerformerProfileRepository port. See internal/adapters/memory/person
// for the general convention this follows — the one difference is the
// personID-only key (like externalid, but a single field rather than a
// composite).
package performerprofile

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"sort"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/memory/performerprofile"

// Repository is the in-memory ports.PerformerProfileRepository adapter.
// Safe for concurrent use.
type Repository struct {
	name string

	mu     sync.RWMutex
	byID   map[string]*afterdark.PerformerProfile
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
		return nil, fmt.Errorf("adapters/memory/performerprofile: name must not be empty")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Repository{
		name:   name,
		byID:   make(map[string]*afterdark.PerformerProfile),
		logger: o.logger.With("component", "adapters.memory.performerprofile", "repository.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("repository.name", name))

	var err error
	if r.creates, err = meter.Int64Counter("performer_profile_repository.creates", metric.WithDescription("PerformerProfile records created")); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating creates counter: %w", err)
	}
	if r.gets, err = meter.Int64Counter("performer_profile_repository.gets", metric.WithDescription("PerformerProfile Get calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating gets counter: %w", err)
	}
	if r.updates, err = meter.Int64Counter("performer_profile_repository.updates", metric.WithDescription("PerformerProfile records updated")); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating updates counter: %w", err)
	}
	if r.deletes, err = meter.Int64Counter("performer_profile_repository.deletes", metric.WithDescription("PerformerProfile records deleted")); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating deletes counter: %w", err)
	}
	if r.lists, err = meter.Int64Counter("performer_profile_repository.lists", metric.WithDescription("PerformerProfile List calls")); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating lists counter: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("performer_profile_repository.items",
		metric.WithDescription("current number of PerformerProfile records"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			r.mu.RLock()
			defer r.mu.RUnlock()
			o.Observe(int64(len(r.byID)), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("adapters/memory/performerprofile: creating items gauge: %w", err)
	}

	r.logger.Info("performer profile repository created")
	return r, nil
}

// Create implements ports.PerformerProfileRepository.
func (r *Repository) Create(ctx context.Context, p *afterdark.PerformerProfile) error {
	ctx, span := r.tracer.Start(ctx, "performer_profile_repository.create",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("performer_profile.person_id", p.PersonID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[p.PersonID]; exists {
		return ports.ErrConflict
	}

	stored := *p
	r.byID[p.PersonID] = &stored

	r.creates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "performer profile created", "performer_profile.person_id", p.PersonID)
	return nil
}

// Get implements ports.PerformerProfileRepository.
func (r *Repository) Get(ctx context.Context, personID string) (*afterdark.PerformerProfile, error) {
	ctx, span := r.tracer.Start(ctx, "performer_profile_repository.get",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("performer_profile.person_id", personID)))
	defer span.End()

	r.mu.RLock()
	defer r.mu.RUnlock()

	r.gets.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))

	p, ok := r.byID[personID]
	if !ok {
		span.SetAttributes(attribute.Bool("performer_profile.found", false))
		return nil, ports.ErrNotFound
	}

	stored := *p
	return &stored, nil
}

// Update implements ports.PerformerProfileRepository.
func (r *Repository) Update(ctx context.Context, p *afterdark.PerformerProfile) error {
	ctx, span := r.tracer.Start(ctx, "performer_profile_repository.update",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("performer_profile.person_id", p.PersonID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[p.PersonID]; !ok {
		return ports.ErrNotFound
	}

	stored := *p
	r.byID[p.PersonID] = &stored

	r.updates.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "performer profile updated", "performer_profile.person_id", p.PersonID)
	return nil
}

// Delete implements ports.PerformerProfileRepository.
func (r *Repository) Delete(ctx context.Context, personID string) error {
	ctx, span := r.tracer.Start(ctx, "performer_profile_repository.delete",
		trace.WithAttributes(attribute.String("repository.name", r.name), attribute.String("performer_profile.person_id", personID)))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[personID]; !ok {
		return ports.ErrNotFound
	}
	delete(r.byID, personID)

	r.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "performer profile deleted", "performer_profile.person_id", personID)
	return nil
}

// List implements ports.PerformerProfileRepository. See
// internal/adapters/memory/person for the pagination convention.
func (r *Repository) List(ctx context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerProfile, string, error) {
	ctx, span := r.tracer.Start(ctx, "performer_profile_repository.list",
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

	profiles := make([]*afterdark.PerformerProfile, 0, end-start)
	for _, id := range ids[start:end] {
		stored := *r.byID[id]
		profiles = append(profiles, &stored)
	}
	r.mu.RUnlock()

	var nextToken string
	if end < len(ids) {
		nextToken = ids[end-1]
	}

	r.lists.Add(ctx, 1, metric.WithAttributes(attribute.String("repository.name", r.name)))
	r.logger.DebugContext(ctx, "performer profile list", "count", len(profiles), "next_page_token", nextToken)
	return profiles, nextToken, nil
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
