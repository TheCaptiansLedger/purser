// Package prowlarr is the Prowlarr adapter implementing
// ports.IndexerSearcher. See docs/technical/acquisition-indexer-search.md
// and docs/technical/acquisition-pipeline.md — this is an ordinary
// capability-shaped port, not a docs/adr/0027-provider-independence.md
// provider exception, even though (like every adapter in this codebase's
// network-backed family) there is exactly one adapter for it today.
//
// Prowlarr (https://github.com/Prowlarr/Prowlarr) exposes a single search
// endpoint, GET {base_url}/search, authenticated via the "X-Api-Key"
// header (never a query param or bearer token) rather than the GraphQL
// "ApiKey" header StashDB uses or the URL-path-segment key TheAudioDB
// uses. It returns a flat JSON array of release objects — guid, title,
// size, indexerId, indexer, publishDate, downloadUrl, magnetUrl (torrent
// only), infoUrl, infoHash, seeders, leechers, protocol
// ("torrent"/"usenet", decoding straight into the shared ports.Protocol),
// and categories ([]{id, name}) — never a 404 for a zero-result search,
// matching every other Search/List method in this codebase.
//
// Prowlarr is self-hosted only, with no public default instance and no
// published rate-limit policy the way MusicBrainz's 1 req/sec is — unlike
// the rest of this adapter family (musicbrainz/stashdb/theaudiodb), this
// Client deliberately carries no rate limiter, no retry-on-503 loop, and
// no response cache: a search's seeders/leechers/result-set are live data
// an operator is querying against their own box, not a static identity
// lookup against a third party worth being polite to or caching.
package prowlarr

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/httpclient"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/prowlarr"

// Config configures a Client built by New. BaseURL and APIKey have no safe
// default and must be supplied by the caller — Prowlarr has no public
// instance and every request needs the key. Field names/tags follow the
// PURSER_<NESTED>_<KEY> convention documented in ADR 0010 for Viper
// embedding (see config.Prowlarr).
type Config struct {
	// BaseURL is Prowlarr's API root every search is issued against, e.g.
	// "http://prowlarr.local:9696/api/v1" — Search appends "/search"
	// directly to this value. Required; New returns an error if empty.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "X-Api-Key" header on every request.
	// Required; New returns an error if empty.
	APIKey string `mapstructure:"api_key"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as every other
	// adapter in this package family. ResponseHeaderTimeout is raised
	// above pkg/httpclient's generic 10s default by DefaultConfig below —
	// see its doc comment.
	HTTPClient httpclient.Config `mapstructure:"http_client"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// BaseURL and APIKey which callers must supply themselves.
//
// ResponseHeaderTimeout is raised to 25s, above pkg/httpclient's generic
// 10s default — a real Prowlarr search fans out to every enabled indexer
// and waits for the slowest one, unlike a single-provider lookup;
// confirmed directly against a real multi-indexer instance during this
// adapter's implementation (issue #581), where an ordinary broad-term
// search took 9.3s, already cutting it close against the 10s generic
// default. config.Prowlarr.ResponseHeaderTimeout lets an operator raise it
// further still without a code change, the same escape hatch
// config.MusicBrainz.ResponseHeaderTimeout provides for the identical
// situation (purser#522).
func DefaultConfig() Config {
	httpCfg := httpclient.DefaultConfig()
	httpCfg.ResponseHeaderTimeout = 25 * time.Second

	return Config{
		HTTPClient: httpCfg,
	}
}

// Client is the Prowlarr ports.IndexerSearcher adapter. Safe for
// concurrent use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.IndexerSearcher = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) and sets a fixed
// User-Agent regardless of cfg.HTTPClient.UserAgent, same as every other
// adapter in this package family.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/prowlarr: BaseURL must not be empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/prowlarr: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/prowlarr: building http client: %w", err)
	}

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("prowlarr.requests", metric.WithDescription("Prowlarr API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/prowlarr: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:  strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		http:     httpClient,
		logger:   o.logger.With("component", "adapters.prowlarr"),
		tracer:   o.tracerProvider.Tracer(instrumentationName),
		requests: requests,
	}, nil
}

