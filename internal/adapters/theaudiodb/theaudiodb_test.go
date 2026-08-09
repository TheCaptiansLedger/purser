package theaudiodb_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"purser/internal/adapters/theaudiodb"
	"purser/internal/ports"
	"purser/internal/ports/theaudiodbtest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

// Every fixture body below is hand-built directly from fields confirmed
// live against the real theaudiodb.com during this adapter's
// implementation (see theaudiodb.go's package doc comment) — no
// API-key credential recording is available in this environment for
// genuinely captured golden files, the same convention theporndb_test.go
// uses for the same reason.

func newTestClient(t *testing.T, baseURL string) *theaudiodb.Client {
	t.Helper()
	cfg := theaudiodb.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := theaudiodb.New(cfg)
	if err != nil {
		t.Fatalf("theaudiodb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...theaudiodb.Option) *theaudiodb.Client {
	t.Helper()
	cfg := theaudiodb.DefaultConfig()
	cfg.APIKey = "test-key"
	allOpts := append([]theaudiodb.Option{theaudiodb.WithBaseTransport(rt)}, opts...)
	c, err := theaudiodb.New(cfg, allOpts...)
	if err != nil {
		t.Fatalf("theaudiodb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_TheAudioDBClientContract(t *testing.T) {
	theaudiodbtest.TestTheAudioDBClient(t, func(t *testing.T, baseURL string) ports.TheAudioDBClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := theaudiodb.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := theaudiodb.New(cfg); err == nil {
		t.Fatal("theaudiodb.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := theaudiodb.DefaultConfig()
	cfg.APIKey = ""
	if _, err := theaudiodb.New(cfg); err == nil {
		t.Fatal("theaudiodb.New with empty APIKey returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/api/v1/json/test-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, map[string]any{"artists": nil})(req)
		},
	})

	cfg := theaudiodb.DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := theaudiodb.New(cfg, theaudiodb.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("theaudiodb.New returned error: %v", err)
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

func TestClient_SendsAPIKeyAsURLPathSegment(t *testing.T) {
	var gotPath string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/api/v1/json/my-secret-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			return httpmock.JSON(http.StatusOK, map[string]any{"artists": nil})(req)
		},
	})

	cfg := theaudiodb.DefaultConfig()
	cfg.APIKey = "my-secret-key"
	c, err := theaudiodb.New(cfg, theaudiodb.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("theaudiodb.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupArtist(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want ports.ErrNotFound", err)
	}
	if gotPath != "/api/v1/json/my-secret-key/artist-mb.php" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/json/my-secret-key/artist-mb.php")
	}
}

// TestLookupArtist_MapsFullFieldSet locks in the real field set confirmed
// live against artist-mb.php?i={mbid} for The Beatles (MBID
// b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d) during this adapter's
// implementation.
func TestLookupArtist_MapsFullFieldSet(t *testing.T) {
	artist := map[string]any{
		"idArtist":           "111247",
		"strArtist":          "The Beatles",
		"strArtistAlternate": "Beatles",
		"strLabel":           "EMI",
		"intFormedYear":      "1957",
		"intBornYear":        "1957",
		"intDiedYear":        "1970",
		"intPopularity":      "85",
		"intFollowers":       "29659769",
		"intMembers":         "4",
		"intCharted":         "7",
		"strDisbanded":       "Yes",
		"strStyle":           "Rock/Pop",
		"strGenre":           "Pop-Rock",
		"strMood":            "Happy",
		"strGender":          "Male",
		"strCountry":         "Liverpool, England",
		"strCountryCode":     "GB",
		"strISNIcode":        nil,
		"strWebsite":         "",
		"strFacebook":        "",
		"strTwitter":         "1",
		"strBiography":       "The Beatles were an English rock band...",
		"strMusicBrainzID":   "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d",
		"strLocked":          "unlocked",
		"strArtistThumb":     "https://r2.theaudiodb.com/images/media/artist/thumb/qpvwuv1347996168.jpg",
		"strArtistLogo":      "https://r2.theaudiodb.com/images/media/artist/logo/sqtvqw1519816358.png",
		"strArtistCutout":    "https://r2.theaudiodb.com/images/media/artist/cutout/le5buy1641552668.png",
		"strArtistClearart":  "https://r2.theaudiodb.com/images/media/artist/clearart/rrywwv1512575176.png",
		"strArtistWideThumb": "https://r2.theaudiodb.com/images/media/artist/widethumb/styrrt1518621883.jpg",
		"strArtistFanart":    "https://r2.theaudiodb.com/images/media/artist/fanart/xrqqqu1541458809.jpg",
		"strArtistFanart2":   "https://r2.theaudiodb.com/images/media/artist/fanart/sssrqr1541458809.jpg",
		"strArtistFanart3":   "https://r2.theaudiodb.com/images/media/artist/fanart/wwvtpp1541458809.jpg",
		"strArtistFanart4":   "https://r2.theaudiodb.com/images/media/artist/fanart/b6zl6c1541458809.jpg",
		"strArtistBanner":    "https://r2.theaudiodb.com/images/media/artist/banner/utwpss1346162520.jpg",
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/api/v1/json/test-key/artist-mb.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"artists": []any{artist}}),
	})

	c := newMockedTestClient(t, rt)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}

	if a.Name != "The Beatles" || a.MusicBrainzID != "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d" {
		t.Errorf("Name/MusicBrainzID = %q/%q, want The Beatles/b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d", a.Name, a.MusicBrainzID)
	}
	if a.Thumb == "" || a.Logo == "" || a.Cutout == "" || a.Clearart == "" || a.WideThumb == "" || a.Fanart == "" || a.Banner == "" {
		t.Errorf("Artist = %+v, want every image field populated", a)
	}
	if a.ISNICode != "" {
		t.Errorf("ISNICode = %q, want empty (JSON null decodes to the zero value)", a.ISNICode)
	}
	if a.Members != "4" || a.Popularity != "85" {
		t.Errorf("Members/Popularity = %q/%q, want 4/85 (quoted JSON strings, not numbers)", a.Members, a.Popularity)
	}
}

