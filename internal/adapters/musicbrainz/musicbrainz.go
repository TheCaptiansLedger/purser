// Package musicbrainz is the single MusicBrainz adapter implementing
// ports.MusicBrainzClient. See
// docs/technical/music-musicbrainz-adapter.md and
// docs/adr/0025-music-identification-confidence-scoring.md.
package musicbrainz

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
	instrumentationName = "purser/internal/adapters/musicbrainz"

	// defaultBaseURL is MusicBrainz's public API root. Overridable via
	// Config.BaseURL — e.g. for a self-hosted mirror, or to point at a
	// fixture server in tests.
	defaultBaseURL = "https://musicbrainz.org/ws/2/"

	// requestsPerSecond is MusicBrainz's own enforced rate limit — not
	// operator-configurable (no config.MusicBrainz field), since it's the
	// provider's policy, not a tuning knob. WithRateLimit exists as a
	// package-level Option purely for tests: multi-request test scenarios
	// (e.g. get's retry-on-503 tests) otherwise pay a real ~1s wait per
	// request even though every one of them is fully mocked and never
	// touches a real socket — the point of those tests is the retry
	// *logic*, not actually re-proving the rate limiter's own pacing
	// (TestClient_RateLimiterSerializesConcurrentRequests already does
	// that, deliberately, against the real limit).
	requestsPerSecond = 1

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error like a timeout) in get — see get's retry
	// loop and docs/technical/music-musicbrainz-adapter.md.
	// MusicBrainz's own rate-limiting docs describe 503 as
	// "declined until the rate drops again," not a permanent failure, so
	// a well-behaved client backs off and retries rather than failing
	// the whole group over one transient response.
	maxAttempts = 4
)