// prowlarrRelease is Prowlarr's own wire shape for one /search result —
// see the package doc comment for field-by-field provenance. Decoded
// separately from ports.IndexerRelease (rather than decoding straight into
// the port DTO) so this adapter's JSON tags can match Prowlarr's real API
// exactly, independent of whatever field names/tags the port itself
// carries.
type prowlarrRelease struct {
	GUID        string             `json:"guid"`
	Title       string             `json:"title"`
	Size        int64              `json:"size"`
	IndexerID   int                `json:"indexerId"`
	Indexer     string             `json:"indexer"`
	PublishDate string             `json:"publishDate"`
	DownloadURL string             `json:"downloadUrl"`
	MagnetURL   string             `json:"magnetUrl"`
	InfoURL     string             `json:"infoUrl"`
	InfoHash    string             `json:"infoHash"`
	Seeders     int                `json:"seeders"`
	Leechers    int                `json:"leechers"`
	Protocol    ports.Protocol     `json:"protocol"`
	Categories  []prowlarrCategory `json:"categories"`
}

type prowlarrCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// parsePublishDate parses Prowlarr's RFC3339 publishDate, degrading to the
// zero time.Time on anything malformed or empty rather than failing the
// whole search over a display field.
func parsePublishDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// toIndexerRelease translates r into the provider-neutral ports.IndexerRelease.
// PublishDate is parsed best-effort (RFC3339, Prowlarr's own format) —
// a malformed/absent date degrades to the zero time.Time rather than
// failing the whole search, since it's a display field, not an identity
// one.
func (r prowlarrRelease) toIndexerRelease() ports.IndexerRelease {
	cats := make([]ports.Category, len(r.Categories))
	for i, c := range r.Categories {
		cats[i] = ports.Category{ID: c.ID, Name: c.Name}
	}
	return ports.IndexerRelease{
		GUID:        r.GUID,
		Title:       r.Title,
		IndexerName: r.Indexer,
		Size:        r.Size,
		Protocol:    r.Protocol,
		PublishDate: parsePublishDate(r.PublishDate),
		Seeders:     r.Seeders,
		Leechers:    r.Leechers,
		DownloadURL: r.DownloadURL,
		MagnetURL:   r.MagnetURL,
		InfoURL:     r.InfoURL,
		InfoHash:    r.InfoHash,
		Categories:  cats,
	}
}

// Search implements ports.IndexerSearcher. A zero-result search returns an
// empty, non-nil slice and a nil error — never ports.ErrNotFound — per the
// port's own doc comment.
func (c *Client) Search(ctx context.Context, params ports.IndexerSearchParams) ([]ports.IndexerRelease, error) {
	ctx, span := c.tracer.Start(ctx, "prowlarr.Search", trace.WithAttributes(
		attribute.String("prowlarr.query", params.Query),
	))
	defer span.End()

	q := url.Values{}
	q.Set("query", params.Query)
	for _, cat := range params.Categories {
		q.Add("categories", strconv.Itoa(cat))
	}
	for _, id := range params.IndexerIDs {
		q.Add("indexerIds", strconv.Itoa(id))
	}
	reqURL := c.baseURL + "/search?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("adapters/prowlarr: building search request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adapters/prowlarr: requesting search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("prowlarr.operation", "Search"),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode == http.StatusNotFound {
		c.logger.DebugContext(ctx, "prowlarr search not found", "query", params.Query)
		return nil, fmt.Errorf("adapters/prowlarr: search: %w", ports.ErrNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("adapters/prowlarr: search: unexpected status %d", resp.StatusCode)
	}

	var raw []prowlarrRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("adapters/prowlarr: decoding search response: %w", err)
	}

	c.logger.DebugContext(ctx, "prowlarr search succeeded", "query", params.Query, "results", len(raw))

	results := make([]ports.IndexerRelease, len(raw))
	for i, r := range raw {
		results[i] = r.toIndexerRelease()
	}
	return results, nil
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
// instrumentation still wraps it, only the actual socket is replaced. Used
// to run this Client against a canned pkg/httpclient/httpmock.Transport
// instead of a live network call.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.baseTransport = rt }
}
