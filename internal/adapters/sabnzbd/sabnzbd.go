// Package sabnzbd is the SABnzbd adapter implementing ports.DownloadClient
// (Protocol() == ports.ProtocolUsenet). See
// docs/technical/acquisition-download-client.md and
// docs/technical/acquisition-pipeline.md — this is an ordinary
// capability-shaped port, not a docs/adr/0027-provider-independence.md
// provider exception, even though (like qBittorrent and Prowlarr) there is
// exactly one usenet-protocol adapter today.
//
// SABnzbd's API (https://sabnzbd.org/wiki/configuration/5.0/api) is a
// single "GET {base_url}/api" endpoint routed by a "mode" query parameter,
// authenticated by a static "&apikey=..." query parameter on every call —
// unlike qBittorrent's session-cookie login (internal/adapters/qbittorrent),
// there is no separate login step to manage.
//
// Add uses "mode=addurl" with "name=<nzb url>" and an optional "cat"
// (Purser's AddDownloadRequest.Category passed through as-is, the same
// passthrough shape qBittorrent's "category" field uses) — the response is
// {"status": true, "nzo_ids": ["SABnzbd_nzo_..."]} on success, or
// {"status": false, "error": "..."} on failure. The first nzo_id is this
// adapter's externalID.
//
// SABnzbd has no single "look up this ID" call the way qBittorrent's
// hashes-filtered /torrents/info does; instead an nzo_id lives in exactly
// one of two lists at any time — "mode=queue" while actively
// downloading/queued/paused, "mode=history" once the download itself has
// finished (including every post-processing stage: verifying, repairing,
// extracting, moving, running a post-processing script). Both endpoints
// accept an "nzo_ids" filter parameter, used by this adapter instead of
// fetching the full list. Status checks queue first, then history, and
// Remove needs the same two-step lookup to know which of
// "mode=queue&name=delete" or "mode=history&name=delete" to call (SABnzbd
// has no single delete-by-ID call spanning both lists, and unlike
// qBittorrent's delete endpoint, calling the wrong one for an ID it doesn't
// hold is not reliably a silent no-op) — ports.ErrNotFound is returned only
// once the ID is confirmed absent from both.
package sabnzbd

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

const instrumentationName = "purser/internal/adapters/sabnzbd"

// Config configures a Client built by New. BaseURL and APIKey have no safe
// default and must be supplied by the caller — like qBittorrent and
// Prowlarr, SABnzbd is self-hosted only. Field names/tags follow the
// PURSER_<NESTED>_<KEY> convention documented in ADR 0010 for Viper
// embedding (see config.SABnzbd).
type Config struct {
	// BaseURL is SABnzbd's API root every call is issued against, e.g.
	// "http://sabnzbd.local:8080/sabnzbd" — every call appends "/api"
	// directly to this value. Required; New returns an error if empty.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "apikey" query parameter on every request.
	// Required; New returns an error if empty.
	APIKey string `mapstructure:"api_key"`

	// HTTPClient configures the shared pkg/httpclient this adapter builds
	// its *http.Client from. UserAgent is always overwritten by New with
	// the fixed "Purser/<version>" value, same convention as every other
	// adapter in this package family.
	HTTPClient httpclient.Config `mapstructure:"http_client"`
}

// DefaultConfig returns the sane defaults every Client starts from, except
// BaseURL/APIKey which callers must supply themselves.
func DefaultConfig() Config {
	return Config{
		HTTPClient: httpclient.DefaultConfig(),
	}
}

// Client is the SABnzbd ports.DownloadClient adapter. Safe for concurrent
// use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

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
		return nil, fmt.Errorf("adapters/sabnzbd: BaseURL must not be empty")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("adapters/sabnzbd: APIKey must not be empty")
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
		return nil, fmt.Errorf("adapters/sabnzbd: building http client: %w", err)
	}

	meter := o.meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("sabnzbd.requests", metric.WithDescription("SABnzbd API requests issued"))
	if err != nil {
		return nil, fmt.Errorf("adapters/sabnzbd: creating requests counter: %w", err)
	}

	return &Client{
		baseURL:  strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		http:     httpClient,
		logger:   o.logger.With("component", "adapters.sabnzbd"),
		tracer:   o.tracerProvider.Tracer(instrumentationName),
		requests: requests,
	}, nil
}