// TestLookupArtist_UnknownMBIDReturnsErrNotFoundFromNullField locks in
// TheAudioDB's confirmed-live not-found shape: HTTP 200 with
// {"artists":null}, not a 404.
func TestLookupArtist_UnknownMBIDReturnsErrNotFoundFromNullField(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/api/v1/json/test-key/artist-mb.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"artists": nil}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupArtist(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want wrapping ports.ErrNotFound", err)
	}
}

// TestLookupAlbum_MapsFullFieldSet locks in the real field set confirmed
// live against album-mb.php?i={releaseGroupMBID} for The Beatles' "Please
// Please Me" (release-group MBID de208292-8db5-3aed-a14a-b37a84d8c521)
// during this adapter's implementation.
func TestLookupAlbum_MapsFullFieldSet(t *testing.T) {
	album := map[string]any{
		"idAlbum":                "2117303",
		"idArtist":               "111247",
		"strAlbum":               "Please Please Me",
		"strArtist":              "The Beatles",
		"intYearReleased":        "1963",
		"strStyle":               "Rock/Pop",
		"strGenre":               "Rock & Roll",
		"strLabel":               "EMI",
		"strReleaseFormat":       "Album",
		"strAlbumThumb":          "https://r2.theaudiodb.com/images/media/album/thumb/please-please-me-4ddaafee1d6c3.jpg",
		"strAlbumThumbHQ":        nil,
		"strAlbumBack":           nil,
		"strAlbumCDart":          "https://r2.theaudiodb.com/images/media/album/cdart/please-please-me-4fd2526f52918.png",
		"strAlbumSpine":          nil,
		"strAlbum3DCase":         "https://r2.theaudiodb.com/images/media/album/3dcase/gydwlv1664370373.png",
		"strAlbum3DFlat":         nil,
		"strAlbum3DFace":         nil,
		"strAlbum3DThumb":        "https://r2.theaudiodb.com/images/media/album/3dthumb/kft2t11664370378.png",
		"strDescription":         "Please Please Me is the debut album by English rock band The Beatles.",
		"strMood":                "Happy",
		"strMusicBrainzID":       "de208292-8db5-3aed-a14a-b37a84d8c521",
		"strMusicBrainzArtistID": "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d",
		"strLocked":              "unlocked",
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/api/v1/json/test-key/album-mb.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"album": []any{album}}),
	})

	c := newMockedTestClient(t, rt)
	a, err := c.LookupAlbum(context.Background(), "de208292-8db5-3aed-a14a-b37a84d8c521")
	if err != nil {
		t.Fatalf("LookupAlbum returned error: %v", err)
	}

	if a.Title != "Please Please Me" || a.MusicBrainzID != "de208292-8db5-3aed-a14a-b37a84d8c521" {
		t.Errorf("Title/MusicBrainzID = %q/%q, want Please Please Me/de208292-8db5-3aed-a14a-b37a84d8c521", a.Title, a.MusicBrainzID)
	}
	if a.Thumb == "" || a.CDArt == "" || a.ThreeDCase == "" || a.ThreeDThumb == "" {
		t.Errorf("Album = %+v, want Thumb/CDArt/ThreeDCase/ThreeDThumb all populated", a)
	}
	if a.ThumbHQ != "" || a.Back != "" || a.Spine != "" || a.ThreeDFlat != "" || a.ThreeDFace != "" {
		t.Errorf("Album = %+v, want ThumbHQ/Back/Spine/ThreeDFlat/ThreeDFace all empty (null in the real response)", a)
	}
}

