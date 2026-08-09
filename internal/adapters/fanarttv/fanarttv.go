// Package fanarttv is the single fanart.tv adapter implementing
// ports.FanartTVClient. See docs/adr/0027-provider-independence.md.
//
// fanart.tv (webservice.fanart.tv) runs a REST API at
// https://webservice.fanart.tv/v3/ — auth is an "api_key" query param,
// confirmed live against the real API during this adapter's
// implementation:
//
//   - GET v3/music/{mbid}?api_key={key} — one call returns both the
//     artist's own images and every one of their release groups' album
//     art, keyed by release-group MBID. A known MBID returns the full
//     object; an unknown one returns HTTP 200 with an empty {} body — not
//     a 404, the only provider in this codebase's adapter family where
//     "not found" isn't a status code. An invalid API key is a real,
//     distinct error: HTTP 401 with {"error":"invalid API key"}.
package fanarttv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"purser/pkg/httpclient"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	instrumentationName = "purser/internal/adapters/fanarttv"

	// defaultBaseURL is fanart.tv's real API root, trailing slash
	// included. Overridable via Config.BaseURL, e.g. to point at a
	// fixture server in tests.
	defaultBaseURL = "https://webservice.fanart.tv/v3/"

	// requestsPerSecond is a conservative, unverified placeholder — no
	// published rate-limit headers were observed on any live response
	// during this adapter's implementation, the same situation
	// theporndb.go's/theaudiodb.go's identical constant documents.
	// WithRateLimit exists purely as a test escape hatch, same as every
	// other adapter in this package family.
	requestsPerSecond = 2

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error) in get — see get's retry loop and
	// musicbrainz.go's/theporndb.go's identical convention.
	maxAttempts = 4
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig except APIKey, which has no safe default and must be
// supplied by the caller — fanart.tv requires it on every request. Field
// names/tags follow the PURSER_<NESTED>_<KEY> convention documented in ADR
// 0010 for future Viper embedding.
type Config struct {
	// BaseURL is fanart.tv's API root every request is issued against,
	// trailing slash included.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "api_key" query param on every request.
	// Required; New returns an error if empty.
	APIKey string `mapstructure:"api_key"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as every other
	// adapter in this package family.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// APIKey which callers must supply themselves. The cache's DefaultTTL
// mirrors the rest of this package family's: fanart.tv's per-artist image
// set is close to static.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the fanart.tv ports.FanartTVClient adapter. Safe for
// concurrent use.
type Client struct {
	baseURL        string
	apiKey         string
	http           *http.Client
	cache          cache.Cache
	limiter        *rate.Limiter
	retryBaseDelay time.Duration

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.FanartTVClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as every other adapter in this package
// family.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/fanarttv: BaseURL must not be empty")
	}
	if !strings.HasSuffix(cfg.BaseURL, "/") {
		cfg.BaseURL += "/"
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/fanarttv: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/fanarttv: building http client: %w", err)
	}

	c, err := memory.New("fanarttv", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/fanarttv: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/fanarttv: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("fanarttv.requests", metric.WithDescription("fanart.tv API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/fanarttv: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		apiKey:         cfg.APIKey,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.fanarttv"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
		requests:       requests,
	}, nil
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// LookupArtist implements ports.FanartTVClient. An unknown MBID's empty
// {} response decodes to a zero-value ports.FanartArtist (Name == ""),
// which is what signals ErrNotFound here — see the package doc comment.
func (c *Client) LookupArtist(ctx context.Context, mbid string) (*ports.FanartArtist, error) {
	q := url.Values{}
	q.Set("api_key", c.apiKey)

	var a ports.FanartArtist
	if err := c.get(ctx, "LookupArtist", "music/"+mbid, q, &a); err != nil {
		return nil, err
	}
	if a.Name == "" {
		return nil, fmt.Errorf("adapters/fanarttv: LookupArtist %s: %w", mbid, ports.ErrNotFound)
	}
	return &a, nil
}

// get issues a rate-limited GET against path (relative to baseURL) with
// query params q, decodes the JSON response into out, and maps a 404
// status to ports.ErrNotFound — the single choke point LookupArtist routes
// through, so rate limiting, tracing, metrics, and error mapping are
// implemented exactly once. A genuine 404 wasn't observed live (fanart.tv
// answers an unknown MBID with 200+{}, handled by LookupArtist itself) but
// is mapped defensively for symmetry with the rest of this package family.
//
// A transient failure (HTTP 503, or a transport-level error) is retried up
// to maxAttempts times with an increasing delay, still paced through the
// limiter on every attempt so a retry storm can't itself violate the rate
// limit. Anything else — including a 401 for an invalid API key — fails
// immediately as a real, non-ErrNotFound error.
func (c *Client) get(ctx context.Context, operationName, path string, q url.Values, out any) error {
	ctx, span := c.tracer.Start(ctx, "fanarttv."+operationName, trace.WithAttributes(
		attribute.String("fanarttv.path", path),
	))
	defer span.End()

	reqURL := c.baseURL + path + "?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/fanarttv: rate limiter: %w", err)
		}

		status, err := c.getOnce(ctx, operationName, reqURL, out, span)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxAttempts || !retryableStatus(status) {
			return err
		}

		delay := c.retryBaseDelay * time.Duration(attempt)
		c.logger.WarnContext(ctx, "fanarttv request failed, retrying",
			"operation", operationName, "attempt", attempt, "delay", delay, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// retryableStatus reports whether status is worth another attempt: 0 (a
// transport-level failure — timeout, connection error) or 503. Anything
// else — 404, 401, another non-2xx, a malformed body — is not: retrying
// wouldn't change the outcome. Mirrors musicbrainz.go's/theporndb.go's
// identical function.
func retryableStatus(status int) bool {
	return status == 0 || status == http.StatusServiceUnavailable
}

// getOnce issues a single GET attempt against reqURL and decodes the JSON
// response into out. Returns the HTTP status actually received (0 if the
// request never got a response at all) so get's retry loop can classify
// the failure via retryableStatus.
func (c *Client) getOnce(ctx context.Context, operationName, reqURL string, out any, span trace.Span) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("adapters/fanarttv: building request for %s: %w", operationName, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/fanarttv: requesting %s: %w", operationName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("fanarttv.operation", operationName),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		// Not confirmed live (see get's doc comment) — kept as defensive
		// symmetry with the rest of this package family.
		c.logger.DebugContext(ctx, "fanarttv not found", "operation", operationName)
		return resp.StatusCode, fmt.Errorf("adapters/fanarttv: %s: %w", operationName, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/fanarttv: %s: unexpected status %d", operationName, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/fanarttv: decoding %s response: %w", operationName, err)
	}

	c.logger.DebugContext(ctx, "fanarttv request succeeded", "operation", operationName)
	return resp.StatusCode, nil
}

// Option customizes a Client constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	baseTransport  http.RoundTripper
	retryBaseDelay time.Duration
	rateLimit      rate.Limit
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
		retryBaseDelay: 2 * time.Second,
		rateLimit:      rate.Limit(requestsPerSecond),
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

// WithRetryBaseDelay overrides the base delay get waits before retrying a
// transient failure — attempt N's delay is this value times N. Defaults to
// 2s; tests override this to a near-zero value so a retry-path test
// doesn't have to actually sleep for several real seconds.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(o *options) { o.retryBaseDelay = d }
}

// WithRateLimit overrides the requests-per-second limit get's rate
// limiter enforces — purely a test escape hatch, same as every other
// adapter in this package family. rate.Inf disables limiting entirely.
func WithRateLimit(limit rate.Limit) Option {
	return func(o *options) { o.rateLimit = limit }
}

// WithBaseTransport overrides the transport pkg/httpclient.New builds from
// cfg.HTTPClient with rt, forwarded via httpclient.WithBaseTransport —
// instrumentation and the caching transport still wrap it, only the actual
// socket is replaced. Used to run this Client against a canned
// pkg/httpclient/httpmock.Transport instead of a live network call.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.baseTransport = rt }
}
