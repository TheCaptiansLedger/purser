// Package qbittorrent is the qBittorrent adapter implementing
// ports.DownloadClient (Protocol() == ports.ProtocolTorrent). See
// docs/technical/acquisition-download-client.md and
// docs/technical/acquisition-pipeline.md — this is an ordinary
// capability-shaped port, not a docs/adr/0027-provider-independence.md
// provider exception, even though (like Prowlarr) there is exactly one
// torrent-protocol adapter today.
//
// qBittorrent's WebUI API (https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1))
// has no static API key. It authenticates via
// "POST /api/v2/auth/login" (form-encoded username/password), which
// replies 200 with a plain-text body — "Ok." on success, "Fails." on bad
// credentials, never a non-200 status — and a "SID" session cookie on
// success. That cookie must ride every subsequent request; Client stores
// it in memory (mutex-protected, never a stdlib http.CookieJar — explicit
// adapter-owned state is simpler to reason about and test than jar/domain
// matching) and re-authenticates once, transparently, on any authenticated
// call that comes back 403 (qBittorrent's "not logged in" response).
//
// "POST /api/v2/torrents/add" (multipart form: urls, category, tags) is
// qBittorrent's own add endpoint — it replies 200 "Ok." and never returns
// an ID for the torrent it just added, a known limitation of this API.
// To satisfy DownloadClient.Add's externalID return, this adapter
// generates a random tag, submits it alongside the add request, and reads
// the resulting torrent's hash back via
// "GET /api/v2/torrents/info?tag=<tag>" (retried briefly — qBittorrent
// registers a torrent by hash synchronously in practice, but this adapter
// doesn't assume that's guaranteed). That hash is the externalID Status/
// Remove are called with afterward.
//
// Status uses "GET /api/v2/torrents/info?hashes=<hash>" (empty result ->
// ports.ErrNotFound). Remove uses
// "POST /api/v2/torrents/delete" (form: hashes, deleteFiles) — qBittorrent
// always replies 200 regardless of whether the hash exists, so Remove
// pre-checks existence via the same info-by-hash call Status uses, to
// satisfy the port's ErrNotFound-on-unknown-ID contract.
package qbittorrent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"purser/internal/ports"
	"purser/internal/version"
	"purser/pkg/httpclient"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/qbittorrent"

// etaInfinity is qBittorrent's sentinel ETA value (100 days, in seconds)
// meaning "no meaningful ETA yet" — reported as-is for torrents with no
// peers/no download activity. Never surfaced as a real *time.Duration.
const etaInfinity = 8640000

// Config configures a Client built by New. BaseURL, Username, and Password
// have no safe default and must be supplied by the caller — like Prowlarr,
// qBittorrent is self-hosted only. Field names/tags follow the
// PURSER_<NESTED>_<KEY> convention documented in ADR 0010 for Viper
// embedding (see config.QBittorrent).
type Config struct {
	// BaseURL is qBittorrent's WebUI API root every call is issued
	// against, e.g. "http://qbittorrent.local:8080". Required; New
	// returns an error if empty.
	BaseURL string `mapstructure:"base_url"`

	// Username authenticates against POST /api/v2/auth/login. Required;
	// New returns an error if empty.
	Username string `mapstructure:"username"`

	// Password authenticates against POST /api/v2/auth/login. Required;
	// New returns an error if empty.
	Password string `mapstructure:"password"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as every other
	// adapter in this package family.
	HTTPClient httpclient.Config `mapstructure:"http_client"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// BaseURL/Username/Password which callers must supply themselves.
func DefaultConfig() Config {
	return Config{
		HTTPClient: httpclient.DefaultConfig(),
	}
}

// Client is the qBittorrent ports.DownloadClient adapter. Safe for
// concurrent use.
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client

	mu  sync.Mutex
	sid string

	tagLookupInterval    time.Duration
	tagLookupMaxAttempts int

	logger *slog.Logger
	tracer trace.Tracer

	requests metric.Int64Counter
}

var _ ports.DownloadClient = (*Client)(nil)

