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
	// configurable, since it's the provider's policy, not a tuning knob.
	requestsPerSecond = 1
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
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the MusicBrainz ports.MusicBrainzClient adapter. Safe for
// concurrent use.
type Client struct {
	baseURL string
	http    *http.Client
	cache   cache.Cache
	limiter *rate.Limiter

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

	httpClient, err := httpclient.New(cfg.HTTPClient,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
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
		baseURL:  cfg.BaseURL,
		http:     httpClient,
		cache:    c,
		limiter:  rate.NewLimiter(rate.Limit(requestsPerSecond), 1),
		logger:   o.logger.With("component", "adapters.musicbrainz"),
		tracer:   o.tracerProvider.Tracer(instrumentationName),
		requests: requests,
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

// LookupRelease implements ports.MusicBrainzClient.
func (c *Client) LookupRelease(ctx context.Context, mbid string) (*ports.Release, error) {
	q := url.Values{}
	q.Set("inc", "recordings labels release-groups")

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
	q.Set("inc", "labels media")
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
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("fmt", "json")

	ctx, span := c.tracer.Start(ctx, "musicbrainz.get", trace.WithAttributes(
		attribute.String("musicbrainz.path", path),
	))
	defer span.End()

	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("adapters/musicbrainz: rate limiter: %w", err)
	}

	reqURL := c.baseURL + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("adapters/musicbrainz: building request for %s: %w", path, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("adapters/musicbrainz: requesting %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("musicbrainz.path", path),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		c.logger.DebugContext(ctx, "musicbrainz not found", "path", path)
		return fmt.Errorf("adapters/musicbrainz: %s: %w", path, ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("adapters/musicbrainz: %s: unexpected status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("adapters/musicbrainz: decoding %s response: %w", path, err)
	}

	c.logger.DebugContext(ctx, "musicbrainz request succeeded", "path", path)
	return nil
}

// Option customizes a Client constructed via New.
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
