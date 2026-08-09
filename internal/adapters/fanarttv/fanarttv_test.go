package fanarttv_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"purser/internal/adapters/fanarttv"
	"purser/internal/ports"
	"purser/internal/ports/fanarttvtest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

// Every fixture body below is hand-built directly from fields confirmed
// live against the real webservice.fanart.tv during this adapter's
// implementation (see fanarttv.go's package doc comment) — no API-key
// credential recording is available in this environment for genuinely
// captured golden files, the same convention theporndb_test.go uses for
// the same reason.

func newTestClient(t *testing.T, baseURL string) *fanarttv.Client {
	t.Helper()
	cfg := fanarttv.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := fanarttv.New(cfg)
	if err != nil {
		t.Fatalf("fanarttv.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...fanarttv.Option) *fanarttv.Client {
	t.Helper()
	cfg := fanarttv.DefaultConfig()
	cfg.APIKey = "test-key"
	allOpts := append([]fanarttv.Option{fanarttv.WithBaseTransport(rt)}, opts...)
	c, err := fanarttv.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("fanarttv.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_FanartTVClientContract(t *testing.T) {
	fanarttvtest.TestFanartTVClient(t, func(t *testing.T, baseURL string) ports.FanartTVClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := fanarttv.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := fanarttv.New(cfg); err == nil {
		t.Fatal("fanarttv.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := fanarttv.DefaultConfig()
	cfg.APIKey = ""
	if _, err := fanarttv.New(cfg); err == nil {
		t.Fatal("fanarttv.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/v3/music/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, map[string]any{})(req)
		},
	})

	cfg := fanarttv.DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := fanarttv.New(cfg, fanarttv.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("fanarttv.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupArtist(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want ports.ErrNotFound", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_SendsAPIKeyAsQueryParam(t *testing.T) {
	var gotKey string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/v3/music/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotKey = req.URL.Query().Get("api_key")
			return httpmock.JSON(http.StatusOK, map[string]any{})(req)
		},
	})

	cfg := fanarttv.DefaultConfig()
	cfg.APIKey = "my-secret-key"
	c, err := fanarttv.New(cfg, fanarttv.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("fanarttv.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupArtist(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want ports.ErrNotFound", err)
	}
	if gotKey != "my-secret-key" {
		t.Errorf("api_key param = %q, want %q", gotKey, "my-secret-key")
	}
}

// TestLookupArtist_MapsFullFieldSet locks in the real field set confirmed
// live against music/{mbid} for The Beatles (MBID
// b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d) during this adapter's
// implementation, including artist4kbackground — a field
// docs/technical/music-data_model.md's original research pass didn't
// document at all.
func TestLookupArtist_MapsFullFieldSet(t *testing.T) {
	body := map[string]any{
		"name":    "The Beatles",
		"mbid_id": "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d",
		"artistthumb": []map[string]any{
			{"id": "30373", "lang": "", "likes": "11", "url": "https://assets.fanart.tv/fanart/beatles-the-4facf1550225f.jpg"},
		},
		"artistbackground": []map[string]any{
			{"id": "112896", "lang": "", "likes": "11", "url": "https://assets.fanart.tv/fanart/beatles-the-53062b2a99606.jpg"},
		},
		"artist4kbackground": []map[string]any{
			{"id": "500000", "lang": "", "likes": "1", "url": "https://assets.fanart.tv/fanart/beatles-the-4k-background.jpg"},
		},
		"hdmusiclogo": []map[string]any{
			{"id": "99297", "lang": "", "likes": "17", "url": "https://assets.fanart.tv/fanart/beatles-the-5244b8bce3ac4.png"},
		},
		"musiclogo": []map[string]any{
			{"id": "6415", "lang": "", "likes": "10", "url": "https://assets.fanart.tv/fanart/the-beatles-4e0468cfd92a7.png"},
		},
		"musicbanner": []map[string]any{
			{"id": "112798", "lang": "", "likes": "6", "url": "https://assets.fanart.tv/fanart/beatles-the-5304a6cd34c6e.jpg"},
		},
		"albums": map[string]any{
			"055be730-dcad-31bf-b550-45ba9c202aa3": map[string]any{
				"albumcover":       []map[string]any{{"id": "177168", "likes": "8", "url": "https://assets.fanart.tv/fanart/the-beatles-55e7655f3f1e3.jpg"}},
				"albumcover_count": 1,
				"cdart": []map[string]any{
					{"disc": "1", "id": "223031", "likes": "8", "size": "1000", "url": "https://assets.fanart.tv/fanart/the-beatles-5978adba6ea50.png"},
				},
				"cdart_count": 1,
			},
			"00fd2a87-8e68-3dc6-b749-ed89fb82cd6a": map[string]any{
				"albumcover":       []map[string]any{{"id": "358900", "likes": "3", "url": "https://assets.fanart.tv/fanart/a-collection-of-beatles-oldies-617402430580c.jpg"}},
				"albumcover_count": 1,
			},
		},
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/v3/music/b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d",
		Responder: httpmock.JSON(http.StatusOK, body),
	})

	c := newMockedTestClient(t, rt)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}

	if a.Name != "The Beatles" || a.MBID != "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d" {
		t.Errorf("Name/MBID = %q/%q, want The Beatles/b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d", a.Name, a.MBID)
	}
	if len(a.ArtistThumb) != 1 || len(a.ArtistBackground) != 1 || len(a.Artist4KBackground) != 1 ||
		len(a.HDMusicLogo) != 1 || len(a.MusicLogo) != 1 || len(a.MusicBanner) != 1 {
		t.Errorf("Artist = %+v, want every artist-level image slot populated with one entry", a)
	}
	if len(a.Albums) != 2 {
		t.Fatalf("len(Albums) = %d, want 2", len(a.Albums))
	}
	withCDArt := a.Albums["055be730-dcad-31bf-b550-45ba9c202aa3"]
	if len(withCDArt.AlbumCover) != 1 || len(withCDArt.CDArt) != 1 || withCDArt.CDArt[0].Disc != "1" {
		t.Errorf("Albums[055be730...] = %+v, want one cover and one disc-1 cdart entry", withCDArt)
	}
	withoutCDArt := a.Albums["00fd2a87-8e68-3dc6-b749-ed89fb82cd6a"]
	if len(withoutCDArt.AlbumCover) != 1 || len(withoutCDArt.CDArt) != 0 {
		t.Errorf("Albums[00fd2a87...] = %+v, want one cover and no cdart (album has none recorded)", withoutCDArt)
	}
}

// TestLookupArtist_UnknownMBIDReturnsErrNotFoundFromEmptyBody locks in
// fanart.tv's confirmed-live not-found shape: HTTP 200 with an empty {}
// body, not a 404.
func TestLookupArtist_UnknownMBIDReturnsErrNotFoundFromEmptyBody(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/v3/music/00000000-0000-0000-0000-000000000000",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupArtist(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want wrapping ports.ErrNotFound", err)
	}
}

// TestLookupArtist_InvalidAPIKeyIsARealErrorNotErrNotFound locks in
// fanart.tv's confirmed-live 401 response for a bad key — distinct from
// the 200+{} not-found case, and must never be silently mapped to
// ports.ErrNotFound.
func TestLookupArtist_InvalidAPIKeyIsARealErrorNotErrNotFound(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/v3/music/x",
		Responder: httpmock.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid API key"}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupArtist(context.Background(), "x")
	if err == nil {
		t.Fatal("LookupArtist with an invalid API key returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Errorf("LookupArtist error = %v, must not wrap ports.ErrNotFound for a 401", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/v3/music/x",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{}),
	})

	c := newMockedTestClient(t, rt,
		fanarttv.WithLogger(slog.Default()),
		fanarttv.WithTracerProvider(otel.GetTracerProvider()),
		fanarttv.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupArtist(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want ports.ErrNotFound", err)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/v3/music/cache-test",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]any{"name": "Cached Artist", "mbid_id": "cache-test"})(req)
		},
	})

	// Rate limiting isn't what this test is about — see
	// musicbrainz_test.go's/theporndb_test.go's identical rationale.
	c := newMockedTestClient(t, rt, fanarttv.WithRateLimit(rate.Inf))
	ctx := context.Background()

	if _, err := c.LookupArtist(ctx, "cache-test"); err != nil {
		t.Fatalf("first LookupArtist returned error: %v", err)
	}
	if _, err := c.LookupArtist(ctx, "cache-test"); err != nil {
		t.Fatalf("second LookupArtist returned error: %v", err)
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
		return httpmock.JSON(http.StatusOK, map[string]any{})(req)
	}

	const n = 3
	routes := make([]httpmock.Route, n)
	for i := range n {
		// Distinct MBIDs (part of the URL path itself for fanart.tv, not
		// just a query param) so the caching transport doesn't
		// short-circuit requests 2 and 3 against request 1's cached
		// response — mirrors musicbrainz_test.go's identical rationale.
		routes[i] = httpmock.Route{
			Method:    http.MethodGet,
			Path:      fmt.Sprintf("/v3/music/rate-limit-test-%d", i),
			Responder: recorder,
		}
	}
	rt := httpmock.New(routes...)
	c := newMockedTestClient(t, rt, fanarttv.WithRateLimit(3))

	var wg sync.WaitGroup
	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mbid := fmt.Sprintf("rate-limit-test-%d", i)
			if _, err := c.LookupArtist(context.Background(), mbid); !errors.Is(err, ports.ErrNotFound) {
				t.Errorf("LookupArtist error = %v, want ports.ErrNotFound", err)
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
		Path:   "/v3/music/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"name": "x", "mbid_id": "x"})(req)
		},
	})

	c := newMockedTestClient(t, rt, fanarttv.WithRetryBaseDelay(time.Millisecond), fanarttv.WithRateLimit(rate.Inf))
	a, err := c.LookupArtist(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.Name != "x" {
		t.Errorf("Name = %q, want %q", a.Name, "x")
	}
	if calls != 3 {
		t.Errorf("Responder called %d times, want 3 (two 503s then a success)", calls)
	}
}

func TestGet_GivesUpAfter503EveryAttempt(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/v3/music/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestClient(t, rt, fanarttv.WithRetryBaseDelay(time.Millisecond), fanarttv.WithRateLimit(rate.Inf))
	if _, err := c.LookupArtist(context.Background(), "x"); err == nil {
		t.Fatal("LookupArtist with a persistent 503 returned nil error")
	}
	if calls != 4 {
		t.Errorf("Responder called %d times, want 4 (maxAttempts, then give up)", calls)
	}
}

func TestGet_RetriesTransportTimeoutThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/v3/music/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"name": "x", "mbid_id": "x"})(req)
		},
	})

	c := newMockedTestClient(t, rt, fanarttv.WithRetryBaseDelay(time.Millisecond), fanarttv.WithRateLimit(rate.Inf))
	a, err := c.LookupArtist(context.Background(), "x")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.Name != "x" {
		t.Errorf("Name = %q, want %q", a.Name, "x")
	}
	if calls != 2 {
		t.Errorf("Responder called %d times, want 2 (one simulated timeout then a success)", calls)
	}
}
