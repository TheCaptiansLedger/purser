package stashdb_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"purser/internal/adapters/stashdb"
	"purser/internal/ports"
	"purser/internal/ports/stashdbtest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

// graphqlPath is defaultBaseURL's path component — every request this
// adapter issues (GET, query/variables as URL params) hits this single
// path, unlike MusicBrainz's one-path-per-resource REST API.
const graphqlPath = "/graphql"

func newTestClient(t *testing.T, baseURL string) *stashdb.Client {
	t.Helper()
	cfg := stashdb.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := stashdb.New(cfg)
	if err != nil {
		t.Fatalf("stashdb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...stashdb.Option) *stashdb.Client {
	t.Helper()
	cfg := stashdb.DefaultConfig()
	cfg.APIKey = "test-key"
	allOpts := append([]stashdb.Option{stashdb.WithBaseTransport(rt)}, opts...)
	c, err := stashdb.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("stashdb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// No API key or credential recording is available in this environment
// (StashDB requires a real account), so unlike MusicBrainz's
// testdata/*.json golden files, the response bodies in the tests below are
// hand-built directly from stash-box's published schema
// (https://github.com/stashapp/stash-box) rather than genuinely recorded —
// the same convention AcoustID's adapter tests already use for the same
// reason.

func TestClient_StashDBClientContract(t *testing.T) {
	stashdbtest.TestStashDBClient(t, func(t *testing.T, baseURL string) ports.StashDBClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := stashdb.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := stashdb.New(cfg); err == nil {
		t.Fatal("stashdb.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := stashdb.DefaultConfig()
	cfg.APIKey = ""
	if _, err := stashdb.New(cfg); err == nil {
		t.Fatal("stashdb.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": nil}})(req)
		},
	})

	cfg := stashdb.DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := stashdb.New(cfg, stashdb.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("stashdb.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupStudio(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupStudio error = %v, want ports.ErrNotFound", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_SendsApiKeyHeaderNotQueryParamOrBearer(t *testing.T) {
	var gotHeader string
	var gotAuthHeader string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotHeader = req.Header.Get("ApiKey")
			gotAuthHeader = req.Header.Get("Authorization")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": nil}})(req)
		},
	})

	cfg := stashdb.DefaultConfig()
	cfg.APIKey = "my-real-key"
	c, err := stashdb.New(cfg, stashdb.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("stashdb.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupStudio(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupStudio error = %v, want ports.ErrNotFound", err)
	}

	if gotHeader != "my-real-key" {
		t.Errorf("ApiKey header = %q, want %q", gotHeader, "my-real-key")
	}
	if gotAuthHeader != "" {
		t.Errorf("Authorization header = %q, want empty (StashDB uses ApiKey, not bearer auth)", gotAuthHeader)
	}
}

func TestLookupPerformer_MapsFullFieldSet(t *testing.T) {
	performer := map[string]any{
		"id":                "11111111-1111-1111-1111-111111111111",
		"name":              "Known Performer",
		"disambiguation":    "the one from Known Studio",
		"aliases":           []string{"Alias One"},
		"gender":            "FEMALE",
		"urls":              []map[string]any{{"url": "https://example.invalid/p", "site": map[string]any{"id": "s1", "name": "Twitter", "url": "https://twitter.com"}}},
		"birth_date":        "1990-01-01",
		"ethnicity":         "CAUCASIAN",
		"country":           "US",
		"eye_color":         "BLUE",
		"hair_color":        "BLONDE",
		"height":            170,
		"cup_size":          "C",
		"band_size":         32,
		"waist_size":        26,
		"hip_size":          36,
		"breast_type":       "NATURAL",
		"career_start_year": 2015,
		"career_end_year":   0,
		"images":            []map[string]any{{"id": "img1", "url": "https://example.invalid/img.jpg", "width": 100, "height": 200}},
		"is_favorite":       false,
		"deleted":           false,
		"merged_ids":        []string{},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      graphqlPath,
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findPerformer": performer}}),
	})

	c := newMockedTestClient(t, rt)
	p, err := c.LookupPerformer(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}

	if p.Name != "Known Performer" || p.Disambiguation != "the one from Known Studio" {
		t.Errorf("Name/Disambiguation = %q/%q, want Known Performer/the one from Known Studio", p.Name, p.Disambiguation)
	}
	if len(p.Aliases) != 1 || p.Aliases[0] != "Alias One" {
		t.Errorf("Aliases = %v, want [Alias One]", p.Aliases)
	}
	if len(p.URLs) != 1 || p.URLs[0].Site.Name != "Twitter" {
		t.Errorf("URLs = %+v, want one URL with Site.Name=Twitter", p.URLs)
	}
	if p.BirthDate != "1990-01-01" || p.Height != 170 || p.CupSize != "C" {
		t.Errorf("BirthDate/Height/CupSize = %q/%d/%q, want 1990-01-01/170/C", p.BirthDate, p.Height, p.CupSize)
	}
	if len(p.Images) != 1 || p.Images[0].Width != 100 {
		t.Errorf("Images = %+v, want one image with Width=100", p.Images)
	}
}

func TestLookupScene_MapsNestedStudioPerformersAndFingerprints(t *testing.T) {
	scene := map[string]any{
		"id":           "33333333-3333-3333-3333-333333333333",
		"title":        "Known Scene",
		"details":      "a scene",
		"release_date": "2020-06-15",
		"urls":         []map[string]any{},
		"studio":       map[string]any{"id": "22222222-2222-2222-2222-222222222222", "name": "Known Studio", "urls": []map[string]any{}, "child_studios": []map[string]any{}, "images": []map[string]any{}},
		"tags":         []map[string]any{{"id": "t1", "name": "Tag One", "aliases": []string{}}},
		"images":       []map[string]any{},
		"performers": []map[string]any{
			{"performer": map[string]any{"id": "11111111-1111-1111-1111-111111111111", "name": "Known Performer", "aliases": []string{}, "urls": []map[string]any{}, "images": []map[string]any{}, "merged_ids": []string{}}, "as": "Stage Name"},
		},
		"fingerprints": []map[string]any{
			{"hash": "aaaaaaaaaaaaaaaa", "algorithm": "OSHASH", "duration": 3600, "submissions": 2, "user_submitted": true},
		},
		"duration": 3600,
		"director": "Some Director",
		"code":     "ABC-123",
		"deleted":  false,
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      graphqlPath,
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findScene": scene}}),
	})

	c := newMockedTestClient(t, rt)
	s, err := c.LookupScene(context.Background(), "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}

	if s.Studio == nil || s.Studio.Name != "Known Studio" {
		t.Errorf("Studio = %+v, want Name=Known Studio", s.Studio)
	}
	if len(s.Performers) != 1 || s.Performers[0].As != "Stage Name" || s.Performers[0].Performer.Name != "Known Performer" {
		t.Errorf("Performers = %+v, want one appearance As=Stage Name of Known Performer", s.Performers)
	}
	if len(s.Fingerprints) != 1 || s.Fingerprints[0].Algorithm != ports.FingerprintAlgorithmOSHash || s.Fingerprints[0].Duration != 3600 {
		t.Errorf("Fingerprints = %+v, want one OSHASH fingerprint with Duration=3600", s.Fingerprints)
	}
	if len(s.Tags) != 1 || s.Tags[0].Name != "Tag One" {
		t.Errorf("Tags = %+v, want one tag named Tag One", s.Tags)
	}
	if s.Code != "ABC-123" || s.Director != "Some Director" {
		t.Errorf("Code/Director = %q/%q, want ABC-123/Some Director", s.Code, s.Director)
	}
}

func TestFindScenesByFingerprints_SendsSingleFileBatchedVariables(t *testing.T) {
	var gotVariables string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotVariables = req.URL.Query().Get("variables")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findScenesBySceneFingerprints": [][]map[string]any{}}})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.FindScenesByFingerprints(context.Background(), []ports.SceneFingerprint{
		{Hash: "aaaa", Algorithm: ports.FingerprintAlgorithmOSHash},
		{Hash: "bbbb", Algorithm: ports.FingerprintAlgorithmPHash},
	}); err != nil {
		t.Fatalf("FindScenesByFingerprints returned error: %v", err)
	}

	var parsed struct {
		Fingerprints [][]ports.SceneFingerprint `json:"fingerprints"`
	}
	if err := json.Unmarshal([]byte(gotVariables), &parsed); err != nil {
		t.Fatalf("decoding sent variables %q: %v", gotVariables, err)
	}
	if len(parsed.Fingerprints) != 1 || len(parsed.Fingerprints[0]) != 2 {
		t.Fatalf("Fingerprints = %+v, want exactly one inner list of 2 (single-file batch)", parsed.Fingerprints)
	}
	if parsed.Fingerprints[0][0].Algorithm != ports.FingerprintAlgorithmOSHash || parsed.Fingerprints[0][1].Algorithm != ports.FingerprintAlgorithmPHash {
		t.Errorf("Fingerprints[0] = %+v, want OSHASH then PHASH", parsed.Fingerprints[0])
	}
}

