package musicbrainz_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"purser/internal/adapters/musicbrainz"
	"purser/internal/ports"
	"purser/internal/ports/musicbrainztest"
	"purser/internal/version"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func newTestClient(t *testing.T, baseURL string) *musicbrainz.Client {
	t.Helper()
	cfg := musicbrainz.DefaultConfig()
	cfg.BaseURL = baseURL
	c, err := musicbrainz.New(cfg)
	if err != nil {
		t.Fatalf("musicbrainz.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestClient builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport. BaseURL is
// left at its real default; RoundTrip intercepts before any DNS/dial ever
// happens, so there's no need for a placeholder URL.
func newMockedTestClient(t *testing.T, rt http.RoundTripper, opts ...musicbrainz.Option) *musicbrainz.Client {
	t.Helper()
	allOpts := append([]musicbrainz.Option{musicbrainz.WithBaseTransport(rt)}, opts...)
	c, err := musicbrainz.New(musicbrainz.DefaultConfig(), allOpts...)
	if err != nil {
		t.Fatalf("musicbrainz.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_MusicBrainzClientContract(t *testing.T) {
	musicbrainztest.TestMusicBrainzClient(t, func(t *testing.T, baseURL string) ports.MusicBrainzClient {
		return newTestClient(t, baseURL)
	})
}

// rawFixture reads a recorded JSON fixture at testdata/name.json — a real,
// previously recorded MusicBrainz response, per
// docs/adr/0003-go-testing-standards.md's "recorded request/response
// fixtures" requirement for adapter-specific tests beyond the shared
// contract test.
func rawFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func TestLookupArtist_MapsRecordedBeatlesFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/artist/b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "artist_lookup")),
	})

	c := newMockedTestClient(t, rt)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}

	if a.Name != "The Beatles" {
		t.Errorf("Name = %q, want %q", a.Name, "The Beatles")
	}
	if a.SortName != "Beatles, The" {
		t.Errorf("SortName = %q, want %q", a.SortName, "Beatles, The")
	}
	if a.Type != "Group" {
		t.Errorf("Type = %q, want %q", a.Type, "Group")
	}
	if a.Country != "GB" {
		t.Errorf("Country = %q, want %q", a.Country, "GB")
	}
	if a.LifeSpan.Begin != "1960-03-27" || a.LifeSpan.End != "1970-04-10" || !a.LifeSpan.Ended {
		t.Errorf("LifeSpan = %+v, want begin=1960-03-27 end=1970-04-10 ended=true", a.LifeSpan)
	}
	if len(a.ISNIs) != 1 || a.ISNIs[0] != "0000000121707484" {
		t.Errorf("ISNIs = %v, want [0000000121707484]", a.ISNIs)
	}
	if len(a.Aliases) == 0 {
		t.Error("Aliases is empty, want at least one recorded alias")
	}
	if len(a.Relations) == 0 {
		t.Error("Relations is empty, want at least one recorded relation")
	}
}

func TestLookupRelease_MapsRecordedReleaseFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/release/ade577f6-6087-4a4f-8e87-38b0f8169814",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "release_lookup")),
	})

	c := newMockedTestClient(t, rt)
	r, err := c.LookupRelease(context.Background(), "ade577f6-6087-4a4f-8e87-38b0f8169814")
	if err != nil {
		t.Fatalf("LookupRelease returned error: %v", err)
	}

	if r.Title != "Please Please Me" {
		t.Errorf("Title = %q, want %q", r.Title, "Please Please Me")
	}
	if r.ReleaseGroup == nil || r.ReleaseGroup.ID != "de208292-8db5-3aed-a14a-b37a84d8c521" {
		t.Errorf("ReleaseGroup = %+v, want ID=de208292-8db5-3aed-a14a-b37a84d8c521", r.ReleaseGroup)
	}
	if len(r.Media) != 1 {
		t.Fatalf("Media has %d entries, want 1", len(r.Media))
	}
	if len(r.Media[0].Tracks) == 0 {
		t.Fatal("Media[0].Tracks is empty, want at least one recorded track")
	}
	first := r.Media[0].Tracks[0]
	if first.Recording == nil || first.Recording.Title == "" {
		t.Errorf("first track's Recording = %+v, want a populated recording", first.Recording)
	}
}

func TestLookupReleaseGroup_MapsRecordedFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/release-group/de208292-8db5-3aed-a14a-b37a84d8c521",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "releasegroup_lookup")),
	})

	c := newMockedTestClient(t, rt)
	rg, err := c.LookupReleaseGroup(context.Background(), "de208292-8db5-3aed-a14a-b37a84d8c521")
	if err != nil {
		t.Fatalf("LookupReleaseGroup returned error: %v", err)
	}
	if rg.Title != "Please Please Me" || rg.PrimaryType != "Album" || rg.FirstReleaseDate != "1963-03-22" {
		t.Errorf("ReleaseGroup = %+v, want Title=Please Please Me PrimaryType=Album FirstReleaseDate=1963-03-22", rg)
	}
}

func TestListReleasesForReleaseGroup_MapsRecordedFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/release",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "releases_for_releasegroup")),
	})

	c := newMockedTestClient(t, rt)
	releases, err := c.ListReleasesForReleaseGroup(context.Background(), "de208292-8db5-3aed-a14a-b37a84d8c521")
	if err != nil {
		t.Fatalf("ListReleasesForReleaseGroup returned error: %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("ListReleasesForReleaseGroup returned no releases")
	}
	for _, r := range releases {
		if r.ID == "" {
			t.Errorf("release missing ID: %+v", r)
		}
	}
}