// Protocol implements ports.DownloadClient.
func (c *Client) Protocol() ports.Protocol {
	return ports.ProtocolUsenet
}

// statusResponse is SABnzbd's own wire shape shared by every write call
// (addurl, queue delete, history delete) — {"status": true} on success,
// {"status": false, "error": "..."} on failure.
type statusResponse struct {
	Status bool   `json:"status"`
	Error  string `json:"error"`
}

// addResponse is SABnzbd's own "mode=addurl" wire shape.
type addResponse struct {
	statusResponse
	NzoIDs []string `json:"nzo_ids"`
}

// Add implements ports.DownloadClient.
func (c *Client) Add(ctx context.Context, req ports.AddDownloadRequest) (string, error) {
	ctx, span := c.tracer.Start(ctx, "sabnzbd.Add", trace.WithAttributes(
		attribute.String("sabnzbd.title", req.Title),
	))
	defer span.End()

	q := c.baseQuery("addurl")
	q.Set("name", req.DownloadURL)
	if req.Category != "" {
		q.Set("cat", req.Category)
	}

	var decoded addResponse
	if err := c.get(ctx, "addurl", q, &decoded); err != nil {
		return "", fmt.Errorf("adapters/sabnzbd: add: %w", err)
	}
	if !decoded.Status {
		return "", fmt.Errorf("adapters/sabnzbd: add: sabnzbd reported failure: %s", decoded.Error)
	}
	if len(decoded.NzoIDs) == 0 {
		return "", fmt.Errorf("adapters/sabnzbd: add: response carried no nzo_ids")
	}

	id := decoded.NzoIDs[0]
	c.logger.DebugContext(ctx, "sabnzbd add succeeded", "nzo_id", id)
	return id, nil
}

// queueSlot is SABnzbd's own "mode=queue" per-job wire shape. Percentage/
// MB/MBLeft/TimeLeft are strings in SABnzbd's own API, not numbers.
type queueSlot struct {
	NzoID      string `json:"nzo_id"`
	Status     string `json:"status"`
	Percentage string `json:"percentage"`
	TimeLeft   string `json:"timeleft"`
}

type queueResponse struct {
	Queue struct {
		Slots []queueSlot `json:"slots"`
	} `json:"queue"`
}

// historySlot is SABnzbd's own "mode=history" per-job wire shape.
type historySlot struct {
	NzoID   string `json:"nzo_id"`
	Status  string `json:"status"`
	Storage string `json:"storage"`
}

type historyResponse struct {
	History struct {
		Slots []historySlot `json:"slots"`
	} `json:"history"`
}

// toDownloadStatus translates a queue slot into the client-neutral
// ports.DownloadStatus. SavePath is empty — SABnzbd doesn't know a job's
// final storage path until it reaches history (see historySlot's Storage).
func (s queueSlot) toDownloadStatus() ports.DownloadStatus {
	status := ports.DownloadStatus{
		ExternalID: s.NzoID,
		State:      normalizeQueueState(s.Status),
		Progress:   parsePercentage(s.Percentage),
	}
	if eta := parseTimeLeft(s.TimeLeft); eta != nil {
		status.ETA = eta
	}
	return status
}

// toDownloadStatus translates a history slot into the client-neutral
// ports.DownloadStatus. Progress is always 1.0: by definition of being in
// history at all, the download itself has fully completed — the remaining
// post-processing stages (verify/repair/extract/move/script) don't affect
// how much of the download was fetched, only whether the file is ready at
// SavePath yet, which is what State communicates. ETA is never set —
// SABnzbd reports no further download-remaining estimate once a job has
// left the queue.
func (s historySlot) toDownloadStatus() ports.DownloadStatus {
	return ports.DownloadStatus{
		ExternalID: s.NzoID,
		State:      normalizeHistoryState(s.Status),
		Progress:   1.0,
		SavePath:   s.Storage,
	}
}

