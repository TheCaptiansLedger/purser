package wikidata_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"purser/internal/adapters/wikidata"
	"purser/internal/ports"
	"purser/internal/ports/wikidatatest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

// Every fixture body below is hand-built directly from fields confirmed
// live against the real www.wikidata.org action API during this adapter's
// implementation (see wikidata.go's package doc comment).

func newTestClient(t *testing.T, baseURL string) *wikidata.Client {
	t.Helper()
	cfg := wikidata.DefaultConfig()
	cfg.BaseURL = baseURL
	c, err := wikidata.New(cfg)
	if err != nil {
		t.Fatalf("wikidata.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...wikidata.Option) *wikidata.Client {
	t.Helper()
	cfg := wikidata.DefaultConfig()
	allOpts := append([]wikidata.Option{wikidata.WithBaseTransport(rt)}, opts...)
	c, err := wikidata.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("wikidata.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_WikidataClientContract(t *testing.T) {
	wikidatatest.TestWikidataClient(t, func(t *testing.T, baseURL string) ports.WikidataClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := wikidata.DefaultConfig()
	cfg.BaseURL = ""
	if _, err := wikidata.New(cfg); err == nil {
		t.Fatal("wikidata.New with empty BaseURL returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, map[string]any{"error": map[string]any{"code": "no-such-entity"}})(req)
		},
	})

	cfg := wikidata.DefaultConfig()
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := wikidata.New(cfg, wikidata.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("wikidata.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupImage error = %v, want ports.ErrNotFound", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

// TestLookupImage_MalformedURLIsARealError locks in that a URL not ending
// in a "Q..." item ID is a real error, not silently mapped to
// ports.ErrNotFound — no request is even issued.
func TestLookupImage_MalformedURLIsARealError(t *testing.T) {
	c := newMockedTestClient(t, httpmock.New())
	_, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/not-an-entity")
	if err == nil {
		t.Fatal("LookupImage with a malformed entity URL returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("LookupImage error = %v, must not wrap ports.ErrNotFound for a malformed URL", err)
	}
}

// TestLookupImage_MultipleP18ClaimsPreserveOrder locks in that every P18
// statement is returned, in the order Wikidata's own claims array gave
// them — confirmed live, e.g. REO Speedwagon's real Q845084 carries two.
func TestLookupImage_MultipleP18ClaimsPreserveOrder(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{
			"claims": map[string]any{
				"P18": []map[string]any{
					{"mainsnak": map[string]any{"datavalue": map[string]any{"value": "REO Speedwagon.jpg"}}},
					{"mainsnak": map[string]any{"datavalue": map[string]any{"value": "REO Speedwagon at Red Rocks July 2010 (cropped).jpg"}}},
				},
			},
		}),
	})

	c := newMockedTestClient(t, rt)
	images, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q845084")
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2", len(images))
	}
	if images[0].URL != "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon.jpg" {
		t.Errorf("images[0].URL = %q, want the Special:FilePath URL for REO_Speedwagon.jpg", images[0].URL)
	}
	if images[1].URL != "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon_at_Red_Rocks_July_2010_%28cropped%29.jpg" {
		t.Errorf("images[1].URL = %q, want the escaped Special:FilePath URL for the second filename", images[1].URL)
	}
}

// TestLookupImage_NoSuchEntityIsErrNotFound locks in Wikidata's confirmed
// live shape for a syntactically valid but nonexistent QID: HTTP 200 with
// an {"error":{"code":"no-such-entity"}} body, not a 404.
func TestLookupImage_NoSuchEntityIsErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/w/api.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"error": map[string]any{"code": "no-such-entity"}}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q999999999")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupImage error = %v, want wrapping ports.ErrNotFound", err)
	}
}

// TestLookupImage_OtherErrorCodeIsARealError locks in that a body-level
// error other than "no-such-entity" (e.g. a malformed request the adapter
// itself built) is a real error, not silently mapped to ports.ErrNotFound.
func TestLookupImage_OtherErrorCodeIsARealError(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/w/api.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"error": map[string]any{"code": "param-invalid"}}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1")
	if err == nil {
		t.Fatal("LookupImage with a param-invalid error returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("LookupImage error = %v, must not wrap ports.ErrNotFound for a non-no-such-entity error", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/w/api.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"error": map[string]any{"code": "no-such-entity"}}),
	})

	c := newMockedTestClient(t, rt,
		wikidata.WithLogger(slog.Default()),
		wikidata.WithTracerProvider(otel.GetTracerProvider()),
		wikidata.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupImage error = %v, want ports.ErrNotFound", err)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]any{
				"claims": map[string]any{"P18": []map[string]any{{"mainsnak": map[string]any{"datavalue": map[string]any{"value": "cache-test.jpg"}}}}},
			})(req)
		},
	})

	// Rate limiting isn't what this test is about — see
	// fanarttv_test.go's identical rationale.
	c := newMockedTestClient(t, rt, wikidata.WithRateLimit(rate.Inf))
	ctx := context.Background()

	if _, err := c.LookupImage(ctx, "https://www.wikidata.org/wiki/Q12345"); err != nil {
		t.Fatalf("first LookupImage returned error: %v", err)
	}
	if _, err := c.LookupImage(ctx, "https://www.wikidata.org/wiki/Q12345"); err != nil {
		t.Fatalf("second LookupImage returned error: %v", err)
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
		return httpmock.JSON(http.StatusOK, map[string]any{"error": map[string]any{"code": "no-such-entity"}})(req)
	}

	const n = 3
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: "/w/api.php", Responder: recorder})
	c := newMockedTestClient(t, rt, wikidata.WithRateLimit(3))

	var wg sync.WaitGroup
	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			url := "https://www.wikidata.org/wiki/Q" + string(rune('1'+i))
			if _, err := c.LookupImage(context.Background(), url); !errors.Is(err, ports.ErrNotFound) {
				t.Errorf("LookupImage error = %v, want ports.ErrNotFound", err)
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

func TestGet_RetriesOn503ThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{
				"claims": map[string]any{"P18": []map[string]any{{"mainsnak": map[string]any{"datavalue": map[string]any{"value": "x.jpg"}}}}},
			})(req)
		},
	})

	c := newMockedTestClient(t, rt, wikidata.WithRetryBaseDelay(time.Millisecond), wikidata.WithRateLimit(rate.Inf))
	images, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1")
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("len(images) = %d, want 1", len(images))
	}
	if calls != 3 {
		t.Errorf("Responder called %d times, want 3 (two 503s then a success)", calls)
	}
}

func TestGet_GivesUpAfter503EveryAttempt(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestClient(t, rt, wikidata.WithRetryBaseDelay(time.Millisecond), wikidata.WithRateLimit(rate.Inf))
	if _, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1"); err == nil {
		t.Fatal("LookupImage with a persistent 503 returned nil error")
	}
	if calls != 4 {
		t.Errorf("Responder called %d times, want 4 (maxAttempts, then give up)", calls)
	}
}

func TestGet_RetriesTransportTimeoutThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/w/api.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{
				"claims": map[string]any{"P18": []map[string]any{{"mainsnak": map[string]any{"datavalue": map[string]any{"value": "x.jpg"}}}}},
			})(req)
		},
	})

	c := newMockedTestClient(t, rt, wikidata.WithRetryBaseDelay(time.Millisecond), wikidata.WithRateLimit(rate.Inf))
	images, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q1")
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("len(images) = %d, want 1", len(images))
	}
	if calls != 2 {
		t.Errorf("Responder called %d times, want 2 (one simulated timeout then a success)", calls)
	}
}
