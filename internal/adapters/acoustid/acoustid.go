// Package acoustid is the single AcoustID adapter implementing
// ports.AcoustIDClient. See
// docs/technical/pipeline-music-acoustid-adapter.md and
// docs/adr/0025-music-identification-confidence-scoring.md.
package acoustid

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"purser/pkg/httpclient"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	instrumentationName = "purser/internal/adapters/acoustid"

	// defaultBaseURL is AcoustID's single lookup endpoint. Overridable via
	// Config.BaseURL — e.g. to point at a fixture server in tests.
	defaultBaseURL = "https://api.acoustid.org/v2/lookup"

	// defaultRequestsPerSecond is a conservative placeholder, not a
	// verified AcoustID policy number — see
	// docs/technical/pipeline-music-acoustid-adapter.md's "Rate limiting"
	// section. Unlike MusicBrainz's 1 req/sec (documented provider
	// policy), this is tunable via Config.RequestsPerSecond precisely
	// because it isn't asserted correct.
	defaultRequestsPerSecond = 3.0

	// fpcalcBinary is the external binary Fingerprint shells out to. Not
	// configurable: resolved from PATH, the same assumption cmd/purser's
	// startup preflight check verifies.
	fpcalcBinary = "fpcalc"
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig except APIKey, which has no safe default and must be
// supplied by the caller. Field names/tags follow the
// PURSER_<NESTED>_<KEY> convention documented in ADR 0010 for future Viper
// embedding.
type Config struct {
	// BaseURL is the AcoustID lookup endpoint every Lookup request is
	// issued against.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is the AcoustID client API key sent as the "client" query
	// param on every Lookup request. Required; New returns an error if
	// empty.
	APIKey string `mapstructure:"api_key"`

	// RequestsPerSecond is the client-side rate limit applied to Lookup.
	// A conservative, unverified placeholder — see defaultRequestsPerSecond.
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as the
	// MusicBrainz adapter.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport. Lookup
	// is issued as a GET specifically so it's cacheable by this transport
	// (which only caches GET/HEAD) — the same reuse requirement the
	// MusicBrainz adapter satisfies.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// APIKey which callers must supply themselves.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:           defaultBaseURL,
		RequestsPerSecond: defaultRequestsPerSecond,
		HTTPClient:        httpclient.DefaultConfig(),
		Cache:             cacheCfg,
	}
}

// Client is the AcoustID ports.AcoustIDClient adapter. Safe for concurrent
// use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	cache   cache.Cache
	limiter *rate.Limiter

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.AcoustIDClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as the MusicBrainz adapter.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/acoustid: BaseURL must not be empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/acoustid: APIKey must not be empty")
	}
	if cfg.RequestsPerSecond <= 0 {
		return nil, fmt.Errorf("adapters/acoustid: RequestsPerSecond must be > 0")
	}
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
		return nil, fmt.Errorf("adapters/acoustid: building http client: %w", err)
	}

	c, err := memory.New("acoustid", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/acoustid: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/acoustid: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("acoustid.requests", metric.WithDescription("AcoustID API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/acoustid: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:  cfg.BaseURL,
		apiKey:   cfg.APIKey,
		http:     httpClient,
		cache:    c,
		limiter:  rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), 1),
		logger:   o.logger.With("component", "adapters.acoustid"),
		tracer:   o.tracerProvider.Tracer(instrumentationName),
		requests: requests,
	}, nil
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// fpcalcOutput is the JSON shape `fpcalc -json` reports.
type fpcalcOutput struct {
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

// Fingerprint implements ports.AcoustIDClient: shells out to fpcalc,
// parses its JSON output. Local only — no network call, no cache, no rate
// limiting.
func (c *Client) Fingerprint(ctx context.Context, path string) (string, float64, error) {
	ctx, span := c.tracer.Start(ctx, "acoustid.fingerprint", trace.WithAttributes(
		attribute.String("pipeline.path", path),
	))
	defer span.End()

	cmd := exec.CommandContext(ctx, fpcalcBinary, "-json", path) //nolint:gosec // path is caller-supplied by design; fingerprinting arbitrary discovered files is this package's purpose
	out, err := cmd.Output()
	if err != nil {
		c.logger.ErrorContext(ctx, "fpcalc failed", "path", path, "error", err)
		return "", 0, fmt.Errorf("adapters/acoustid: running fpcalc on %s: %w", path, err)
	}

	var parsed fpcalcOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", 0, fmt.Errorf("adapters/acoustid: parsing fpcalc output for %s: %w", path, err)
	}

	c.logger.DebugContext(ctx, "fingerprinted file", "path", path, "duration_seconds", parsed.Duration)
	return parsed.Fingerprint, parsed.Duration, nil
}

// lookupResponse is the AcoustID API's Lookup response shape.
type lookupResponse struct {
	Status  string                `json:"status"`
	Error   *lookupError          `json:"error,omitempty"`
	Results []ports.AcoustIDMatch `json:"results"`
}

type lookupError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Lookup implements ports.AcoustIDClient: a rate-limited GET against the
// AcoustID lookup endpoint. Issued as GET (not POST, despite AcoustID
// supporting both) specifically so it flows through the shared
// httpclient caching transport, which only caches GET/HEAD responses —
// see docs/technical/pipeline-music-acoustid-adapter.md.
func (c *Client) Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]ports.AcoustIDMatch, error) {
	ctx, span := c.tracer.Start(ctx, "acoustid.lookup")
	defer span.End()

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("adapters/acoustid: rate limiter: %w", err)
	}

	q := url.Values{}
	q.Set("client", c.apiKey)
	q.Set("fingerprint", fingerprint)
	q.Set("duration", strconv.Itoa(int(durationSeconds+0.5)))
	q.Set("meta", "recordings+releasegroups")
	q.Set("format", "json")

	reqURL := c.baseURL + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("adapters/acoustid: building lookup request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adapters/acoustid: requesting lookup: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("adapters/acoustid: lookup: unexpected status %d", resp.StatusCode)
	}

	var out lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("adapters/acoustid: decoding lookup response: %w", err)
	}

	if out.Status == "error" {
		msg := "unknown error"
		if out.Error != nil {
			msg = out.Error.Message
		}
		return nil, fmt.Errorf("adapters/acoustid: lookup: %s", msg)
	}

	if len(out.Results) == 0 {
		c.logger.DebugContext(ctx, "acoustid lookup: no match")
		return nil, fmt.Errorf("adapters/acoustid: lookup: %w", ports.ErrNotFound)
	}

	c.logger.DebugContext(ctx, "acoustid lookup succeeded", "match_count", len(out.Results))
	return out.Results, nil
}

// Option customizes a Client constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	baseTransport  http.RoundTripper
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

// WithBaseTransport overrides the transport pkg/httpclient.New builds from
// cfg.HTTPClient with rt, forwarded via httpclient.WithBaseTransport —
// instrumentation and the caching transport still wrap it, only the actual
// socket is replaced. Used to run this Client against a canned
// pkg/httpclient/httpmock.Transport instead of a live network call, in
// tests or in a CI-only deployment mode.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.baseTransport = rt }
}