func TestSearchScenes_SendsTermVariable(t *testing.T) {
	var gotVariables string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			gotVariables = req.URL.Query().Get("variables")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"searchScene": []map[string]any{}}})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.SearchScenes(context.Background(), "Please Please Me"); err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}

	want := `{"term":"Please Please Me"}`
	if gotVariables != want {
		t.Errorf("variables = %q, want %q", gotVariables, want)
	}
}

func TestDoQuery_MapsGraphQLErrorsToError(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      graphqlPath,
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"errors": []map[string]any{{"message": "not authorized"}}, "data": nil}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupPerformer(context.Background(), "x")
	if err == nil {
		t.Fatal("LookupPerformer returned nil error, want an error for a GraphQL errors[] response")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("LookupPerformer error = %v, want NOT ports.ErrNotFound (a GraphQL error is a real failure, not a not-found signal)", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      graphqlPath,
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": nil}}),
	})

	c := newMockedTestClient(t, rt,
		stashdb.WithLogger(slog.Default()),
		stashdb.WithTracerProvider(otel.GetTracerProvider()),
		stashdb.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupStudio(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupStudio error = %v, want ports.ErrNotFound", err)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": map[string]any{"id": "cache-test", "name": "Cached Studio"}}})(req)
		},
	})

	// Rate limiting isn't what this test is about — see musicbrainz_test.go's
	// identical rationale.
	c := newMockedTestClient(t, rt, stashdb.WithRateLimit(rate.Inf))
	ctx := context.Background()

	if _, err := c.LookupStudio(ctx, "cache-test"); err != nil {
		t.Fatalf("first LookupStudio returned error: %v", err)
	}
	if _, err := c.LookupStudio(ctx, "cache-test"); err != nil {
		t.Fatalf("second LookupStudio returned error: %v", err)
	}

	if hits != 1 {
		t.Errorf("server received %d requests, want 1 (second call should be served from cache)", hits)
	}
}