// normalizeQueueState maps SABnzbd's own "mode=queue" status vocabulary
// onto the shared ports.DownloadState values, per
// https://sabnzbd.org/wiki/configuration/5.0/api's documented queue status
// enum:
//
//   - Downloading/Fetching (downloading extra par2 files, still active
//     data transfer) -> downloading
//   - Queued/Propagating (a deliberately delayed download, not yet
//     started) -> queued
//   - Paused -> paused
//   - anything else -> queued, the closest "no verdict yet" default among
//     the five shared states.
func normalizeQueueState(s string) ports.DownloadState {
	switch s {
	case "Downloading", "Fetching":
		return ports.DownloadStateDownloading
	case "Queued", "Propagating":
		return ports.DownloadStateQueued
	case "Paused":
		return ports.DownloadStatePaused
	default:
		return ports.DownloadStateQueued
	}
}

// normalizeHistoryState maps SABnzbd's own "mode=history" status
// vocabulary onto the shared ports.DownloadState values, per
// https://sabnzbd.org/wiki/configuration/5.0/api's documented history
// status enum. Unlike qBittorrent's post-download seeding states (mapped
// to completed because the file is already fully assembled and usable),
// SABnzbd's post-processing states mean the file is *not* yet at its final
// SavePath — repair verifies/fixes it, extract unpacks it, move relocates
// it there — so they map to downloading (still in progress, not yet safe
// to hand off) rather than completed:
//
//   - Completed -> completed
//   - Failed -> failed
//   - QuickCheck/Verifying/Repairing/Extracting/Moving/Running/Fetching
//     (post-processing stages; the download itself is done but the file
//     isn't ready yet) -> downloading
//   - Queued (rare: finished downloading, waiting for a post-processing
//     slot) -> queued
//   - anything else -> queued, the closest "no verdict yet" default among
//     the five shared states.
func normalizeHistoryState(s string) ports.DownloadState {
	switch s {
	case "Completed":
		return ports.DownloadStateCompleted
	case "Failed":
		return ports.DownloadStateFailed
	case "QuickCheck", "Verifying", "Repairing", "Extracting", "Moving", "Running", "Fetching":
		return ports.DownloadStateDownloading
	case "Queued":
		return ports.DownloadStateQueued
	default:
		return ports.DownloadStateQueued
	}
}

// parsePercentage parses SABnzbd's own "percentage" string (0-100) into
// the port's 0-1 Progress, degrading to 0 on anything malformed or empty
// rather than failing the whole call over a display field.
func parsePercentage(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v / 100
}

// parseTimeLeft parses SABnzbd's own "timeleft" string
// ("<hours>:<minutes>:<seconds>", e.g. "0:16:44") into a *time.Duration,
// degrading to nil on anything malformed or empty rather than failing the
// whole call over a display field.
func parseTimeLeft(s string) *time.Duration {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return nil
	}
	hours, err1 := strconv.Atoi(parts[0])
	minutes, err2 := strconv.Atoi(parts[1])
	seconds, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	return &d
}

// queueSlotByID looks up externalID in "mode=queue", returning found=false
// (not an error) when it isn't present there — the caller falls back to
// history.
func (c *Client) queueSlotByID(ctx context.Context, externalID string) (queueSlot, bool, error) {
	q := c.baseQuery("queue")
	q.Set("nzo_ids", externalID)

	var decoded queueResponse
	if err := c.get(ctx, "queue", q, &decoded); err != nil {
		return queueSlot{}, false, err
	}
	if len(decoded.Queue.Slots) == 0 {
		return queueSlot{}, false, nil
	}
	return decoded.Queue.Slots[0], true, nil
}

// historySlotByID looks up externalID in "mode=history", returning
// found=false (not an error) when it isn't present there.
func (c *Client) historySlotByID(ctx context.Context, externalID string) (historySlot, bool, error) {
	q := c.baseQuery("history")
	q.Set("nzo_ids", externalID)

	var decoded historyResponse
	if err := c.get(ctx, "history", q, &decoded); err != nil {
		return historySlot{}, false, err
	}
	if len(decoded.History.Slots) == 0 {
		return historySlot{}, false, nil
	}
	return decoded.History.Slots[0], true, nil
}

