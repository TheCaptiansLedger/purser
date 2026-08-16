// Package memory is the in-memory adapter for the cache.Cache port defined
// in pkg/cache. It bounds entries by count (MaxItems) and approximate total
// size (MaxBytes), and lazily expires entries older than DefaultTTL. See
// ADR 0001 for why this lives behind cache.Cache rather than being used
// directly by callers.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"purser/pkg/cache"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/pkg/cache/memory"

// entry is what's actually stored in the LRU; expiresAt is the zero Time
// when the entry has no TTL.
type entry struct {
	data      []byte
	expiresAt time.Time
}

func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

func (e entry) size(key string) int64 {
	return int64(len(key) + len(e.data))
}

// Cache is the in-memory cache.Cache adapter.
type Cache struct {
	name string
	cfg  cache.Config

	lru *lru.Cache[string, entry]

	currentBytes atomic.Int64
	closed       atomic.Bool

	// Cumulative counters backing Stats. These mirror what's recorded to
	// the OTel counters below, kept separately because OTel instruments
	// are write-only from application code — Stats needs a synchronously
	// readable value.
	statHits      atomic.Int64
	statMisses    atomic.Int64
	statSets      atomic.Int64
	statDeletes   atomic.Int64
	statEvictions atomic.Int64

	logger *slog.Logger
	tracer trace.Tracer

	hits      metric.Int64Counter
	misses    metric.Int64Counter
	sets      metric.Int64Counter
	deletes   metric.Int64Counter
	evictions metric.Int64Counter
	flushes   metric.Int64Counter
}

// New constructs a named in-memory Cache. name identifies this instance in
// logs, traces, and metrics — two instances with different names can carry
// different Config values (per-module caches), per ADR's "name a cache"
// requirement.
func New(name string, cfg cache.Config, opts ...Option) (cache.Cache, error) {
	if name == "" {
		return nil, fmt.Errorf("cache/memory: name must not be empty")
	}
	if cfg.MaxItems <= 0 {
		return nil, fmt.Errorf("cache/memory: MaxItems must be > 0, got %d", cfg.MaxItems)
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &Cache{
		name:   name,
		cfg:    cfg,
		logger: o.logger.With("component", "cache.memory", "cache.name", name),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	attrs := metric.WithAttributes(attribute.String("cache.name", name))

	var err error
	if c.hits, err = meter.Int64Counter("cache.hits", metric.WithDescription("cache hits")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating hits counter: %w", err)
	}
	if c.misses, err = meter.Int64Counter("cache.misses", metric.WithDescription("cache misses")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating misses counter: %w", err)
	}
	if c.sets, err = meter.Int64Counter("cache.sets", metric.WithDescription("cache sets")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating sets counter: %w", err)
	}
	if c.deletes, err = meter.Int64Counter("cache.deletes", metric.WithDescription("explicit cache deletes")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating deletes counter: %w", err)
	}
	if c.evictions, err = meter.Int64Counter("cache.evictions", metric.WithDescription("entries removed due to capacity or TTL pressure")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating evictions counter: %w", err)
	}
	if c.flushes, err = meter.Int64Counter("cache.flushes", metric.WithDescription("explicit cache flushes")); err != nil {
		return nil, fmt.Errorf("cache/memory: creating flushes counter: %w", err)
	}

	l, err := lru.NewWithEvict(cfg.MaxItems, c.onEvicted)
	if err != nil {
		return nil, fmt.Errorf("cache/memory: constructing LRU: %w", err)
	}
	c.lru = l

	if _, err := meter.Int64ObservableGauge("cache.items",
		metric.WithDescription("current number of entries"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(c.lru.Len()), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("cache/memory: creating items gauge: %w", err)
	}

	if _, err := meter.Int64ObservableGauge("cache.bytes",
		metric.WithDescription("current approximate total size in bytes"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(c.currentBytes.Load(), attrs)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("cache/memory: creating bytes gauge: %w", err)
	}

	c.logger.Info("cache created",
		"max_items", cfg.MaxItems,
		"max_bytes", cfg.MaxBytes,
		"default_ttl", cfg.DefaultTTL,
	)

	return c, nil
}

// onEvicted keeps the byte-size accounting correct regardless of why an
// entry left the LRU (capacity eviction on Add, explicit Remove/Delete, our
// own MaxBytes-driven RemoveOldest loop, or Purge on Close) — the
// underlying library routes all of those through this single callback.
// Eviction-reason metrics are recorded at the call site instead, since this
// callback alone can't tell them apart.
func (c *Cache) onEvicted(key string, v entry) {
	c.currentBytes.Add(-v.size(key))
}

// Get implements cache.Cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if c.closed.Load() {
		return nil, false, cache.ErrClosed
	}

	ctx, span := c.tracer.Start(ctx, "cache.get", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	attrs := metric.WithAttributes(attribute.String("cache.name", c.name))

	e, ok := c.lru.Get(key)
	if !ok {
		c.statMisses.Add(1)
		c.misses.Add(ctx, 1, attrs)
		span.SetAttributes(attribute.Bool("cache.hit", false))
		c.logger.DebugContext(ctx, "cache miss", "key", key)
		return nil, false, nil
	}

	if e.expired(time.Now()) {
		c.lru.Remove(key)
		c.statEvictions.Add(1)
		c.evictions.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name), attribute.String("reason", "ttl")))
		c.statMisses.Add(1)
		c.misses.Add(ctx, 1, attrs)
		span.SetAttributes(attribute.Bool("cache.hit", false))
		c.logger.DebugContext(ctx, "cache miss (expired)", "key", key)
		return nil, false, nil
	}

	c.statHits.Add(1)
	c.hits.Add(ctx, 1, attrs)
	span.SetAttributes(attribute.Bool("cache.hit", true))
	c.logger.DebugContext(ctx, "cache hit", "key", key)
	return e.data, true, nil
}

