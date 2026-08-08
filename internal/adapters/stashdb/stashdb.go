// Package stashdb is the single StashDB adapter implementing
// ports.StashDBClient. See docs/adr/0027-provider-independence.md.
//
// StashDB runs stash-box's GraphQL API (https://github.com/stashapp/stash-box)
// at https://stashdb.org/graphql. Every operation here is a read-only
// GraphQL query issued as an HTTP GET — query and variables encoded as URL
// query parameters — rather than the more common POST-with-JSON-body
// convention. This was confirmed directly against the live API (GET
// requests resolve identically to POST) specifically so requests flow
// through pkg/httpclient's caching transport unmodified: that transport
// only caches GET/HEAD (see pkg/httpclient/cachingtransport.go), the same
// requirement AcoustID's adapter satisfies by issuing its lookup as a GET.
// Authentication is the "ApiKey" request header (not a bearer token, not a
// query param) — confirmed against the real API, which answers an
// unauthenticated query with a GraphQL-level "not authorized" error rather
// than an HTTP 401/403.
package stashdb

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
	instrumentationName = "purser/internal/adapters/stashdb"

	// defaultBaseURL is StashDB's public GraphQL endpoint. Overridable via
	// Config.BaseURL — e.g. to point at a fixture server in tests.
	defaultBaseURL = "https://stashdb.org/graphql"

	// requestsPerSecond is a conservative, unverified placeholder — unlike
	// MusicBrainz's 1 req/sec (a documented provider policy), StashDB
	// publishes no fixed rate limit. Not exposed via Config for the same
	// reason MusicBrainz's isn't: it's a courtesy default, not something
	// an operator should be tuning against a stated policy. WithRateLimit
	// exists purely as a test escape hatch, same as musicbrainz's.
	requestsPerSecond = 3

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error) in doQuery — see doQuery's retry loop and
	// musicbrainz.go's identical convention.
	maxAttempts = 4
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig except APIKey, which has no safe default and must be
// supplied by the caller — StashDB requires the ApiKey header on every
// query. Field names/tags follow the PURSER_<NESTED>_<KEY> convention
// documented in ADR 0010 for future Viper embedding.
type Config struct {
	// BaseURL is StashDB's GraphQL endpoint every query is issued against.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "ApiKey" header on every request. Required;
	// New returns an error if empty.
	APIKey string `mapstructure:"api_key"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as the
	// MusicBrainz/AcoustID adapters.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// APIKey which callers must supply themselves. The cache's DefaultTTL
// mirrors MusicBrainz's: StashDB's performer/studio/scene identity data is
// close to static.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the StashDB ports.StashDBClient adapter. Safe for concurrent
// use.
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

var _ ports.StashDBClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as the MusicBrainz/AcoustID adapters.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/stashdb: BaseURL must not be empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/stashdb: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/stashdb: building http client: %w", err)
	}

	c, err := memory.New("stashdb", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/stashdb: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/stashdb: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("stashdb.requests", metric.WithDescription("StashDB GraphQL requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/stashdb: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		apiKey:         cfg.APIKey,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.stashdb"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
		requests:       requests,
	}, nil
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// LookupPerformer implements ports.StashDBClient.
func (c *Client) LookupPerformer(ctx context.Context, id string) (*ports.Performer, error) {
	var resp struct {
		FindPerformer *ports.Performer `json:"findPerformer"`
	}
	if err := c.doQuery(ctx, "FindPerformer", queryFindPerformer, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindPerformer == nil {
		return nil, fmt.Errorf("adapters/stashdb: findPerformer %s: %w", id, ports.ErrNotFound)
	}
	return resp.FindPerformer, nil
}

// SearchPerformers implements ports.StashDBClient.
func (c *Client) SearchPerformers(ctx context.Context, term string) ([]ports.Performer, error) {
	var resp struct {
		SearchPerformer []ports.Performer `json:"searchPerformer"`
	}
	if err := c.doQuery(ctx, "SearchPerformer", querySearchPerformer, map[string]any{"term": term}, &resp); err != nil {
		return nil, err
	}
	return resp.SearchPerformer, nil
}

// LookupStudio implements ports.StashDBClient.
func (c *Client) LookupStudio(ctx context.Context, id string) (*ports.Studio, error) {
	var resp struct {
		FindStudio *ports.Studio `json:"findStudio"`
	}
	if err := c.doQuery(ctx, "FindStudio", queryFindStudio, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindStudio == nil {
		return nil, fmt.Errorf("adapters/stashdb: findStudio %s: %w", id, ports.ErrNotFound)
	}
	return resp.FindStudio, nil
}

// LookupScene implements ports.StashDBClient.
func (c *Client) LookupScene(ctx context.Context, id string) (*ports.Scene, error) {
	var resp struct {
		FindScene *ports.Scene `json:"findScene"`
	}
	if err := c.doQuery(ctx, "FindScene", queryFindScene, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindScene == nil {
		return nil, fmt.Errorf("adapters/stashdb: findScene %s: %w", id, ports.ErrNotFound)
	}
	return resp.FindScene, nil
}

// SearchScenes implements ports.StashDBClient.
func (c *Client) SearchScenes(ctx context.Context, term string) ([]ports.Scene, error) {
	var resp struct {
		SearchScene []ports.Scene `json:"searchScene"`
	}
	if err := c.doQuery(ctx, "SearchScene", querySearchScene, map[string]any{"term": term}, &resp); err != nil {
		return nil, err
	}
	return resp.SearchScene, nil
}

// FindScenesByFingerprints implements ports.StashDBClient. StashDB's
// findScenesBySceneFingerprints is batch-shaped ([[input]] -> [[Scene]]);
// this method always sends a single-file batch (one inner list) and
// returns that single result set — see the port doc comment.
func (c *Client) FindScenesByFingerprints(ctx context.Context, fingerprints []ports.SceneFingerprint) ([]ports.Scene, error) {
	var resp struct {
		FindScenesBySceneFingerprints [][]ports.Scene `json:"findScenesBySceneFingerprints"`
	}
	vars := map[string]any{"fingerprints": [][]ports.SceneFingerprint{fingerprints}}
	if err := c.doQuery(ctx, "FindScenesBySceneFingerprints", queryFindScenesBySceneFingerprints, vars, &resp); err != nil {
		return nil, err
	}
	if len(resp.FindScenesBySceneFingerprints) == 0 {
		return nil, nil
	}
	return resp.FindScenesBySceneFingerprints[0], nil
}

// graphqlResponse is the GraphQL-over-HTTP envelope every StashDB response
// carries, per https://graphql.org/learn/serving-over-http/#response.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
}

// doQuery issues a rate-limited GraphQL query (as an HTTP GET — see the
// package doc comment) with the given variables, decodes the "data" field
// of the response into out, and surfaces any "errors" entries as a Go
// error. This is the single choke point every method above routes
// through, so rate limiting, tracing, metrics, and error mapping are
// implemented exactly once — mirrors musicbrainz.go's get().
//
// A GraphQL-level "errors" entry (auth failure, malformed query/ID, etc.)
// is always a real failure, never StashDB's not-found signal: an
// individual Lookup* method's own null-result check
// (e.g. LookupPerformer's resp.FindPerformer == nil) is what maps to
// ports.ErrNotFound, exactly mirroring how musicbrainz.go's get() maps an
// HTTP 404 — GraphQL has no transport-level equivalent, a resolver simply
// returns null.
//
// A transient failure (HTTP 503, or a transport-level error) is retried up
// to maxAttempts times with an increasing delay, still paced through the
// limiter on every attempt. Anything else fails immediately.
func (c *Client) doQuery(ctx context.Context, operationName, query string, variables map[string]any, out any) error {
	ctx, span := c.tracer.Start(ctx, "stashdb."+operationName, trace.WithAttributes(
		attribute.String("stashdb.operation", operationName),
	))
	defer span.End()

	q := url.Values{}
	q.Set("query", query)
	if len(variables) > 0 {
		varsJSON, err := json.Marshal(variables)
		if err != nil {
			return fmt.Errorf("adapters/stashdb: marshaling variables for %s: %w", operationName, err)
		}
		q.Set("variables", string(varsJSON))
	}
	reqURL := c.baseURL + "?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/stashdb: rate limiter: %w", err)
		}

		status, err := c.doQueryOnce(ctx, operationName, reqURL, out, span)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxAttempts || !retryableStatus(status) {
			return err
		}

		delay := c.retryBaseDelay * time.Duration(attempt)
		c.logger.WarnContext(ctx, "stashdb request failed, retrying",
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
// else — a GraphQL-level "errors" entry (still carried on a 200), another
// non-2xx, a malformed body — is not: retrying wouldn't change the
// outcome. Mirrors musicbrainz.go's identical function.
func retryableStatus(status int) bool {
	return status == 0 || status == http.StatusServiceUnavailable
}

// doQueryOnce issues a single GET attempt against reqURL, decodes the
// GraphQL envelope, and unmarshals its "data" field into out. Returns the
// HTTP status actually received (0 if the request never got a response at
// all) so doQuery's retry loop can classify the failure via
// retryableStatus.
func (c *Client) doQueryOnce(ctx context.Context, operationName, reqURL string, out any, span trace.Span) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("adapters/stashdb: building request for %s: %w", operationName, err)
	}
	req.Header.Set("ApiKey", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/stashdb: requesting %s: %w", operationName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("stashdb.operation", operationName),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/stashdb: %s: unexpected status %d", operationName, resp.StatusCode)
	}

	var gr graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/stashdb: decoding %s response: %w", operationName, err)
	}
	if len(gr.Errors) > 0 {
		msgs := make([]string, len(gr.Errors))
		for i, e := range gr.Errors {
			msgs[i] = e.Message
		}
		return resp.StatusCode, fmt.Errorf("adapters/stashdb: %s: %s", operationName, strings.Join(msgs, "; "))
	}
	if len(gr.Data) > 0 && string(gr.Data) != "null" {
		if err := json.Unmarshal(gr.Data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("adapters/stashdb: decoding %s data: %w", operationName, err)
		}
	}

	c.logger.DebugContext(ctx, "stashdb request succeeded", "operation", operationName)
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

// WithRetryBaseDelay overrides the base delay doQuery waits before
// retrying a transient failure — attempt N's delay is this value times N.
// Defaults to 2s; tests override this to a near-zero value so a
// retry-path test doesn't have to actually sleep for several real seconds.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(o *options) { o.retryBaseDelay = d }
}

// WithRateLimit overrides the requests-per-second limit doQuery's rate
// limiter enforces — purely a test escape hatch, same as
// musicbrainz.WithRateLimit. rate.Inf disables limiting entirely.
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
