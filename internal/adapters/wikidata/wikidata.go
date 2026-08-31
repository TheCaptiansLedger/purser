// Package wikidata is the single Wikidata adapter implementing
// ports.WikidataClient. See docs/adr/0027-provider-independence.md.
//
// Wikidata's action API (www.wikidata.org/w/api.php) is public and
// requires no authentication. LookupImage issues one call —
// action=wbgetclaims&entity={qid}&property=P18&format=json — confirmed
// live against real entities during this adapter's implementation:
//
//   - A known entity with an image: {"claims":{"P18":[{"mainsnak":
//     {"datavalue":{"value":"<Commons filename>"}}}, ...]}}, HTTP 200.
//     Multiple P18 statements are real (e.g. REO Speedwagon's own
//     Q845084 carries two) — every one is returned, in the order
//     Wikidata's own claims array gave them.
//   - A known entity with no image claim: {"claims":{}}, HTTP 200 — not
//     distinguishable, at the wire level, from an unknown-but
//     syntactically-valid QID's own body shape below except by the
//     "error" key's presence.
//   - An unknown (but syntactically valid) QID:
//     {"error":{"code":"no-such-entity", ...}}, still HTTP 200 — the
//     action API never uses an HTTP status code the way this port's
//     REST-backed siblings (StashDB, ThePornDB) do.
//
// Both no-image cases map to ports.ErrNotFound: this adapter's only
// capability is "get a photo for this entity," and there's nothing more
// specific a caller could do differently between them.
//
// The P18 claim's own value is a bare Commons filename (e.g.
// "REO Speedwagon.jpg"), not a URL. LookupImage turns each one into a
// hotlinkable URL via Commons' Special:FilePath redirect helper — spaces
// swapped for underscores, then path-escaped — confirmed live to redirect
// (Special:FilePath -> Special:Redirect/file -> the real
// upload.wikimedia.org file, two hops, both hops followed transparently by
// a browser's <img> tag or a Go http.Client's default redirect handling)
// straight to the actual image bytes. No second network call is needed to
// resolve it — the transformation is Commons' own deterministic URL
// convention, not something this adapter has to look up.
package wikidata

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
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	instrumentationName = "purser/internal/adapters/wikidata"

	// defaultBaseURL is Wikidata's real action API endpoint. Overridable
	// via Config.BaseURL, e.g. to point at a fixture server in tests.
	defaultBaseURL = "https://www.wikidata.org/w/api.php"

	// commonsFilePathBaseURL is Commons' fixed Special:FilePath helper —
	// a URL-construction convention, not a connection endpoint this
	// adapter calls, so unlike BaseURL it isn't part of Config.
	commonsFilePathBaseURL = "https://commons.wikimedia.org/wiki/Special:FilePath/"

	// noSuchEntityErrorCode is the wbgetclaims error code confirmed live
	// for a syntactically valid but nonexistent QID.
	noSuchEntityErrorCode = "no-such-entity"

	// requestsPerSecond is a conservative, unverified placeholder — no
	// published rate-limit headers were observed on any live response
	// during this adapter's implementation, the same situation
	// fanarttv.go's/theaudiodb.go's identical constant documents.
	requestsPerSecond = 5

	// maxAttempts bounds retries of a transient failure (HTTP 503, or a
	// transport-level error) in get — see get's retry loop and
	// musicbrainz.go's/fanarttv.go's identical convention.
	maxAttempts = 4
)

// entityIDPattern matches a Wikidata item ID (a "Q" followed by digits,
// e.g. "Q845084") — the last path segment of a wikidata.org entity URL
// (https://www.wikidata.org/wiki/Q845084). Property IDs ("P...") are
// deliberately not matched: nothing in this codebase looks up a property
// entity's own image.
var entityIDPattern = regexp.MustCompile(`^Q[1-9][0-9]*$`)

