package prowlarr_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"purser/internal/adapters/prowlarr"
	"purser/internal/ports"
	"purser/internal/ports/indexertest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"testing"

	"go.opentelemetry.io/otel"
)

// searchPath is defaultBaseURL's path component every Search call hits —
// see prowlarr.go's Search: baseURL + "/search".
const searchPath = "/search"

func newTestClient(t *testing.T, baseURL string) *prowlarr.Client {
	t.Helper()
	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := prowlarr.New(cfg)
	if err != nil {
		t.Fatalf("prowlarr.New returned error: %v", err)
	}
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at a placeholder value; RoundTrip intercepts before any DNS/dial
// ever happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...prowlarr.Option) *prowlarr.Client {
	t.Helper()
	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = "http://prowlarr.invalid"
	cfg.APIKey = "test-key"
	allOpts := append([]prowlarr.Option{prowlarr.WithBaseTransport(rt)}, opts...)
	c, err := prowlarr.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("prowlarr.New returned error: %v", err)
	}
	return c
}

func TestClient_IndexerSearcherContract(t *testing.T) {
	indexertest.TestIndexerSearcher(t, func(t *testing.T, baseURL string) ports.IndexerSearcher {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := prowlarr.New(cfg); err == nil {
		t.Fatal("prowlarr.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = "http://prowlarr.invalid"
	cfg.APIKey = ""
	if _, err := prowlarr.New(cfg); err == nil {
		t.Fatal("prowlarr.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   searchPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, []map[string]any{})(req)
		},
	})

	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = "http://prowlarr.invalid"
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := prowlarr.New(cfg, prowlarr.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("prowlarr.New returned error: %v", err)
	}

	if _, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_SendsXApiKeyHeaderNotQueryParamOrBearer(t *testing.T) {
	var gotHeader, gotAuthHeader string
	var gotQueryParam string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   searchPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotHeader = req.Header.Get("X-Api-Key")
			gotAuthHeader = req.Header.Get("Authorization")
			gotQueryParam = req.URL.Query().Get("apikey")
			return httpmock.JSON(http.StatusOK, []map[string]any{})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if gotHeader != "test-key" {
		t.Errorf("X-Api-Key header = %q, want %q", gotHeader, "test-key")
	}
	if gotAuthHeader != "" {
		t.Errorf("Authorization header = %q, want empty (Prowlarr uses X-Api-Key, not bearer auth)", gotAuthHeader)
	}
	if gotQueryParam != "" {
		t.Errorf("apikey query param = %q, want empty (key must not leak into the URL)", gotQueryParam)
	}
}

func TestSearch_SendsQueryCategoriesAndIndexerIDsParams(t *testing.T) {
	var gotQuery string
	var gotCategories, gotIndexerIDs []string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   searchPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			gotQuery = q.Get("query")
			gotCategories = q["categories"]
			gotIndexerIDs = q["indexerIds"]
			return httpmock.JSON(http.StatusOK, []map[string]any{})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{
		Query:      "Please Please Me",
		Categories: []int{3000, 3010},
		IndexerIDs: []int{1},
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if gotQuery != "Please Please Me" {
		t.Errorf("query = %q, want %q", gotQuery, "Please Please Me")
	}
	if len(gotCategories) != 2 || gotCategories[0] != "3000" || gotCategories[1] != "3010" {
		t.Errorf("categories = %v, want [3000 3010]", gotCategories)
	}
	if len(gotIndexerIDs) != 1 || gotIndexerIDs[0] != "1" {
		t.Errorf("indexerIds = %v, want [1]", gotIndexerIDs)
	}
}

func TestSearch_MapsRealProwlarrFieldSet(t *testing.T) {
	release := map[string]any{
		"guid":        "release-guid-1",
		"title":       "Some Release Title",
		"size":        123456789,
		"indexerId":   1,
		"indexer":     "SomeIndexer",
		"publishDate": "2024-05-01T12:30:00Z",
		"downloadUrl": "https://example.invalid/download",
		"magnetUrl":   "magnet:?xt=urn:btih:abc123",
		"infoUrl":     "https://example.invalid/info",
		"infoHash":    "abc123",
		"seeders":     10,
		"leechers":    2,
		"protocol":    "torrent",
		"categories":  []map[string]any{{"id": 3000, "name": "Music"}},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.JSON(http.StatusOK, []map[string]any{release}),
	})

	c := newMockedTestClient(t, rt)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}

	r := results[0]
	if r.GUID != "release-guid-1" || r.Title != "Some Release Title" {
		t.Errorf("GUID/Title = %q/%q, want release-guid-1/Some Release Title", r.GUID, r.Title)
	}
	if r.IndexerName != "SomeIndexer" {
		t.Errorf("IndexerName = %q, want SomeIndexer", r.IndexerName)
	}
	if r.Size != 123456789 {
		t.Errorf("Size = %d, want 123456789", r.Size)
	}
	if r.Protocol != ports.ProtocolTorrent {
		t.Errorf("Protocol = %q, want %q", r.Protocol, ports.ProtocolTorrent)
	}
	if r.Seeders != 10 || r.Leechers != 2 {
		t.Errorf("Seeders/Leechers = %d/%d, want 10/2", r.Seeders, r.Leechers)
	}
	if r.DownloadURL != "https://example.invalid/download" || r.MagnetURL != "magnet:?xt=urn:btih:abc123" {
		t.Errorf("DownloadURL/MagnetURL = %q/%q", r.DownloadURL, r.MagnetURL)
	}
	if r.InfoURL != "https://example.invalid/info" || r.InfoHash != "abc123" {
		t.Errorf("InfoURL/InfoHash = %q/%q", r.InfoURL, r.InfoHash)
	}
	if len(r.Categories) != 1 || r.Categories[0].ID != 3000 || r.Categories[0].Name != "Music" {
		t.Errorf("Categories = %+v, want one {3000 Music}", r.Categories)
	}
	if r.PublishDate.IsZero() {
		t.Error("PublishDate is zero, want the parsed 2024-05-01T12:30:00Z")
	}
}

func TestSearch_MalformedPublishDateDegradesToZeroTime(t *testing.T) {
	release := map[string]any{
		"guid":        "release-guid-1",
		"publishDate": "not-a-date",
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.JSON(http.StatusOK, []map[string]any{release}),
	})

	c := newMockedTestClient(t, rt)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}
	if !results[0].PublishDate.IsZero() {
		t.Errorf("PublishDate = %v, want zero time for a malformed date", results[0].PublishDate)
	}
}

func TestSearch_UnauthorizedMapsToPlainErrorNotErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.Status(http.StatusUnauthorized),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if err == nil {
		t.Fatal("Search with a 401 response returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("Search error = %v, want a plain error, not ports.ErrNotFound", err)
	}
}

func TestSearch_ForbiddenMapsToPlainErrorNotErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.Status(http.StatusForbidden),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if err == nil {
		t.Fatal("Search with a 403 response returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("Search error = %v, want a plain error, not ports.ErrNotFound", err)
	}
}

func TestSearch_ServerErrorMapsToPlainErrorNotErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.Status(http.StatusInternalServerError),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if err == nil {
		t.Fatal("Search with a 500 response returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("Search error = %v, want a plain error, not ports.ErrNotFound", err)
	}
}

func TestSearch_NotFoundMapsToErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.Status(http.StatusNotFound),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Search error = %v, want ports.ErrNotFound for a genuine 404", err)
	}
}

func TestSearch_EmptyResultIsEmptySliceNotError(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.JSON(http.StatusOK, []map[string]any{}),
	})

	c := newMockedTestClient(t, rt)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "no-match"})
	if err != nil {
		t.Fatalf("Search returned error: %v, want nil for a zero-result search", err)
	}
	if len(results) != 0 {
		t.Errorf("Search = %+v, want empty", results)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      searchPath,
		Responder: httpmock.JSON(http.StatusOK, []map[string]any{}),
	})

	c := newMockedTestClient(t, rt,
		prowlarr.WithLogger(slog.Default()),
		prowlarr.WithTracerProvider(otel.GetTracerProvider()),
		prowlarr.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "x"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
}