// TestLookupAlbum_UnknownMBIDReturnsErrNotFoundFromNullField mirrors
// TestLookupArtist_UnknownMBIDReturnsErrNotFoundFromNullField for
// album-mb.php's identical {"album":null} shape.
func TestLookupAlbum_UnknownMBIDReturnsErrNotFoundFromNullField(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/api/v1/json/test-key/album-mb.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"album": nil}),
	})

	c := newMockedTestClient(t, rt)
	_, err := c.LookupAlbum(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupAlbum error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/api/v1/json/test-key/artist-mb.php",
		Responder: httpmock.JSON(http.StatusOK, map[string]any{"artists": nil}),
	})

	c := newMockedTestClient(t, rt,
		theaudiodb.WithLogger(slog.Default()),
		theaudiodb.WithTracerProvider(otel.GetTracerProvider()),
		theaudiodb.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupArtist(context.Background(), "x"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want ports.ErrNotFound", err)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/api/v1/json/test-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]any{"artists": []any{map[string]any{"idArtist": "1", "strArtist": "Cached Artist"}}})(req)
		},
	})

	// Rate limiting isn't what this test is about — see
	// musicbrainz_test.go's/theporndb_test.go's identical rationale.
	c := newMockedTestClient(t, rt, theaudiodb.WithRateLimit(rate.Inf))
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
		return httpmock.JSON(http.StatusOK, map[string]any{"artists": nil})(req)
	}

	const n = 3
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: "/api/v1/json/test-key/artist-mb.php", Responder: recorder})
	c := newMockedTestClient(t, rt, theaudiodb.WithRateLimit(3))

	// Each goroutine uses a distinct MBID so the caching transport (keyed
	// on the full request URL, including query) sees n distinct URLs
	// rather than deduplicating identical concurrent requests down to one
	// — this test is about the rate limiter serializing dispatch, not
	// about caching behavior (that's TestClient_CachesGETResponses' job).
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
		Path:   "/api/v1/json/test-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"artists": []any{map[string]any{"idArtist": "1", "strArtist": "x"}}})(req)
		},
	})

	c := newMockedTestClient(t, rt, theaudiodb.WithRetryBaseDelay(time.Millisecond), theaudiodb.WithRateLimit(rate.Inf))
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
		Path:   "/api/v1/json/test-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestClient(t, rt, theaudiodb.WithRetryBaseDelay(time.Millisecond), theaudiodb.WithRateLimit(rate.Inf))
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
		Path:   "/api/v1/json/test-key/artist-mb.php",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.JSON(http.StatusOK, map[string]any{"artists": []any{map[string]any{"idArtist": "1", "strArtist": "x"}}})(req)
		},
	})

	c := newMockedTestClient(t, rt, theaudiodb.WithRetryBaseDelay(time.Millisecond), theaudiodb.WithRateLimit(rate.Inf))
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
