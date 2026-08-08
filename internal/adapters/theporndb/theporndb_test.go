package theporndb_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"purser/internal/adapters/theporndb"
	"purser/internal/ports"
	"purser/internal/ports/theporndbtest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

// Every fixture body below is hand-built directly from fields confirmed
// live against the real api.theporndb.net during this adapter's
// implementation (see theporndb.go's package doc comment) — no API-key
// credential recording is available in this environment for genuinely
// captured golden files, the same convention stashdb_test.go already uses
// for the same reason.

func newTestClient(t *testing.T, baseURL string) *theporndb.Client {
	t.Helper()
	cfg := theporndb.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := theporndb.New(cfg)
	if err != nil {
		t.Fatalf("theporndb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...theporndb.Option) *theporndb.Client {
	t.Helper()
	cfg := theporndb.DefaultConfig()
	cfg.APIKey = "test-key"
	allOpts := append([]theporndb.Option{theporndb.WithBaseTransport(rt)}, opts...)
	c, err := theporndb.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("theporndb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_ThePornDBClientContract(t *testing.T) {
	theporndbtest.TestThePornDBClient(t, func(t *testing.T, baseURL string) ports.ThePornDBClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := theporndb.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := theporndb.New(cfg); err == nil {
		t.Fatal("theporndb.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := theporndb.DefaultConfig()
	cfg.APIKey = ""
	if _, err := theporndb.New(cfg); err == nil {
		t.Fatal("theporndb.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.Status(http.StatusNotFound)(req)
		},
	})

	cfg := theporndb.DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := theporndb.New(cfg, theporndb.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("theporndb.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupScene(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupScene error = %v, want ports.ErrNotFound", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_SendsBearerAuthorizationHeader(t *testing.T) {
	var gotAuth string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			return httpmock.Status(http.StatusNotFound)(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.LookupScene(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupScene error = %v, want ports.ErrNotFound", err)
	}

	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-key")
	}
}

func TestLookupPerformer_MapsFullFieldSet(t *testing.T) {
	performer := map[string]any{
		"id":             "26d101c0-1e23-4e1f-ac12-8c30e0e2f451",
		"_id":            83047,
		"slug":           "riley-reid",
		"name":           "Riley Reid",
		"full_name":      "Riley Reid",
		"disambiguation": nil,
		"bio":            "a bio",
		"rating":         4.68,
		"is_parent":      true,
		"image":          "https://cdn.theporndb.net/performer/riley-reid.webp",
		"thumbnail":      "https://cdn.theporndb.net/performer/riley-reid-thumb.jpg",
		"face":           "https://thumb.theporndb.net/riley-reid-face.webp",
		"posters":        []map[string]any{{"id": 12177381, "url": "https://cdn.theporndb.net/performer/riley-reid.webp", "size": 549632, "order": 1}},
		"aliases":        []string{"Riley474", "Riley"},
		"extras": map[string]any{
			"gender":             "Female",
			"birthday":           "1991-07-09",
			"birthday_timestamp": 679017600,
			"birthplace":         "Loxahatchee, FL, USA",
			"birthplace_code":    "US",
			"ethnicity":          "Caucasian",
			"hair_colour":        "Brunette",
			"eye_colour":         "Green",
			"height":             "160cm",
			"cupsize":            "32A",
			"career_start_year":  2010,
			"links": map[string]any{
				"IAFD":    "https://www.iafd.com/person.rme/id=x",
				"StashDB": "https://stashdb.org/performers/90a42491-f3f6-4764-8da8-564be11140f6",
			},
		},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/performers/26d101c0-1e23-4e1f-ac12-8c30e0e2f451",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": performer}),
	})

	c := newMockedTestClient(t, rt)
	p, err := c.LookupPerformer(context.Background(), "26d101c0-1e23-4e1f-ac12-8c30e0e2f451")
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}

	if p.Name != "Riley Reid" || p.FullName != "Riley Reid" {
		t.Errorf("Name/FullName = %q/%q, want Riley Reid/Riley Reid", p.Name, p.FullName)
	}
	if !p.IsParent {
		t.Error("IsParent = false, want true")
	}
	if len(p.Aliases) != 2 || p.Aliases[0] != "Riley474" {
		t.Errorf("Aliases = %v, want [Riley474 Riley]", p.Aliases)
	}
	if len(p.Posters) != 1 || p.Posters[0].Order != 1 {
		t.Errorf("Posters = %+v, want one poster with Order=1", p.Posters)
	}
	if p.Extras.Gender != "Female" || p.Extras.CareerStartYear != 2010 {
		t.Errorf("Extras.Gender/CareerStartYear = %q/%d, want Female/2010", p.Extras.Gender, p.Extras.CareerStartYear)
	}
	if p.Extras.Links["StashDB"] != "https://stashdb.org/performers/90a42491-f3f6-4764-8da8-564be11140f6" {
		t.Errorf("Extras.Links[StashDB] = %q, want the cross-referenced StashDB URL", p.Extras.Links["StashDB"])
	}
}

// TestLookupPerformer_TreatsEmptyArrayLinksAsEmptyMap locks in a real
// live-API discrepancy caught by live_test.go during this adapter's
// implementation: a performer with no external links serializes "extras.links"
// as a bare JSON array ([]), not an empty object ({}) — a PHP/Laravel
// empty-associative-array quirk. See ports.TPDBLinks' doc comment.
func TestLookupPerformer_TreatsEmptyArrayLinksAsEmptyMap(t *testing.T) {
	performer := map[string]any{
		"id":   "no-links-performer",
		"name": "No Links Performer",
		"extras": map[string]any{
			"gender": "Female",
			"links":  []any{},
		},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/performers/no-links-performer",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": performer}),
	})

	c := newMockedTestClient(t, rt)
	p, err := c.LookupPerformer(context.Background(), "no-links-performer")
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}
	if len(p.Extras.Links) != 0 {
		t.Errorf("Extras.Links = %+v, want empty", p.Extras.Links)
	}
}