func TestClient_RateLimiterSerializesConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	var arrivals []time.Time
	recorder := func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		arrivals = append(arrivals, time.Now())
		mu.Unlock()
		return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": nil}})(req)
	}

	const n = 3
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: graphqlPath, Responder: recorder})
	c := newMockedTestClient(t, rt, stashdb.WithRateLimit(3))

	var wg sync.WaitGroup
	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := c.LookupStudio(context.Background(), fmt.Sprintf("rate-limit-test-%d", i)); !errors.Is(err, ports.ErrNotFound) {
				t.Errorf("LookupStudio error = %v, want ports.ErrNotFound", err)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// 3 req/sec with burst 1 means n requests take at least (n-1)/3 sec,
	// serialized. Allow slack for scheduling jitter.
	minExpected := time.Duration(n-1) * 300 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("elapsed = %v, want at least %v (rate limiter should serialize concurrent requests)", elapsed, minExpected)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(arrivals) != n {
		t.Fatalf("server received %d requests, want %d", len(arrivals), n)
	}
}

func TestDoQuery_RetriesOn503ThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": map[string]any{"id": "x", "name": "y"}}})(req)
		},
	})

	c := newMockedTestClient(t, rt, stashdb.WithRetryBaseDelay(time.Millisecond), stashdb.WithRateLimit(rate.Inf))
	s, err := c.LookupStudio(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupStudio returned error: %v", err)
	}
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
	if calls != 3 {
		t.Errorf("Responder called %d times, want 3 (two 503s then a success)", calls)
	}
}

func TestDoQuery_GivesUpAfter503EveryAttempt(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestClient(t, rt, stashdb.WithRetryBaseDelay(time.Millisecond), stashdb.WithRateLimit(rate.Inf))
	if _, err := c.LookupStudio(context.Background(), "x"); err == nil {
		t.Fatal("LookupStudio with a persistent 503 returned nil error")
	}
	if calls != 4 {
		t.Errorf("Responder called %d times, want 4 (maxAttempts, then give up)", calls)
	}
}

func TestDoQuery_DoesNotRetryGraphQLError(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.JSON(http.StatusOK, map[string]any{"errors": []map[string]any{{"message": "not authorized"}}, "data": nil})(req)
		},
	})

	c := newMockedTestClient(t, rt, stashdb.WithRetryBaseDelay(time.Millisecond), stashdb.WithRateLimit(rate.Inf))
	if _, err := c.LookupStudio(context.Background(), "x"); err == nil {
		t.Fatal("LookupStudio returned nil error, want an error for a GraphQL errors[] response")
	}
	if calls != 1 {
		t.Errorf("Responder called %d times, want 1 — a GraphQL error is a real answer, not a transient failure", calls)
	}
}

func TestDoQuery_RetriesTransportTimeoutThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   graphqlPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"findStudio": map[string]any{"id": "x", "name": "y"}}})(req)
		},
	})

	c := newMockedTestClient(t, rt, stashdb.WithRetryBaseDelay(time.Millisecond), stashdb.WithRateLimit(rate.Inf))
	s, err := c.LookupStudio(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupStudio returned error: %v", err)
	}
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
	if calls != 2 {
		t.Errorf("Responder called %d times, want 2 (one simulated timeout then a success)", calls)
	}
}
