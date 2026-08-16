// Package theporndb is the single ThePornDB adapter implementing
// ports.ThePornDBClient. See docs/adr/0027-provider-independence.md.
//
// ThePornDB (theporndb.net) runs a REST API at https://api.theporndb.net —
// note this differs from theporndb.net/api, which returns 401 regardless
// of key validity (confirmed during docs/technical/afterdark-data_model.md's
// research pass, and re-confirmed directly against the real API during this
// adapter's implementation). Every endpoint below was hit live against the
// real API (using the key already present in this environment's .env) to
// confirm its exact shape before being coded, not assumed from the research
// doc alone:
//
//   - GET /performers/{id}, GET /scenes/{id} — single-resource lookups,
//     wrapped in a {"data": {...}} envelope, 404 with
//     {"message": "performer not found"} / {"message": "scene not found"}
//     for an unknown ID.
//   - GET /performers?q=, GET /scenes?q= — free-text search, wrapped in a
//     {"data": [...], "links": {...}, "meta": {...}} paginated envelope,
//     200 with an empty "data" array for no matches (never a 404).
//   - GET /scenes/hash/{hash} — dedicated single-scene hash lookup
//     (distinct from the list-shaped GET /scenes?hash= search this adapter
//     does not use), same {"data": {...}} envelope, 404 with
//     {"message": "hash not found"} for an unrecognized hash.
//   - GET /jav?parse={code} — same paginated list envelope as search, but
//     is NOT a single-match resolver: confirmed live, it returns ThePornDB's
//     own ranked list of candidate scenes (best match first, tolerant of an
//     imperfectly parsed code), not just the exact match. This adapter
//     returns that list exactly as received — see
//     ports.ThePornDBClient.ResolveJAVCode's doc comment.
//
// Auth is "Authorization: Bearer {token}" — confirmed live, standard bearer
// auth (unlike StashDB's "ApiKey" header).
package theporndb

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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	instrumentationName = "purser/internal/adapters/theporndb"

	// defaultBaseURL is ThePornDB's real API root — deliberately
	// api.theporndb.net, not theporndb.net/api (see the package doc
	// comment). Overridable via Config.BaseURL, e.g. to point at a fixture
	// server in tests.
	defaultBaseURL = "https://api.theporndb.net"

	// requestsPerSecond is a conservative, unverified placeholder — no
	// published rate-limit headers were observed on any live response
	// during this adapter's implementation, the same situation
	// stashdb.go's identical constant documents. WithRateLimit exists
	// purely as a test escape hatch, same as stashdb's/musicbrainz's.
	requestsPerSecond = 3

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error) in doRequest — see doRequest's retry loop and
	// musicbrainz.go's/stashdb.go's identical convention.
	maxAttempts = 4
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig except APIKey, which has no safe default and must be
// supplied by the caller — ThePornDB requires the Bearer token on every
// request. Field names/tags follow the PURSER_<NESTED>_<KEY> convention
// documented in ADR 0010 for future Viper embedding.
type Config struct {
	// BaseURL is ThePornDB's API root every request is issued against.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as "Authorization: Bearer {APIKey}" on every request.
	// Required; New returns an error if empty.
	APIKey string `mapstructure:"api_key"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as the
	// MusicBrainz/StashDB adapters.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// APIKey which callers must supply themselves. The cache's DefaultTTL
// mirrors StashDB's: ThePornDB's performer/scene identity data is close to
// static.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the ThePornDB ports.ThePornDBClient adapter. Safe for
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

var _ ports.ThePornDBClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as the MusicBrainz/StashDB adapters.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/theporndb: BaseURL must not be empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/theporndb: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/theporndb: building http client: %w", err)
	}

	c, err := memory.New("theporndb", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/theporndb: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/theporndb: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("theporndb.requests", metric.WithDescription("ThePornDB API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/theporndb: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		apiKey:         cfg.APIKey,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.theporndb"),
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

// tpdbEnvelope is the {"data": ...} wrapper every ThePornDB response
// carries, confirmed live on every endpoint this adapter calls — list
// endpoints additionally carry "links"/"meta" pagination fields this
// adapter doesn't consume, so they're simply left undecoded here.
type tpdbEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// LookupPerformer implements ports.ThePornDBClient.
func (c *Client) LookupPerformer(ctx context.Context, id string) (*ports.TPDBPerformer, error) {
	var p ports.TPDBPerformer
	if err := c.doRequest(ctx, "LookupPerformer", "performers/"+id, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SearchPerformers implements ports.ThePornDBClient.
func (c *Client) SearchPerformers(ctx context.Context, term string) ([]ports.TPDBPerformer, error) {
	q := url.Values{}
	q.Set("q", term)

	performers := []ports.TPDBPerformer{}
	if err := c.doRequest(ctx, "SearchPerformers", "performers", q, &performers); err != nil {
		return nil, err
	}
	return performers, nil
}

// LookupScene implements ports.ThePornDBClient.
func (c *Client) LookupScene(ctx context.Context, id string) (*ports.TPDBScene, error) {
	var s ports.TPDBScene
	if err := c.doRequest(ctx, "LookupScene", "scenes/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SearchScenes implements ports.ThePornDBClient.
func (c *Client) SearchScenes(ctx context.Context, term string) ([]ports.TPDBScene, error) {
	q := url.Values{}
	q.Set("q", term)

	scenes := []ports.TPDBScene{}
	if err := c.doRequest(ctx, "SearchScenes", "scenes", q, &scenes); err != nil {
		return nil, err
	}
	return scenes, nil
}

// LookupSceneByHash implements ports.ThePornDBClient.
func (c *Client) LookupSceneByHash(ctx context.Context, hash string) (*ports.TPDBScene, error) {
	var s ports.TPDBScene
	if err := c.doRequest(ctx, "LookupSceneByHash", "scenes/hash/"+hash, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ResolveJAVCode implements ports.ThePornDBClient.
func (c *Client) ResolveJAVCode(ctx context.Context, code string) ([]ports.TPDBScene, error) {
	q := url.Values{}
	q.Set("parse", code)

	scenes := []ports.TPDBScene{}
	if err := c.doRequest(ctx, "ResolveJAVCode", "jav", q, &scenes); err != nil {
		return nil, err
	}
	return scenes, nil
}

// doRequest issues a rate-limited GET against path (relative to baseURL)
// with query params q, decodes the "data" field of the {"data": ...}
// envelope into out, and maps a 404 to ports.ErrNotFound — the single
// choke point every method above routes through, so rate limiting,
// tracing, metrics, and error mapping are implemented exactly once —
// mirrors musicbrainz.go's get()/stashdb.go's doQuery().
//
// A transient failure (HTTP 503, or a transport-level error) is retried up
// to maxAttempts times with an increasing delay, still paced through the
// limiter on every attempt. Anything else fails immediately.
func (c *Client) doRequest(ctx context.Context, operationName, path string, q url.Values, out any) error {
	ctx, span := c.tracer.Start(ctx, "theporndb."+operationName, trace.WithAttributes(
		attribute.String("theporndb.path", path),
	))
	defer span.End()

	reqURL := c.baseURL + "/" + path
	if len(q) > 0 {
		reqURL += "?" + q.Encode()
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/theporndb: rate limiter: %w", err)
		}

		status, err := c.doRequestOnce(ctx, operationName, reqURL, out, span)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxAttempts || !retryableStatus(status) {
			return err
		}

		delay := c.retryBaseDelay * time.Duration(attempt)
		c.logger.WarnContext(ctx, "theporndb request failed, retrying",
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
// wouldn't change the outcome. Mirrors musicbrainz.go's/stashdb.go's
// identical function.
func retryableStatus(status int) bool {
	return status == 0 || status == http.StatusServiceUnavailable
}

// doRequestOnce issues a single GET attempt against reqURL, decodes the
// {"data": ...} envelope, and maps a 404 to ports.ErrNotFound. Returns the
// HTTP status actually received (0 if the request never got a response at
// all) so doRequest's retry loop can classify the failure via
// retryableStatus.
func (c *Client) doRequestOnce(ctx context.Context, operationName, reqURL string, out any, span trace.Span) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("adapters/theporndb: building request for %s: %w", operationName, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/theporndb: requesting %s: %w", operationName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("theporndb.operation", operationName),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		c.logger.DebugContext(ctx, "theporndb not found", "operation", operationName)
		return resp.StatusCode, fmt.Errorf("adapters/theporndb: %s: %w", operationName, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/theporndb: %s: unexpected status %d", operationName, resp.StatusCode)
	}

	var env tpdbEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/theporndb: decoding %s response: %w", operationName, err)
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("adapters/theporndb: decoding %s data: %w", operationName, err)
		}
	}

	c.logger.DebugContext(ctx, "theporndb request succeeded", "operation", operationName)
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

// WithRetryBaseDelay overrides the base delay doRequest waits before
// retrying a transient failure — attempt N's delay is this value times N.
// Defaults to 2s; tests override this to a near-zero value so a
// retry-path test doesn't have to actually sleep for several real seconds.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(o *options) { o.retryBaseDelay = d }
}

// WithRateLimit overrides the requests-per-second limit doRequest's rate
// limiter enforces — purely a test escape hatch, same as
// stashdb.WithRateLimit/musicbrainz.WithRateLimit. rate.Inf disables
// limiting entirely.
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