// New constructs a Client from cfg. It always builds its *http.Client via
// pkg/httpclient.New (never a bare http.Client) and sets a fixed
// User-Agent regardless of cfg.HTTPClient.UserAgent, same as every other
// adapter in this package family.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("adapters/qbittorrent: BaseURL must not be empty")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("adapters/qbittorrent: Username must not be empty")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("adapters/qbittorrent: Password must not be empty")
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
		return nil, fmt.Errorf("adapters/qbittorrent: building http client: %w", err)
	}

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("qbittorrent.requests", metric.WithDescription("qBittorrent WebUI API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/qbittorrent: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:              strings.TrimSuffix(cfg.BaseURL, "/"),
		username:             cfg.Username,
		password:             cfg.Password,
		http:                 httpClient,
		tagLookupInterval:    o.tagLookupInterval,
		tagLookupMaxAttempts: o.tagLookupMaxAttempts,
		logger:               o.logger.With("component", "adapters.qbittorrent"),
		tracer:               o.tracerProvider.Tracer(instrumentationName),
		requests:             requests,
	}, nil
}

// Protocol implements ports.DownloadClient.
func (c *Client) Protocol() ports.Protocol {
	return ports.ProtocolTorrent
}

func (c *Client) getSID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sid
}

func (c *Client) setSID(sid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sid = sid
}

// login authenticates against POST /api/v2/auth/login and stores the
// resulting SID session cookie. See the package doc comment for the exact
// success/failure response shape.
func (c *Client) login(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "qbittorrent.login")
	defer span.End()

	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("adapters/qbittorrent: building login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("adapters/qbittorrent: requesting login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("qbittorrent.operation", "login"),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("adapters/qbittorrent: login: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("adapters/qbittorrent: reading login response: %w", err)
	}
	if strings.TrimSpace(string(body)) != "Ok." {
		return fmt.Errorf("adapters/qbittorrent: login: authentication failed")
	}

	sid := sessionCookie(resp.Cookies())
	if sid == "" {
		return fmt.Errorf("adapters/qbittorrent: login: response carried no SID cookie")
	}
	c.setSID(sid)
	c.logger.DebugContext(ctx, "qbittorrent login succeeded")
	return nil
}

func sessionCookie(cookies []*http.Cookie) string {
	for _, ck := range cookies {
		if ck.Name == "SID" {
			return ck.Value
		}
	}
	return ""
}

// requestBuilder builds a fresh *http.Request on every call — doAuthenticated
// may call it twice (once, then again after a re-login), and an *http.Request
// with a body can't be replayed once its body has been read.
type requestBuilder func(ctx context.Context) (*http.Request, error)

// doAuthenticated issues the request build produces, logging in first if no
// session exists yet, and transparently re-authenticating and retrying once
// if the call comes back 403 (qBittorrent's "not logged in"/expired-session
// response). The caller owns closing the returned response's body.
func (c *Client) doAuthenticated(ctx context.Context, build requestBuilder) (*http.Response, error) {
	if c.getSID() == "" {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}

	resp, err := c.attempt(ctx, build)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		c.logger.DebugContext(ctx, "qbittorrent session expired, re-authenticating")
		if err := c.login(ctx); err != nil {
			return nil, err
		}
		resp, err = c.attempt(ctx, build)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (c *Client) attempt(ctx context.Context, build requestBuilder) (*http.Response, error) {
	req, err := build(ctx)
	if err != nil {
		return nil, fmt.Errorf("adapters/qbittorrent: building request: %w", err)
	}
	if sid := c.getSID(); sid != "" {
		// Set the Cookie header directly rather than http.Cookie+AddCookie —
		// AddCookie only ever reads Name/Value to build this header, but an
		// http.Cookie literal missing Secure/HttpOnly/SameSite trips gosec's
		// G124 regardless of use; those are response-cookie attributes that
		// don't apply to an outgoing request header at all.
		req.Header.Set("Cookie", "SID="+sid)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adapters/qbittorrent: requesting %s: %w", req.URL.Path, err)
	}
	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("qbittorrent.path", req.URL.Path),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))
	return resp, nil
}