// Status implements ports.DownloadClient. externalID is looked up in the
// queue first, then history — see the package doc comment for why SABnzbd
// needs a two-step lookup instead of one filtered call.
func (c *Client) Status(ctx context.Context, externalID string) (ports.DownloadStatus, error) {
	ctx, span := c.tracer.Start(ctx, "sabnzbd.Status", trace.WithAttributes(
		attribute.String("sabnzbd.nzo_id", externalID),
	))
	defer span.End()

	if slot, found, err := c.queueSlotByID(ctx, externalID); err != nil {
		return ports.DownloadStatus{}, fmt.Errorf("adapters/sabnzbd: status: %w", err)
	} else if found {
		return slot.toDownloadStatus(), nil
	}

	if slot, found, err := c.historySlotByID(ctx, externalID); err != nil {
		return ports.DownloadStatus{}, fmt.Errorf("adapters/sabnzbd: status: %w", err)
	} else if found {
		return slot.toDownloadStatus(), nil
	}

	c.logger.DebugContext(ctx, "sabnzbd status not found", "nzo_id", externalID)
	return ports.DownloadStatus{}, fmt.Errorf("adapters/sabnzbd: status: %w", ports.ErrNotFound)
}

// Remove implements ports.DownloadClient. externalID must be located (queue
// or history) before deleting — SABnzbd has no single delete call spanning
// both lists, so which one to call depends on where the job actually is.
func (c *Client) Remove(ctx context.Context, externalID string, deleteFiles bool) error {
	ctx, span := c.tracer.Start(ctx, "sabnzbd.Remove", trace.WithAttributes(
		attribute.String("sabnzbd.nzo_id", externalID),
		attribute.Bool("sabnzbd.delete_files", deleteFiles),
	))
	defer span.End()

	_, found, err := c.queueSlotByID(ctx, externalID)
	if err != nil {
		return fmt.Errorf("adapters/sabnzbd: remove: %w", err)
	}
	if found {
		return c.delete(ctx, "queue", externalID, deleteFiles)
	}

	_, found, err = c.historySlotByID(ctx, externalID)
	if err != nil {
		return fmt.Errorf("adapters/sabnzbd: remove: %w", err)
	}
	if found {
		return c.delete(ctx, "history", externalID, deleteFiles)
	}

	c.logger.DebugContext(ctx, "sabnzbd remove not found", "nzo_id", externalID)
	return fmt.Errorf("adapters/sabnzbd: remove: %w", ports.ErrNotFound)
}

// delete calls "mode=<listMode>&name=delete" against externalID.
func (c *Client) delete(ctx context.Context, listMode, externalID string, deleteFiles bool) error {
	q := c.baseQuery(listMode)
	q.Set("name", "delete")
	q.Set("value", externalID)
	if deleteFiles {
		q.Set("del_files", "1")
	}

	var decoded statusResponse
	if err := c.get(ctx, listMode+"-delete", q, &decoded); err != nil {
		return fmt.Errorf("adapters/sabnzbd: remove: %w", err)
	}
	if !decoded.Status {
		return fmt.Errorf("adapters/sabnzbd: remove: sabnzbd reported failure: %s", decoded.Error)
	}
	c.logger.DebugContext(ctx, "sabnzbd remove succeeded", "nzo_id", externalID, "list", listMode)
	return nil
}

// baseQuery builds the query parameters every SABnzbd call shares: mode,
// apikey, and output=json.
func (c *Client) baseQuery(mode string) url.Values {
	q := url.Values{}
	q.Set("mode", mode)
	q.Set("apikey", c.apiKey)
	q.Set("output", "json")
	return q
}

// get issues "GET {baseURL}/api?{q}", decoding the JSON response into out.
// operation labels the requests counter/log entries for this call.
func (c *Client) get(ctx context.Context, operation string, q url.Values, out any) error {
	reqURL := c.baseURL + "/api?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("building %s request: %w", operation, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", operation, err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("sabnzbd.operation", operation),
		attribute.Int("http.response.status_code", resp.StatusCode),
	))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: unexpected status %d", operation, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s response: %w", operation, err)
	}
	return nil
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