func TestLookupScene_MapsNestedSiteTagsPerformersAndHashes(t *testing.T) {
	scene := map[string]any{
		"id":          "848fda1b-b133-48eb-886f-85016e3fddce",
		"_id":         5213763,
		"title":       "Known Scene",
		"type":        "Scene",
		"slug":        "known-scene",
		"external_id": "riley-reid",
		"sku":         nil,
		"description": "",
		"date":        "2024-07-09",
		"url":         "https://example.invalid/scene",
		"duration":    306,
		"site":        map[string]any{"uuid": "6b803b28-d0b1-406c-9cb3-8689846994ec", "id": 8107, "name": "Subby Hubby", "short_name": "subbyhubby", "url": "https://subbyhubby.com"},
		"performers": []map[string]any{
			{"id": "b376f671-4a63-4641-b861-efe15befe23d", "name": "Riley Reid", "is_parent": false},
		},
		"tags":      []map[string]any{{"id": 856, "uuid": "7dee1116-0a15-4e7d-a2ec-f75f72f1a804", "name": "Amateur"}},
		"directors": []map[string]any{{"id": 1046, "uuid": "6544f953-01d1-40c6-82b7-c71078f69ceb", "name": "Julia Grandi", "slug": "julia-grandi"}},
		"hashes": []map[string]any{
			{"id": 17626136, "hash": "6153b0b63569d23c", "type": "OSHASH", "duration": 306, "submissions": 1},
			{"id": 17626135, "hash": "b4b1c73f19e18ac8", "type": "PHASH", "duration": 306, "submissions": 1},
		},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/scenes/848fda1b-b133-48eb-886f-85016e3fddce",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": scene}),
	})

	c := newMockedTestClient(t, rt)
	s, err := c.LookupScene(context.Background(), "848fda1b-b133-48eb-886f-85016e3fddce")
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}

	if s.Site == nil || s.Site.Name != "Subby Hubby" {
		t.Errorf("Site = %+v, want Name=Subby Hubby", s.Site)
	}
	if len(s.Performers) != 1 || s.Performers[0].Name != "Riley Reid" {
		t.Errorf("Performers = %+v, want one performer named Riley Reid", s.Performers)
	}
	if len(s.Tags) != 1 || s.Tags[0].Name != "Amateur" {
		t.Errorf("Tags = %+v, want one tag named Amateur", s.Tags)
	}
	if len(s.Directors) != 1 || s.Directors[0].Name != "Julia Grandi" {
		t.Errorf("Directors = %+v, want one director named Julia Grandi", s.Directors)
	}
	if len(s.Hashes) != 2 || s.Hashes[0].Type != "OSHASH" || s.Hashes[1].Type != "PHASH" {
		t.Errorf("Hashes = %+v, want one OSHASH then one PHASH", s.Hashes)
	}
}