// qbTorrent is qBittorrent's own wire shape for one /api/v2/torrents/info
// result. Decoded separately from ports.DownloadStatus (rather than
// decoding straight into the port DTO) so this adapter's JSON tags can
// match qBittorrent's real API exactly, independent of the port DTO's own
// field names.
type qbTorrent struct {
	Hash     string  `json:"hash"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	SavePath string  `json:"save_path"`
	ETA      int64   `json:"eta"`
}

// toDownloadStatus translates t into the client-neutral ports.DownloadStatus.
func (t qbTorrent) toDownloadStatus() ports.DownloadStatus {
	status := ports.DownloadStatus{
		ExternalID: t.Hash,
		State:      normalizeState(t.State),
		Progress:   t.Progress,
		SavePath:   t.SavePath,
	}
	if t.ETA > 0 && t.ETA < etaInfinity {
		eta := time.Duration(t.ETA) * time.Second
		status.ETA = &eta
	}
	return status
}

// normalizeState maps qBittorrent's own state vocabulary onto the shared
// ports.DownloadState values, per
// https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#get-torrent-list's
// documented state enum:
//
//   - downloading/metaDL/forcedMetaDL/forcedDL/allocating/moving/checkingDL
//     -> downloading (still actively fetching data or being prepared to)
//   - pausedDL -> paused (download itself is paused, not yet finished)
//   - queuedDL/stalledDL -> queued (waiting on the queue or on peers,
//     download not yet started/progressing)
//   - uploading/stalledUP/checkingUP/forcedUP/pausedUP/queuedUP -> completed
//     (the download itself already finished; these are all post-download
//     seeding states)
//   - error/missingFiles -> failed
//   - checkingResumeData/unknown/anything else -> queued, the closest
//     "no verdict yet" default among the five shared states.
func normalizeState(s string) ports.DownloadState {
	switch s {
	case "downloading", "metaDL", "forcedMetaDL", "forcedDL", "allocating", "moving", "checkingDL":
		return ports.DownloadStateDownloading
	case "pausedDL":
		return ports.DownloadStatePaused
	case "queuedDL", "stalledDL":
		return ports.DownloadStateQueued
	case "uploading", "stalledUP", "checkingUP", "forcedUP", "pausedUP", "queuedUP":
		return ports.DownloadStateCompleted
	case "error", "missingFiles":
		return ports.DownloadStateFailed
	default:
		return ports.DownloadStateQueued
	}
}

// torrentsInfo calls GET /api/v2/torrents/info with q as the query string
// (e.g. {"hashes": [hash]} or {"tag": [tag]}), shared by Status/Remove's
// existence check and Add's post-submit tag lookup.
func (c *Client) torrentsInfo(ctx context.Context, q url.Values) ([]qbTorrent, error) {
	build := func(ctx context.Context) (*http.Request, error) {
		reqURL := c.baseURL + "/api/v2/torrents/info?" + q.Encode()
		return http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	}

	resp, err := c.doAuthenticated(ctx, build)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adapters/qbittorrent: torrents info: unexpected status %d", resp.StatusCode)
	}

	var torrents []qbTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("adapters/qbittorrent: decoding torrents info response: %w", err)
	}
	return torrents, nil
}

// newTag generates a short random tag used only to look up the hash of a
// just-submitted torrent — see the package doc comment.
func newTag() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("adapters/qbittorrent: generating tag: %w", err)
	}
	return "purser-" + hex.EncodeToString(b[:]), nil
}

// findHashByTag polls GET /api/v2/torrents/info?tag=<tag> for the hash of
// the torrent just submitted under tag, retrying briefly since qBittorrent
// gives Add no other way to learn the new torrent's identity.
func (c *Client) findHashByTag(ctx context.Context, tag string) (string, error) {
	for attempt := 1; attempt <= c.tagLookupMaxAttempts; attempt++ {
		torrents, err := c.torrentsInfo(ctx, url.Values{"tag": {tag}})
		if err != nil {
			return "", err
		}
		if len(torrents) > 0 {
			return torrents[0].Hash, nil
		}

		if attempt == c.tagLookupMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.tagLookupInterval):
		}
	}
	return "", fmt.Errorf("adapters/qbittorrent: no torrent found for tag %q after add", tag)
}

// Add implements ports.DownloadClient.
func (c *Client) Add(ctx context.Context, req ports.AddDownloadRequest) (string, error) {
	ctx, span := c.tracer.Start(ctx, "qbittorrent.Add", trace.WithAttributes(
		attribute.String("qbittorrent.title", req.Title),
	))
	defer span.End()

	tag, err := newTag()
	if err != nil {
		return "", fmt.Errorf("adapters/qbittorrent: add: %w", err)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		if err := w.WriteField("urls", req.DownloadURL); err != nil {
			return nil, err
		}
		if req.Category != "" {
			if err := w.WriteField("category", req.Category); err != nil {
				return nil, err
			}
		}
		if err := w.WriteField("tags", tag); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", body)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", w.FormDataContentType())
		return httpReq, nil
	}

	resp, err := c.doAuthenticated(ctx, build)
	if err != nil {
		return "", fmt.Errorf("adapters/qbittorrent: add: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("adapters/qbittorrent: add: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("adapters/qbittorrent: reading add response: %w", err)
	}
	if strings.TrimSpace(string(body)) != "Ok." {
		return "", fmt.Errorf("adapters/qbittorrent: add: qbittorrent reported failure")
	}

	hash, err := c.findHashByTag(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("adapters/qbittorrent: add: %w", err)
	}
	c.logger.DebugContext(ctx, "qbittorrent add succeeded", "hash", hash)
	return hash, nil
}

// Status implements ports.DownloadClient.
func (c *Client) Status(ctx context.Context, externalID string) (ports.DownloadStatus, error) {
	ctx, span := c.tracer.Start(ctx, "qbittorrent.Status", trace.WithAttributes(
		attribute.String("qbittorrent.hash", externalID),
	))
	defer span.End()

	torrents, err := c.torrentsInfo(ctx, url.Values{"hashes": {externalID}})
	if err != nil {
		return ports.DownloadStatus{}, fmt.Errorf("adapters/qbittorrent: status: %w", err)
	}
	if len(torrents) == 0 {
		c.logger.DebugContext(ctx, "qbittorrent status not found", "hash", externalID)
		return ports.DownloadStatus{}, fmt.Errorf("adapters/qbittorrent: status: %w", ports.ErrNotFound)
	}
	return torrents[0].toDownloadStatus(), nil
}

// Remove implements ports.DownloadClient. qBittorrent's own delete endpoint
// replies 200 regardless of whether hashes matches anything, so existence is
// checked first via the same call Status uses, to surface ports.ErrNotFound
// for an unknown externalID per the port's contract.
func (c *Client) Remove(ctx context.Context, externalID string, deleteFiles bool) error {
	ctx, span := c.tracer.Start(ctx, "qbittorrent.Remove", trace.WithAttributes(
		attribute.String("qbittorrent.hash", externalID),
		attribute.Bool("qbittorrent.delete_files", deleteFiles),
	))
	defer span.End()

	torrents, err := c.torrentsInfo(ctx, url.Values{"hashes": {externalID}})
	if err != nil {
		return fmt.Errorf("adapters/qbittorrent: remove: %w", err)
	}
	if len(torrents) == 0 {
		c.logger.DebugContext(ctx, "qbittorrent remove not found", "hash", externalID)
		return fmt.Errorf("adapters/qbittorrent: remove: %w", ports.ErrNotFound)
	}

	build := func(ctx context.Context) (*http.Request, error) {
		form := url.Values{}
		form.Set("hashes", externalID)
		form.Set("deleteFiles", strconv.FormatBool(deleteFiles))
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/delete", strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return httpReq, nil
	}

	resp, err := c.doAuthenticated(ctx, build)
	if err != nil {
		return fmt.Errorf("adapters/qbittorrent: remove: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("adapters/qbittorrent: remove: unexpected status %d", resp.StatusCode)
	}
	c.logger.DebugContext(ctx, "qbittorrent remove succeeded", "hash", externalID)
	return nil
}

// Option customizes a Client constructed via New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	baseTransport  http.RoundTripper

	tagLookupInterval    time.Duration
	tagLookupMaxAttempts int
}

func defaultOptions() *options {
	return &options{
		logger:               slog.Default(),
		tracerProvider:       otel.GetTracerProvider(),
		meterProvider:        otel.GetMeterProvider(),
		tagLookupInterval:    250 * time.Millisecond,
		tagLookupMaxAttempts: 10,
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

// WithTagLookupInterval overrides the default (250ms) delay findHashByTag
// waits between retries. Tests pass a near-zero value so a not-found-by-tag
// test doesn't have to actually wait out the real interval.
func WithTagLookupInterval(d time.Duration) Option {
	return func(o *options) { o.tagLookupInterval = d }
}

// WithTagLookupMaxAttempts overrides the default (10) number of times
// findHashByTag polls before giving up.
func WithTagLookupMaxAttempts(n int) Option {
	return func(o *options) { o.tagLookupMaxAttempts = n }
}