func TestSearchReleaseByBarcode_MapsRecordedFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/release",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "barcode_search")),
	})

	c := newMockedTestClient(t, rt)
	releases, err := c.SearchReleaseByBarcode(context.Background(), "094638241621")
	if err != nil {
		t.Fatalf("SearchReleaseByBarcode returned error: %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("SearchReleaseByBarcode returned no releases")
	}
	if releases[0].Barcode != "094638241621" {
		t.Errorf("first release Barcode = %q, want 094638241621", releases[0].Barcode)
	}
	if releases[0].ReleaseGroup == nil || releases[0].ReleaseGroup.ID != "de208292-8db5-3aed-a14a-b37a84d8c521" {
		t.Errorf("first release's ReleaseGroup = %+v, want ID=de208292-8db5-3aed-a14a-b37a84d8c521", releases[0].ReleaseGroup)
	}
}

func TestLookupRecordingByISRC_MapsRecordedFixture(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/isrc/GBAYE0600350",
		Responder: httpmock.Raw(http.StatusOK, rawFixture(t, "isrc_lookup")),
	})

	c := newMockedTestClient(t, rt)
	recs, err := c.LookupRecordingByISRC(context.Background(), "GBAYE0600350")
	if err != nil {
		t.Fatalf("LookupRecordingByISRC returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("LookupRecordingByISRC returned %d recordings, want 1", len(recs))
	}
	if recs[0].Title != "Please Please Me" {
		t.Errorf("Title = %q, want %q", recs[0].Title, "Please Please Me")
	}
	if len(recs[0].ArtistCredit) != 1 || recs[0].ArtistCredit[0].Artist.Name != "The Beatles" {
		t.Errorf("ArtistCredit = %+v, want one credit to The Beatles", recs[0].ArtistCredit)
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/ws/2/release-group/x",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.JSON(http.StatusOK, map[string]string{"id": "x", "title": "y"})(req)
		},
	})

	cfg := musicbrainz.DefaultConfig()
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := musicbrainz.New(cfg, musicbrainz.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("musicbrainz.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.LookupReleaseGroup(context.Background(), "x"); err != nil {
		t.Fatalf("LookupReleaseGroup returned error: %v", err)
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/ws/2/release-group/cache-test",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.JSON(http.StatusOK, map[string]string{"id": "cache-test", "title": "Cached Release Group"})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	ctx := context.Background()

	if _, err := c.LookupReleaseGroup(ctx, "cache-test"); err != nil {
		t.Fatalf("first LookupReleaseGroup returned error: %v", err)
	}
	if _, err := c.LookupReleaseGroup(ctx, "cache-test"); err != nil {
		t.Fatalf("second LookupReleaseGroup returned error: %v", err)
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
		return httpmock.JSON(http.StatusOK, map[string]string{"id": "x", "title": "y"})(req)
	}

	const n = 3
	routes := make([]httpmock.Route, n)
	for i := range n {
		// Distinct MBIDs so the caching transport doesn't short-circuit
		// requests 2 and 3 against request 1's cached response.
		routes[i] = httpmock.Route{
			Method:    http.MethodGet,
			Path:      fmt.Sprintf("/ws/2/release-group/rate-limit-test-%d", i),
			Responder: recorder,
		}
	}
	rt := httpmock.New(routes...)
	c := newMockedTestClient(t, rt)

	var wg sync.WaitGroup
	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mbid := fmt.Sprintf("rate-limit-test-%d", i)
			if _, err := c.LookupReleaseGroup(context.Background(), mbid); err != nil {
				t.Errorf("LookupReleaseGroup returned error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// 1 req/sec with burst 1 means n requests take at least (n-1) seconds,
	// serialized. Allow slack for scheduling jitter.
	minExpected := time.Duration(n-1) * 900 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("elapsed = %v, want at least %v (rate limiter should serialize concurrent requests to 1/sec)", elapsed, minExpected)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(arrivals) != n {
		t.Fatalf("server received %d requests, want %d", len(arrivals), n)
	}
}

func TestNew_WithOptions(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/ws/2/release-group/x",
		Responder: httpmock.JSON(http.StatusOK, map[string]string{"id": "x", "title": "y"}),
	})

	c := newMockedTestClient(t, rt,
		musicbrainz.WithLogger(slog.Default()),
		musicbrainz.WithTracerProvider(otel.GetTracerProvider()),
		musicbrainz.WithMeterProvider(otel.GetMeterProvider()),
	)

	if _, err := c.LookupReleaseGroup(context.Background(), "x"); err != nil {
		t.Fatalf("LookupReleaseGroup returned error: %v", err)
	}
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := musicbrainz.DefaultConfig()
	cfg.BaseURL = ""
	if _, err := musicbrainz.New(cfg); err == nil {
		t.Fatal("musicbrainz.New with empty BaseURL returned nil error")
	}
}

func TestSearchReleaseGroups_BuildsLuceneQueryFromArtistAndAlbumNames(t *testing.T) {
	var gotQuery string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/ws/2/release-group",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotQuery = req.URL.Query().Get("query")
			return httpmock.JSON(http.StatusOK, struct {
				ReleaseGroups []ports.ReleaseGroup `json:"release-groups"`
			}{})(req)
		},
	})

	c := newMockedTestClient(t, rt)
	if _, err := c.SearchReleaseGroups(context.Background(), "The Beatles", "Please Please Me"); err != nil {
		t.Fatalf("SearchReleaseGroups returned error: %v", err)
	}

	want := `artist:"The Beatles" AND releasegroup:"Please Please Me"`
	if gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}
}