// Set implements cache.Cache.
func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	if c.closed.Load() {
		return cache.ErrClosed
	}

	ctx, span := c.tracer.Start(ctx, "cache.set", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	e := entry{data: value}
	if c.cfg.DefaultTTL > 0 {
		e.expiresAt = time.Now().Add(c.cfg.DefaultTTL)
	}

	if old, ok := c.lru.Peek(key); ok {
		c.currentBytes.Add(-old.size(key))
	}

	if c.cfg.MaxBytes > 0 {
		for c.currentBytes.Load()+e.size(key) > c.cfg.MaxBytes {
			if _, _, ok := c.lru.RemoveOldest(); !ok {
				break
			}
			c.statEvictions.Add(1)
			c.evictions.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name), attribute.String("reason", "capacity_bytes")))
		}
	}

	evicted := c.lru.Add(key, e)
	c.currentBytes.Add(e.size(key))
	if evicted {
		c.statEvictions.Add(1)
		c.evictions.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name), attribute.String("reason", "capacity_items")))
	}

	c.statSets.Add(1)
	c.sets.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name)))
	c.logger.DebugContext(ctx, "cache set", "key", key, "bytes", e.size(key))
	return nil
}

// Delete implements cache.Cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if c.closed.Load() {
		return cache.ErrClosed
	}

	ctx, span := c.tracer.Start(ctx, "cache.delete", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	c.lru.Remove(key)
	c.statDeletes.Add(1)
	c.deletes.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name)))
	c.logger.DebugContext(ctx, "cache delete", "key", key)
	return nil
}

// Len implements cache.Cache.
func (c *Cache) Len(ctx context.Context) (int, error) {
	if c.closed.Load() {
		return 0, cache.ErrClosed
	}

	_, span := c.tracer.Start(ctx, "cache.len", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	n := c.lru.Len()
	span.SetAttributes(attribute.Int("cache.len", n))
	return n, nil
}

// Stats implements cache.Cache.
func (c *Cache) Stats(ctx context.Context) (cache.Stats, error) {
	if c.closed.Load() {
		return cache.Stats{}, cache.ErrClosed
	}

	_, span := c.tracer.Start(ctx, "cache.stats", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	return cache.Stats{
		Items:     c.lru.Len(),
		Bytes:     c.currentBytes.Load(),
		Hits:      c.statHits.Load(),
		Misses:    c.statMisses.Load(),
		Sets:      c.statSets.Load(),
		Deletes:   c.statDeletes.Load(),
		Evictions: c.statEvictions.Load(),
	}, nil
}

// Flush implements cache.Cache.
func (c *Cache) Flush(ctx context.Context) error {
	if c.closed.Load() {
		return cache.ErrClosed
	}

	ctx, span := c.tracer.Start(ctx, "cache.flush", trace.WithAttributes(attribute.String("cache.name", c.name)))
	defer span.End()

	n := c.lru.Len()
	c.lru.Purge()
	c.currentBytes.Store(0)

	c.flushes.Add(ctx, 1, metric.WithAttributes(attribute.String("cache.name", c.name)))
	c.logger.InfoContext(ctx, "cache flushed", "items_removed", n)
	return nil
}

// Close implements cache.Cache.
func (c *Cache) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	c.lru.Purge()
	c.logger.Info("cache closed")
	return nil
}

// Option customizes a Cache constructed via New.
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
