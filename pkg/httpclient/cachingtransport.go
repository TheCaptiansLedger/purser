package httpclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Cache is the minimal capability NewCachingTransport needs from a cache
// implementation. Any purser/pkg/cache.Cache implementation (e.g.
// pkg/cache/memory) satisfies this automatically — Go checks method sets
// structurally, so this package never imports pkg/cache.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte) error
}

// cachingTransport implements http.RoundTripper by wrapping another
// RoundTripper and adding response caching in front of it — it remains
// substitutable anywhere an http.RoundTripper is expected, e.g. as
// client.Transport.
type cachingTransport struct {
	next  http.RoundTripper
	cache Cache

	logger *slog.Logger
	tracer trace.Tracer

	hits   metric.Int64Counter
	misses metric.Int64Counter
}

// NewCachingTransport wraps next so cacheable GET/HEAD responses are served
// from c on subsequent requests instead of round-tripping again. Every
// other method passes straight through to next untouched. Entry lifetime is
// governed entirely by c's own configuration (see pkg/cache) —
// NewCachingTransport has no TTL of its own.
//
//	client, _ := httpclient.New(cfg)
//	client.Transport, _ = httpclient.NewCachingTransport(client.Transport, c)
func NewCachingTransport(next http.RoundTripper, c Cache, opts ...Option) (http.RoundTripper, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	meter := o.meterProvider.Meter(instrumentationName)
	hits, err := meter.Int64Counter("http.cache.hits", metric.WithDescription("HTTP responses served from cache"))
	if err != nil {
		return nil, fmt.Errorf("httpclient: creating cache hits counter: %w", err)
	}
	misses, err := meter.Int64Counter("http.cache.misses", metric.WithDescription("HTTP requests not served from cache"))
	if err != nil {
		return nil, fmt.Errorf("httpclient: creating cache misses counter: %w", err)
	}

	return &cachingTransport{
		next:   next,
		cache:  c,
		logger: o.logger.With("component", "httpclient.cache"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
		hits:   hits,
		misses: misses,
	}, nil
}

// RoundTrip implements http.RoundTripper.
func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.next.RoundTrip(req)
	}

	ctx, span := t.tracer.Start(req.Context(), "httpclient.cache.round_trip",
		trace.WithAttributes(attribute.String("http.request.method", req.Method)),
	)
	defer span.End()

	key := cacheKey(req)

	if data, ok, err := t.cache.Get(ctx, key); err != nil {
		t.logger.WarnContext(ctx, "http cache lookup failed", "key", key, "error", err)
	} else if ok {
		resp, rerr := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), req)
		if rerr == nil {
			t.hits.Add(ctx, 1, metric.WithAttributes(attribute.String("http.request.method", req.Method)))
			span.SetAttributes(attribute.Bool("http.cache.hit", true))
			t.logger.DebugContext(ctx, "http cache hit", "key", key)
			return resp, nil
		}
		t.logger.WarnContext(ctx, "discarding corrupt http cache entry", "key", key, "error", rerr)
	}

	t.misses.Add(ctx, 1, metric.WithAttributes(attribute.String("http.request.method", req.Method)))
	span.SetAttributes(attribute.Bool("http.cache.hit", false))

	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == http.StatusOK {
		if dumped, derr := httputil.DumpResponse(resp, true); derr == nil {
			if serr := t.cache.Set(ctx, key, dumped); serr != nil {
				t.logger.WarnContext(ctx, "failed to store http response in cache", "key", key, "error", serr)
			}
		}
	}

	return resp, nil
}

func cacheKey(req *http.Request) string {
	return req.Method + " " + req.URL.String()
}