func TestLookupScene_MapsJAVFields(t *testing.T) {
	scene := map[string]any{
		"id":          "jav-scene-id",
		"title":       "Ssis-001",
		"type":        "JAV",
		"external_id": "ssis-001",
		"sku":         "ssis00001",
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/scenes/jav-scene-id",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"data": scene}),
	})

	c := newMockedTestClient(t, rt)
	s, err := c.LookupScene(context.Background(), "jav-scene-id")
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}
	if s.Type != "JAV" || s.SKU != "ssis00001" || s.ExternalID != "ssis-001" {
		t.Errorf("Type/SKU/ExternalID = %q/%q/%q, want JAV/ssis00001/ssis-001", s.Type, s.SKU, s.ExternalID)
	}
}

func TestSearchPerformers_SendsQueryParam(t *testing.T) {
	var gotQuery string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/performers",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotQuery = req.URL.Query().Get("q")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": []map[string]any{}})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.SearchPerformers(context.Background(), "Riley Reid"); err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if gotQuery != "Riley Reid" {
		t.Errorf("q param = %q, want %q", gotQuery, "Riley Reid")
	}
}

func TestResolveJAVCode_SendsParseParamAndReturnsRankedList(t *testing.T) {
	var gotParse string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/jav",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotParse = req.URL.Query().Get("parse")
			return httpmock.JSON(http.StatusOK, map[string]any{"data": []map[string]any{
				{"id": "a", "title": "Ssis-001", "type": "JAV", "external_id": "ssis-001"},
				{"id": "b", "title": "Some Other Loosely-Matched Title", "type": "JAV", "external_id": "other-001"},
			}})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	scenes, err := c.ResolveJAVCode(context.Background(), "SSIS-001")
	if err != nil {
		t.Fatalf("ResolveJAVCode returned error: %v", err)
	}
	if gotParse != "SSIS-001" {
		t.Errorf("parse param = %q, want %q", gotParse, "SSIS-001")
	}
	// The provider's own order must be preserved verbatim — never
	// re-sorted or filtered down to a single "best" match, per
	// ADR-0027's read-only-passthrough rule.
	if len(scenes) != 2 || scenes[0].ID != "a" || scenes[1].ID != "b" {
		t.Errorf("ResolveJAVCode = %+v, want [a b] in the order the fixture returned them", scenes)
	}
}

func TestLookupSceneByHash_UsesDedicatedHashPath(t *testing.T) {
	var gotPath string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/hash/6153b0b63569d23c",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"id": "x", "title": "y"}})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	s, err := c.LookupSceneByHash(context.Background(), "6153b0b63569d23c")
	if err != nil {
		t.Fatalf("LookupSceneByHash returned error: %v", err)
	}
	if gotPath != "/scenes/hash/6153b0b63569d23c" {
		t.Errorf("path = %q, want %q", gotPath, "/scenes/hash/6153b0b63569d23c")
	}
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
}

