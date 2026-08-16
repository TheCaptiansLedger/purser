// Package theaudiodb is the single TheAudioDB adapter implementing
// ports.TheAudioDBClient. See docs/adr/0027-provider-independence.md.
//
// TheAudioDB (theaudiodb.com) runs a REST API at
// https://www.theaudiodb.com/api/v1/json/{key}/ — the API key is a URL
// path segment, not a header or query param, confirmed live against the
// real API during this adapter's implementation:
//
//   - GET {key}/artist-mb.php?i={mbid} — {"artists": [{...}]} for a known
//     MBID, {"artists": null} for an unknown one. Always HTTP 200 either
//     way — TheAudioDB never answers an unknown MBID with an HTTP error
//     status, unlike ThePornDB/fanart.tv's 404/empty-body conventions.
//   - GET {key}/album-mb.php?i={releaseGroupMBID} — {"album": [{...}]} /
//     {"album": null}, same always-200 shape.
//
// The free-tier key "123" still works today (confirmed live, independent
// of this environment's own personal key) and returns identical data —
// docs/technical/music-data_model.md's research pass already noted this.
package theaudiodb

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
	instrumentationName = "purser/internal/adapters/theaudiodb"

	// defaultBaseURL is TheAudioDB's real API root, trailing slash
	// included — the API key is appended as a path segment after this
	// (see New). Overridable via Config.BaseURL, e.g. to point at a
	// fixture server in tests.
	defaultBaseURL = "https://www.theaudiodb.com/api/v1/json/"

	// requestsPerSecond is a conservative, unverified placeholder — no
	// published rate-limit headers were observed on any live response
	// during this adapter's implementation, the same situation
	// theporndb.go's identical constant documents. WithRateLimit exists
	// purely as a test escape hatch, same as every other adapter in this
	// package family.
	requestsPerSecond = 2

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error) in get — see get's retry loop and
	// musicbrainz.go's/theporndb.go's identical convention.
	maxAttempts = 4
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig except APIKey, which has no safe default and must be
// supplied by the caller — TheAudioDB requires the key on every request
// path. Field names/tags follow the PURSER_<NESTED>_<KEY> convention
// documented in ADR 0010 for future Viper embedding.
type Config struct {
	// BaseURL is TheAudioDB's API root every request is issued against,
	// trailing slash included. The API key is appended as its own path
	// segment after this.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is appended as a URL path segment on every request (e.g.
	// .../json/{APIKey}/artist-mb.php). Required; New returns an error if
	// empty.
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
// mirrors the rest of this package family's: TheAudioDB's artist/album
// identity data is close to static.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the TheAudioDB ports.TheAudioDBClient adapter. Safe for
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

var _ ports.TheAudioDBClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as every other adapter in this package
// family.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/theaudiodb: BaseURL must not be empty")
	}
	if !strings.HasSuffix(cfg.BaseURL, "/") {
		cfg.BaseURL += "/"
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/theaudiodb: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/theaudiodb: building http client: %w", err)
	}

	c, err := memory.New("theaudiodb", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/theaudiodb: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/theaudiodb: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("theaudiodb.requests", metric.WithDescription("TheAudioDB API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/theaudiodb: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		apiKey:         cfg.APIKey,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.theaudiodb"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
		requests:       requests,
	}, nil
}

// Cache returns the Client's own named cache.Cache instance, so the
// composition root can register it in a cache instance registry (see
// internal/ports.CacheRegistry) for the administration UI to report on.
func (c *Client) Cache() cache.Cache {
	return c.cache
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// LookupArtist implements ports.TheAudioDBClient.
func (c *Client) LookupArtist(ctx context.Context, mbid string) (*ports.TADBArtist, error) {
	q := url.Values{}
	q.Set("i", mbid)

	var resp struct {
		Artists []ports.TADBArtist `json:"artists"`
	}
	if err := c.get(ctx, "LookupArtist", "artist-mb.php", q, &resp); err != nil {
		return nil, err
	}
	if len(resp.Artists) == 0 {
		return nil, fmt.Errorf("adapters/theaudiodb: LookupArtist %s: %w", mbid, ports.ErrNotFound)
	}
	return &resp.Artists[0], nil
}

// LookupAlbum implements ports.TheAudioDBClient.
func (c *Client) LookupAlbum(ctx context.Context, releaseGroupMBID string) (*ports.TADBAlbum, error) {
	q := url.Values{}
	q.Set("i", releaseGroupMBID)

	var resp struct {
		Album []ports.TADBAlbum `json:"album"`
	}
	if err := c.get(ctx, "LookupAlbum", "album-mb.php", q, &resp); err != nil {
		return nil, err
	}
	if len(resp.Album) == 0 {
		return nil, fmt.Errorf("adapters/theaudiodb: LookupAlbum %s: %w", releaseGroupMBID, ports.ErrNotFound)
	}
	return &resp.Album[0], nil
}

// get issues a rate-limited GET against baseURL/apiKey/path (the API key
// is a URL path segment, not a header/query param — see the package doc
// comment) with query params q, decodes the JSON response into out, and
// maps a non-2xx status to a real error — the single choke point every
// method above routes through, so rate limiting, tracing, metrics, and
// error mapping are implemented exactly once. Note that TheAudioDB's own
// "not found" signal is a null field inside a 200 response, not a status
// code — LookupArtist/LookupAlbum handle that themselves after decoding;
// this method's own 404 mapping is defensive symmetry with the rest of
// this package family, not a case confirmed live against the real API.
//
// A transient failure (HTTP 503, or a transport-level error) is retried up
// to maxAttempts times with an increasing delay, still paced through the
// limiter on every attempt so a retry storm can't itself violate the rate
// limit. Anything else fails immediately.
func (c *Client) get(ctx context.Context, operationName, path string, q url.Values, out any) error {
	ctx, span := c.tracer.Start(ctx, "theaudiodb."+operationName, trace.WithAttributes(
		attribute.String("theaudiodb.path", path),
	))
	defer span.End()

	reqURL := c.baseURL + c.apiKey + "/" + path
	if len(q) > 0 {
		reqURL += "?" + q.Encode()
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/theaudiodb: rate limiter: %w", err)
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
		c.logger.WarnContext(ctx, "theaudiodb request failed, retrying",
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
// else — 404, another non-2xx, a malformed body — is not: retrying
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
		return 0, fmt.Errorf("adapters/theaudiodb: building request for %s: %w", operationName, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/theaudiodb: requesting %s: %w", operationName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("theaudiodb.operation", operationName),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		// Not confirmed live (see get's doc comment) — kept as defensive
		// symmetry with the rest of this package family.
		c.logger.DebugContext(ctx, "theaudiodb not found", "operation", operationName)
		return resp.StatusCode, fmt.Errorf("adapters/theaudiodb: %s: %w", operationName, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/theaudiodb: %s: unexpected status %d", operationName, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/theaudiodb: decoding %s response: %w", operationName, err)
	}

	c.logger.DebugContext(ctx, "theaudiodb request succeeded", "operation", operationName)
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
