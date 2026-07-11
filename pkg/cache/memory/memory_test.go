package memory

import (
	"bytes"
	"context"
	"log/slog"
	"purser/pkg/cache"
	"purser/pkg/cache/cachetest"
	"strings"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestCache_Contract(t *testing.T) {
	cachetest.TestCache(t, func(t *testing.T, cfg cache.Config) cache.Cache {
		t.Helper()
		c, err := New("contract-test", cfg)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return c
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	t.Run("empty name is rejected", func(t *testing.T) {
		if _, err := New("", cache.DefaultConfig()); err == nil {
			t.Fatal("New with empty name did not return an error")
		}
	})

	t.Run("non-positive MaxItems is rejected", func(t *testing.T) {
		cfg := cache.DefaultConfig()
		cfg.MaxItems = 0
		if _, err := New("test", cfg); err == nil {
			t.Fatal("New with MaxItems=0 did not return an error")
		}
	})
}

func TestCache_MaxBytesEvictsOldestEntries(t *testing.T) {
	cfg := cache.DefaultConfig()
	cfg.MaxItems = 100
	cfg.MaxBytes = 6 // "a"+"aa" = 1+2=3 bytes per entry below (key+value)

	c, err := New("bytes-test", cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	_ = c.Set(ctx, "a", []byte("a")) // 2 bytes
	_ = c.Set(ctx, "b", []byte("b")) // 2 bytes, total 4
	_ = c.Set(ctx, "c", []byte("c")) // 2 bytes, total 6, still fits
	_ = c.Set(ctx, "d", []byte("d")) // would push to 8 > 6, evict "a"

	if _, ok, _ := c.Get(ctx, "a"); ok {
		t.Fatal("entry \"a\" should have been evicted by MaxBytes pressure")
	}
	if _, ok, _ := c.Get(ctx, "d"); !ok {
		t.Fatal("entry \"d\" should be present")
	}
}

func TestCache_SetTracksBytesAcrossOverwrite(t *testing.T) {
	cfg := cache.DefaultConfig()
	cfg.MaxBytes = 1024

	c, err := New("overwrite-bytes-test", cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("short")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Set(ctx, "key", []byte("a much longer value than before")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("Get missed after overwrite")
	}
	if string(got) != "a much longer value than before" {
		t.Fatalf("Get returned %q", got)
	}
}

func TestCache_DoubleCloseIsSafe(t *testing.T) {
	c, err := New("double-close-test", cache.DefaultConfig())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestCache_EmitsTraces(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	c, err := New("trace-test", cache.DefaultConfig(), WithTracerProvider(tp))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	_ = c.Set(ctx, "key", []byte("value"))
	_, _, _ = c.Get(ctx, "key")
	_, _, _ = c.Get(ctx, "missing")
	_ = c.Delete(ctx, "key")
	_, _ = c.Len(ctx)

	spans := exporter.GetSpans()
	if len(spans) != 5 {
		t.Fatalf("got %d spans, want 5", len(spans))
	}

	names := map[string]bool{}
	for _, s := range spans {
		names[s.Name] = true
	}
	for _, want := range []string{"cache.set", "cache.get", "cache.delete", "cache.len"} {
		if !names[want] {
			t.Errorf("missing span %q, got spans %v", want, names)
		}
	}
}

func TestCache_EmitsMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	c, err := New("metric-test", cache.DefaultConfig(), WithMeterProvider(mp))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	_ = c.Set(ctx, "key", []byte("value"))
	_, _, _ = c.Get(ctx, "key")
	_, _, _ = c.Get(ctx, "missing")
	_ = c.Delete(ctx, "key")

	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	found := map[string]bool{}
	for _, sm := range data.ScopeMetrics {
		for _, m := range sm.Metrics {
			found[m.Name] = true
		}
	}
	for _, want := range []string{"cache.hits", "cache.misses", "cache.sets", "cache.deletes", "cache.items", "cache.bytes"} {
		if !found[want] {
			t.Errorf("missing metric %q, got %v", want, found)
		}
	}
}

func TestCache_LogsAreStructured(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	c, err := New("log-test", cache.DefaultConfig(), WithLogger(logger))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := c.Set(context.Background(), "key", []byte("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	_ = c.Close()

	out := buf.String()
	if !strings.Contains(out, `"component":"cache.memory"`) {
		t.Errorf("log output missing component attribute: %s", out)
	}
	if !strings.Contains(out, `"cache.name":"log-test"`) {
		t.Errorf("log output missing cache.name attribute: %s", out)
	}
}

func TestCache_EvictionMetricReasons(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	cfg := cache.DefaultConfig()
	cfg.MaxItems = 1

	c, err := New("eviction-test", cfg, WithMeterProvider(mp))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	_ = c.Set(ctx, "a", []byte("a"))
	_ = c.Set(ctx, "b", []byte("b")) // evicts "a" via capacity_items

	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	var evictions *metricdata.Metrics
	for _, sm := range data.ScopeMetrics {
		for i, m := range sm.Metrics {
			if m.Name == "cache.evictions" {
				evictions = &sm.Metrics[i]
			}
		}
	}
	if evictions == nil {
		t.Fatal("cache.evictions metric not found")
	}
	sum, ok := evictions.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("cache.evictions has unexpected data type %T", evictions.Data)
	}
	if len(sum.DataPoints) == 0 || sum.DataPoints[0].Value != 1 {
		t.Fatalf("cache.evictions data points = %+v, want a single point with value 1", sum.DataPoints)
	}
}