func TestDoRequest_MapsNotFoundBodyToErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/performers/unknown",
		Responder: httpmock.JSON(http.StatusNotFound, map[string]any{"message": "performer not found"}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupPerformer(context.Background(), "unknown")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupPerformer error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/scenes/x",
		Responder: httpmock.Status(http.StatusNotFound),
	})

	c := newMockedTestClient(t, rt,
		theporndb.WithLogger(slog.Default()),
		theporndb.WithTracerProvider(otel.GetTracerProvider()),
		theporndb.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupScene(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupScene error = %v, want ports.ErrNotFound", err)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/cache-test",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"id": "cache-test", "title": "Cached Scene"}})(req)
		},
	})

	// Rate limiting isn't what this test is about — see
	// musicbrainz_test.go's/stashdb_test.go's identical rationale.
	c := newMockedTestClient(t, rt, theporndb.WithRateLimit(rate.Inf))
	ctx := context.Background()

	if _, err := c.LookupScene(ctx, "cache-test"); err != nil {
		t.Fatalf("first LookupScene returned error: %v", err)
	}
	if _, err := c.LookupScene(ctx, "cache-test"); err != nil {
		t.Fatalf("second LookupScene returned error: %v", err)
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
		return httpmock.Status(http.StatusNotFound)(req)
	}

	const n = 3
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: "/performers/rate-limit-test", Responder: recorder})
	c := newMockedTestClient(t, rt, theporndb.WithRateLimit(3))

	var wg sync.WaitGroup
	start := time.Now()
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.LookupPerformer(context.Background(), "rate-limit-test"); !errors.Is(err, ports.ErrNotFound) {
				t.Errorf("LookupPerformer error = %v, want ports.ErrNotFound", err)
			}
		}()
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

func TestDoRequest_RetriesOn503ThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"id": "x", "title": "y"}})(req)
		},
	})

	c := newMockedTestClient(t, rt, theporndb.WithRetryBaseDelay(time.Millisecond), theporndb.WithRateLimit(rate.Inf))
	s, err := c.LookupScene(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
	if calls != 3 {
		t.Errorf("Responder called %d times, want 3 (two 503s then a success)", calls)
	}
}

func TestDoRequest_GivesUpAfter503EveryAttempt(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestClient(t, rt, theporndb.WithRetryBaseDelay(time.Millisecond), theporndb.WithRateLimit(rate.Inf))
	if _, err := c.LookupScene(context.Background(), "x"); err == nil {
		t.Fatal("LookupScene with a persistent 503 returned nil error")
	}
	if calls != 4 {
		t.Errorf("Responder called %d times, want 4 (maxAttempts, then give up)", calls)
	}
}

func TestDoRequest_DoesNotRetry404(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.JSON(http.StatusNotFound, map[string]any{"message": "scene not found"})(req)
		},
	})

	c := newMockedTestClient(t, rt, theporndb.WithRetryBaseDelay(time.Millisecond), theporndb.WithRateLimit(rate.Inf))
	if _, err := c.LookupScene(context.Background(), "x"); err == nil {
		t.Fatal("LookupScene returned nil error, want an error for a 404 response")
	}
	if calls != 1 {
		t.Errorf("Responder called %d times, want 1 — a 404 is a real answer, not a transient failure", calls)
	}
}

func TestDoRequest_RetriesTransportTimeoutThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/scenes/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"data": map[string]any{"id": "x", "title": "y"}})(req)
		},
	})

	c := newMockedTestClient(t, rt, theporndb.WithRetryBaseDelay(time.Millisecond), theporndb.WithRateLimit(rate.Inf))
	s, err := c.LookupScene(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
	if calls != 2 {
		t.Errorf("Responder called %d times, want 2 (one simulated timeout then a success)", calls)
	}
}