// Config configures a Client built by New. Every field has a sane default
// via DefaultConfig — callers only need to override what differs for
// their instance. Field names/tags follow the PURSER_<NESTED>_<KEY>
// convention documented in ADR 0010 for future Viper embedding.
type Config struct {
	// BaseURL is the MusicBrainz API root every request is issued
	// against, trailing slash included.
	BaseURL string `mapstructure:"base_url"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value MusicBrainz's policy requires,
	// regardless of what's set here.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns the sane defaults every Client starts from. The
// cache's DefaultTTL is set well above pkg/cache's own 15-minute default —
// MusicBrainz's identity-graph data is close to static.
//
// ResponseHeaderTimeout is raised well above pkg/httpclient's generic 10s
// default — a real, uncached release/artist lookup with a heavy
// inc=recordings+labels+release-groups can legitimately take 8-10s+ on a
// cold request against musicbrainz.org itself (confirmed directly with
// repeated curl timings during M11b's manual verification, purser#522: a
// cold lookup ran 3.7-8s+, the identical URL requested again moments later
// ran 0.7s). 10s cuts that off mid-flight on a fair fraction of first
// attempts; config.MusicBrainz.ResponseHeaderTimeout lets an operator
// raise it further still without a code change.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	httpCfg := httpclient.DefaultConfig()
	httpCfg.ResponseHeaderTimeout = 25 * time.Second

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpCfg,
		Cache:      cacheCfg,
	}
}

// Client is the MusicBrainz ports.MusicBrainzClient adapter. Safe for
// concurrent use.
type Client struct {
	baseURL        string
	http           *http.Client
	cache          cache.Cache
	limiter        *rate.Limiter
	retryBaseDelay time.Duration

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.MusicBrainzClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and enforces MusicBrainz's fixed User-Agent policy
// regardless of cfg.HTTPClient.UserAgent.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/musicbrainz: BaseURL must not be empty")
	}
	if !strings.HasSuffix(cfg.BaseURL, "/") {
		cfg.BaseURL += "/"
	}
	// Fixed by MusicBrainz policy, not caller-configurable — see
	// docs/technical/music-musicbrainz-adapter.md.
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
		return nil, fmt.Errorf("adapters/musicbrainz: building http client: %w", err)
	}

	c, err := memory.New("musicbrainz", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/musicbrainz: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/musicbrainz: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("musicbrainz.requests", metric.WithDescription("MusicBrainz API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/musicbrainz: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.musicbrainz"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
		requests:       requests,
	}, nil
}

// Close releases the Client's own cache instance.
func (c *Client) Close() error {
	return c.cache.Close()
}

// LookupArtist implements ports.MusicBrainzClient.
func (c *Client) LookupArtist(ctx context.Context, mbid string) (*ports.Artist, error) {
	q := url.Values{}
	q.Set("inc", "url-rels aliases")

	var a ports.Artist
	if err := c.get(ctx, "artist/"+mbid, q, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// SearchArtists implements ports.MusicBrainzClient.
func (c *Client) SearchArtists(ctx context.Context, query string) ([]ports.Artist, error) {
	q := url.Values{}
	q.Set("query", query)

	var resp struct {
		Artists []ports.Artist `json:"artists"`
	}
	if err := c.get(ctx, "artist", q, &resp); err != nil {
		return nil, err
	}
	return resp.Artists, nil
}

// LookupReleaseGroup implements ports.MusicBrainzClient.
func (c *Client) LookupReleaseGroup(ctx context.Context, mbid string) (*ports.ReleaseGroup, error) {
	var rg ports.ReleaseGroup
	if err := c.get(ctx, "release-group/"+mbid, nil, &rg); err != nil {
		return nil, err
	}
	return &rg, nil
}

// ListReleaseGroupsForArtist implements ports.MusicBrainzClient.
func (c *Client) ListReleaseGroupsForArtist(ctx context.Context, artistMBID string) ([]ports.ReleaseGroup, error) {
	q := url.Values{}
	q.Set("artist", artistMBID)
	q.Set("limit", "100")

	var resp struct {
		ReleaseGroups []ports.ReleaseGroup `json:"release-groups"`
	}
	if err := c.get(ctx, "release-group", q, &resp); err != nil {
		return nil, err
	}
	return resp.ReleaseGroups, nil
}

// SearchReleaseGroups implements ports.MusicBrainzClient.
func (c *Client) SearchReleaseGroups(ctx context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error) {
	q := url.Values{}
	q.Set("query", fmt.Sprintf(`artist:%q AND releasegroup:%q`, artistName, albumName))

	var resp struct {
		ReleaseGroups []ports.ReleaseGroup `json:"release-groups"`
	}
	if err := c.get(ctx, "release-group", q, &resp); err != nil {
		return nil, err
	}
	return resp.ReleaseGroups, nil
}

// LookupRelease implements ports.MusicBrainzClient. inc must list every
// sub-resource this codebase actually reads off the result — MusicBrainz
// omits each one from the response entirely unless explicitly requested,
// it doesn't just default them empty. artist-credits and isrcs were
// missing here until purser#522's manual verification caught it: every
// real (non-fixture) Persist call was failing on ports.Release.
// ArtistCredit being empty, 100% reproducibly — confirmed directly
// against musicbrainz.org (identical URL, artist-credit present with the
// inc token, absent without it). recordings/labels/release-groups feed
// Track.Recording, LabelInfo, and ReleaseGroup respectively (see
// adapters/pipeline/music/persister.go and identifier.go's readers of
// this DTO).
func (c *Client) LookupRelease(ctx context.Context, mbid string) (*ports.Release, error) {
	q := url.Values{}
	q.Set("inc", "recordings labels release-groups artist-credits isrcs")

	var r ports.Release
	if err := c.get(ctx, "release/"+mbid, q, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListReleasesForReleaseGroup implements ports.MusicBrainzClient.
func (c *Client) ListReleasesForReleaseGroup(ctx context.Context, rgMBID string) ([]ports.Release, error) {
	q := url.Values{}
	q.Set("release-group", rgMBID)
	// artist-credits added for MusicBrainzService's edition-search RPC
	// (proto/purser/music/v1/musicbrainz_search.proto) — every returned
	// Release.ArtistCredit came back empty without it, the exact same
	// missing-inc= shape LookupRelease had (purser#522). identifier.go's
	// own internal use of this method (releaseTrackCountSane) never
	// needed ArtistCredit, so this was invisible until a second consumer
	// actually wanted it.
	q.Set("inc", "labels media artist-credits")
	q.Set("limit", "100")

	var resp struct {
		Releases []ports.Release `json:"releases"`
	}
	if err := c.get(ctx, "release", q, &resp); err != nil {
		return nil, err
	}
	return resp.Releases, nil
}

// SearchReleaseByBarcode implements ports.MusicBrainzClient.
func (c *Client) SearchReleaseByBarcode(ctx context.Context, barcode string) ([]ports.Release, error) {
	q := url.Values{}
	q.Set("query", "barcode:"+barcode)

	var resp struct {
		Releases []ports.Release `json:"releases"`
	}
	if err := c.get(ctx, "release", q, &resp); err != nil {
		return nil, err
	}
	return resp.Releases, nil
}

// LookupRecordingByISRC implements ports.MusicBrainzClient.
func (c *Client) LookupRecordingByISRC(ctx context.Context, isrc string) ([]ports.Recording, error) {
	q := url.Values{}
	q.Set("inc", "releases artist-credits")

	var resp struct {
		Recordings []ports.Recording `json:"recordings"`
	}
	if err := c.get(ctx, "isrc/"+isrc, q, &resp); err != nil {
		return nil, err
	}
	return resp.Recordings, nil
}

// get issues a rate-limited GET against path (relative to baseURL) with
// query params q, decodes the JSON response into out, and maps a 404 to
// ports.ErrNotFound — the single choke point every method above routes
// through, so rate limiting, tracing, metrics, and error mapping are
// implemented exactly once.
//
// A transient failure — HTTP 503 (MusicBrainz's own documented
// rate-limit response: "declined until the rate drops again," not a
// permanent failure — see docs/technical/music-musicbrainz-adapter.md)
// or a transport-level error (a slow/cold request timing out, confirmed
// against the real API during purser#522's manual verification: a cold,
// heavy release lookup legitimately took 8-10s+ on a first attempt, then
// under a second moments later) — is retried up to maxAttempts times
// with an increasing delay, still paced through the limiter on every
// attempt so a retry storm can't itself violate the rate limit.
// Anything else (404, another non-2xx, a malformed body) fails
// immediately — retrying those wouldn't change the outcome.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("fmt", "json")

	ctx, span := c.tracer.Start(ctx, "musicbrainz.get", trace.WithAttributes(
		attribute.String("musicbrainz.path", path),
	))
	defer span.End()

	reqURL := c.baseURL + path + "?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/musicbrainz: rate limiter: %w", err)
		}

		status, err := c.getOnce(ctx, path, reqURL, out, span)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxAttempts || !retryableStatus(status) {
			return err
		}

		delay := c.retryBaseDelay * time.Duration(attempt)
		c.logger.WarnContext(ctx, "musicbrainz request failed, retrying",
			"path", path, "attempt", attempt, "delay", delay, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// retryableStatus reports whether status is worth another attempt: 0 (a
// transport-level failure that never got an HTTP response at all — a
// timeout, a connection error) or MusicBrainz's own documented
// rate-limit signal, 503. Anything else — 404, another non-2xx, a
// successfully-received but malformed body (which still carries its
// real 2xx status here) — is not: retrying wouldn't change the outcome.
func retryableStatus(status int) bool {
	return status == 0 || status == http.StatusServiceUnavailable
}

// getOnce issues a single GET attempt against reqURL, decodes the JSON
// response into out, and maps a 404 to ports.ErrNotFound. Returns the
// HTTP status actually received (0 if the request never got a response
// at all) so get's retry loop can classify the failure via
// retryableStatus.
func (c *Client) getOnce(ctx context.Context, path, reqURL string, out any, span trace.Span) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("adapters/musicbrainz: building request for %s: %w", path, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/musicbrainz: requesting %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("musicbrainz.path", path),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		c.logger.DebugContext(ctx, "musicbrainz not found", "path", path)
		return resp.StatusCode, fmt.Errorf("adapters/musicbrainz: %s: %w", path, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/musicbrainz: %s: unexpected status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/musicbrainz: decoding %s response: %w", path, err)
	}

	c.logger.DebugContext(ctx, "musicbrainz request succeeded", "path", path)
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
// transient failure (HTTP 503, or a transport-level error) — attempt N's
// delay is this value times N. Defaults to 2s; tests override this to a
// near-zero value so a retry-path test doesn't have to actually sleep for
// several real seconds.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(o *options) { o.retryBaseDelay = d }
}

// WithRateLimit overrides the requests-per-second limit get's rate
// limiter enforces — see requestsPerSecond's own doc comment for why this
// exists (purely a test escape hatch, never operator-configurable in
// production). rate.Inf disables limiting entirely, the common case for a
// test that isn't itself about pacing.
func WithRateLimit(limit rate.Limit) Option {
	return func(o *options) { o.rateLimit = limit }
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