// Config configures a Client built by New. Unlike fanarttv.Config/
// stashdb.Config, there is no APIKey field — Wikidata's action API
// requires no authentication.
type Config struct {
	// BaseURL is Wikidata's action API endpoint every request is issued
	// against.
	BaseURL string `mapstructure:"base_url"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as every other
	// adapter in this package family.
	HTTPClient httpclient.Config `mapstructure:"http_client"`

	// Cache configures this adapter's own named pkg/cache.Cache instance,
	// wrapping the HTTP client via httpclient.NewCachingTransport.
	Cache cache.Config `mapstructure:"cache"`
}

// DefaultConfig returns Config's sane defaults. The cache's DefaultTTL
// mirrors the rest of this package family's: an entity's P18 claim rarely
// changes.
func DefaultConfig() Config {
	cacheCfg := cache.DefaultConfig()
	cacheCfg.DefaultTTL = 24 * time.Hour

	return Config{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpclient.DefaultConfig(),
		Cache:      cacheCfg,
	}
}

// Client is the Wikidata ports.WikidataClient adapter. Safe for concurrent
// use.
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

var _ ports.WikidataClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) with a caching transport
// (pkg/httpclient.NewCachingTransport) wrapping a named in-memory
// pkg/cache instance, and sets a fixed User-Agent regardless of
// cfg.HTTPClient.UserAgent, same as every other adapter in this package
// family.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/wikidata: BaseURL must not be empty")
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
		return nil, fmt.Errorf("adapters/wikidata: building http client: %w", err)
	}

	c, err := memory.New("wikidata", cfg.Cache,
		memory.WithLogger(o.logger),
		memory.WithTracerProvider(o.tracerProvider),
		memory.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/wikidata: building cache: %w", err)
	}

	cachingTransport, err := httpclient.NewCachingTransport(httpClient.Transport, c,
		httpclient.WithLogger(o.logger),
		httpclient.WithTracerProvider(o.tracerProvider),
		httpclient.WithMeterProvider(o.meterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("adapters/wikidata: building caching transport: %w", err)
	}
	httpClient.Transport = cachingTransport

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("wikidata.requests", metric.WithDescription("Wikidata API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/wikidata: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:        cfg.BaseURL,
		http:           httpClient,
		cache:          c,
		limiter:        rate.NewLimiter(o.rateLimit, 1),
		retryBaseDelay: o.retryBaseDelay,
		logger:         o.logger.With("component", "adapters.wikidata"),
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

// wbGetClaimsResponse is wbgetclaims' own response shape, filtered to the
// one property (P18) this adapter ever requests.
type wbGetClaimsResponse struct {
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
	Claims struct {
		P18 []struct {
			Mainsnak struct {
				Datavalue struct {
					Value string `json:"value"`
				} `json:"datavalue"`
			} `json:"mainsnak"`
		} `json:"P18"`
	} `json:"claims"`
}

// LookupImage implements ports.WikidataClient.
func (c *Client) LookupImage(ctx context.Context, entityURL string) ([]ports.WikidataImage, error) {
	qid, err := entityIDFromURL(entityURL)
	if err != nil {
		return nil, fmt.Errorf("adapters/wikidata: LookupImage %s: %w", entityURL, err)
	}

	q := url.Values{}
	q.Set("action", "wbgetclaims")
	q.Set("entity", qid)
	q.Set("property", "P18")
	q.Set("format", "json")

	var resp wbGetClaimsResponse
	if err := c.get(ctx, "LookupImage", q, &resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		if resp.Error.Code == noSuchEntityErrorCode {
			return nil, fmt.Errorf("adapters/wikidata: LookupImage %s: %w", qid, ports.ErrNotFound)
		}
		return nil, fmt.Errorf("adapters/wikidata: LookupImage %s: %s", qid, resp.Error.Code)
	}
	if len(resp.Claims.P18) == 0 {
		return nil, fmt.Errorf("adapters/wikidata: LookupImage %s: no P18 claim: %w", qid, ports.ErrNotFound)
	}

	images := make([]ports.WikidataImage, len(resp.Claims.P18))
	for i, claim := range resp.Claims.P18 {
		images[i] = ports.WikidataImage{URL: commonsFilePathURL(claim.Mainsnak.Datavalue.Value)}
	}
	return images, nil
}

// entityIDFromURL extracts the "Q..." item ID from the last path segment
// of a wikidata.org entity URL (e.g.
// "https://www.wikidata.org/wiki/Q845084" -> "Q845084").
func entityIDFromURL(entityURL string) (string, error) {
	u, err := url.Parse(entityURL)
	if err != nil {
		return "", fmt.Errorf("parsing entity URL: %w", err)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	qid := segments[len(segments)-1]
	if !entityIDPattern.MatchString(qid) {
		return "", fmt.Errorf("entity URL %q does not end in a Wikidata item ID (Q...)", entityURL)
	}
	return qid, nil
}

// commonsFilePathURL turns a raw Commons filename (a P18 claim's own
// value, e.g. "REO Speedwagon.jpg") into Commons' Special:FilePath
// redirect URL — confirmed live to redirect straight to the real file on
// upload.wikimedia.org, hotlinkable directly by a browser's <img> tag.
func commonsFilePathURL(filename string) string {
	return commonsFilePathBaseURL + url.PathEscape(strings.ReplaceAll(filename, " ", "_"))
}

// get issues a rate-limited GET against baseURL with query params q and
// decodes the JSON response into out — the single choke point LookupImage
// routes through, so rate limiting, tracing, metrics, and retry are
// implemented exactly once. Unlike fanarttv.go's/musicbrainz.go's
// identical-shaped helper, there is no resource path to append: Wikidata's
// action API is one fixed endpoint, every request distinguished purely by
// query params.
//
// A transient failure (HTTP 503, or a transport-level error) is retried up
// to maxAttempts times with an increasing delay, still paced through the
// limiter on every attempt so a retry storm can't itself violate the rate
// limit. A non-2xx status otherwise is a real error — confirmed live,
// Wikidata's action API answers even "no such entity" with HTTP 200 (see
// the package doc comment), so a genuine non-2xx here means something else
// went wrong (a malformed request, an outage) and is never mapped to
// ports.ErrNotFound.
func (c *Client) get(ctx context.Context, operationName string, q url.Values, out any) error {
	ctx, span := c.tracer.Start(ctx, "wikidata."+operationName)
	defer span.End()

	reqURL := c.baseURL + "?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("adapters/wikidata: rate limiter: %w", err)
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
		c.logger.WarnContext(ctx, "wikidata request failed, retrying",
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
// else is not: retrying wouldn't change the outcome. Mirrors
// fanarttv.go's/musicbrainz.go's identical function.
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
		return 0, fmt.Errorf("adapters/wikidata: building request for %s: %w", operationName, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("adapters/wikidata: requesting %s: %w", operationName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("wikidata.operation", operationName),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("adapters/wikidata: %s: unexpected status %d", operationName, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("adapters/wikidata: decoding %s response: %w", operationName, err)
	}

	c.logger.DebugContext(ctx, "wikidata request succeeded", "operation", operationName)
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
