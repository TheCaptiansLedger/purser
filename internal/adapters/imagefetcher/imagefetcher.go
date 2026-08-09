// Package imagefetcher is the single adapter implementing
// ports.ImageFetcher — a provider-agnostic HTTP GET of an arbitrary image
// URL. See docs/adr/0013-image-blob-storage.md, the "Not built in this
// pass" gap this package fills in. Unlike internal/adapters/stashdb or
// internal/adapters/theporndb, this is not one adapter per external
// provider (docs/adr/0027-provider-independence.md's port-per-provider
// rule doesn't apply here): fetching an image's bytes from a URL a
// provider's own DTO already returned carries no provider-specific
// request shape, auth, or response envelope to abstract — it's plain
// infrastructure any module's Persister can share, today for
// StashDB/ThePornDB scene images, later for any other provider's images
// (fanart.tv, TheAudioDB, ...).
package imagefetcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"purser/pkg/httpclient"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/imagefetcher"

// maxAttempts bounds retries of a transient failure (HTTP 503, or a
// transport-level error) in Fetch — same convention as
// theporndb.go's/stashdb.go's doRequest.
const maxAttempts = 4

// Config configures a Client built by New. Field names/tags follow the
// PURSER_<NESTED>_<KEY> convention documented in ADR 0010 for future Viper
// embedding.
type Config struct {
	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as the
	// MusicBrainz/StashDB/ThePornDB adapters.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport — same
	// mechanism theporndb.go/stashdb.go already use, not the "wrong tool"
	// docs/adr/0013-image-blob-storage.md rejected for *permanent* image
	// storage (that's ImageStore's job); this is only a short-lived
	// dedup layer over repeated Fetch calls for the same URL.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from.
func DefaultConfig() Config {
	return Config{
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cache.DefaultConfig(),
	}
}

// Client is the ports.ImageFetcher adapter. Safe for concurrent use.
type Client struct {
	http           *http.Client
	cache          cache.Cache
	retryBaseDelay time.Duration

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.ImageFetcher = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent — same pattern theporndb.go/stashdb.go follow.
func New(cfg Config, opts ...Option) (*Client, error) {
	cfg.HTTPClient.UserAgent = "Purser/" + version.Version

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	httpClientOpts := []httpclient.Option{
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	}
	if o.baseTransport != nil {
		httpClientOpts = append(httpClientOpts, httpclient.WithBaseTransport(o.baseTransport))
	}
	httpClient, err := httpclient.New(cfg.HTTPClient, httpClientOpts...)
	if err != nil {
		return nil, fmt.Errorf("adapters/imagefetcher: building http client: %w", err)
	}

	c, err := memory.New("imagefetcher", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/imagefetcher: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/imagefetcher: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("imagefetcher.requests", metric.WithDescription("Image fetch requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/imagefetcher: creating requests counter: %w", err)
	}

	return &Client{
		http:           httpClient,
		cache:          c,
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.imagefetcher"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
		requests:       requests,
	}, nil
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// Fetch implements ports.ImageFetcher. A transient failure (HTTP 503, or a
// transport-level error) is retried up to maxAttempts times with an
// increasing delay, same retry shape as theporndb.go's/stashdb.go's
// doRequest — deliberately with no rate limiter, unlike those adapters:
// url can point at any host (whichever CDN the calling provider's DTO
// happened to return), so there is no single shared per-adapter budget to
// pace against the way there is for one provider's own API.
func (c *Client) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	ctx, span := c.tracer.Start(ctx, "imagefetcher.fetch", trace.WithAttributes(
		attribute.String("imagefetcher.url", url),
	))
	defer span.End()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, status, err := c.fetchOnce(ctx, url, span)
		if err == nil {
			return body, nil
		}
		lastErr = err

		if attempt == maxAttempts || !retryableStatus(status) {
			return nil, err
		}

		delay := c.retryBaseDelay * time.Duration(attempt)
		c.logger.WarnContext(ctx, "image fetch failed, retrying",
			"url", url, "attempt", attempt, "delay", delay, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

// retryableStatus reports whether status is worth another attempt: 0 (a
// transport-level failure) or 503. Mirrors theporndb.go's/stashdb.go's
// identical function.
func retryableStatus(status int) bool {
	return status == 0 || status == http.StatusServiceUnavailable
}

// fetchOnce issues a single GET attempt against url, mapping a 404 to
// ports.ErrNotFound. On success, the caller owns the returned body and
// must Close it; on any error, the response body (if any) is already
// closed here. Returns the HTTP status actually received (0 if the
// request never got a response at all) so Fetch's retry loop can classify
// the failure via retryableStatus.
func (c *Client) fetchOnce(ctx context.Context, url string, span trace.Span) (io.ReadCloser, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("adapters/imagefetcher: building request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("adapters/imagefetcher: fetching %s: %w", url, err)
	}

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		c.logger.DebugContext(ctx, "image not found", "url", url)
		return nil, resp.StatusCode, fmt.Errorf("adapters/imagefetcher: fetching %s: %w", url, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, resp.StatusCode, fmt.Errorf("adapters/imagefetcher: fetching %s: unexpected status %d", url, resp.StatusCode)
	}

	c.logger.DebugContext(ctx, "image fetch succeeded", "url", url)
	return resp.Body, resp.StatusCode, nil
}

// Option customizes a Client constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	baseTransport  http.RoundTripper
	retryBaseDelay time.Duration
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
		retryBaseDelay: 2 * time.Second,
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

// WithRetryBaseDelay overrides the base delay Fetch waits before retrying
// a transient failure — attempt N's delay is this value times N. Defaults
// to 2s; tests override this to a near-zero value so a retry-path test
// doesn't have to actually sleep for several real seconds.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(o *options) { o.retryBaseDelay = d }
}

// WithBaseTransport overrides the transport pkg/httpclient.New builds from
// cfg.HTTPClient with rt, forwarded via httpclient.WithBaseTransport —
// instrumentation and the caching transport still wrap it, only the actual
// socket is replaced. Used to run this Client against a canned
// pkg/httpclient/httpmock.Transport instead of a live network call.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.baseTransport = rt }
}
