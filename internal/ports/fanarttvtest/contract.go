// Package fanarttvtest is the shared contract test suite for the
// ports.FanartTVClient port. Like theaudiodbtest, this port is
// network-backed, so NewClientFunc takes a baseURL: the suite runs its own
// httptest.Server serving small canned fixtures and hands the adapter
// constructor that server's URL. Richer field-mapping assertions against
// real recorded fanart.tv responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md) — this suite only proves the
// port's contract: decoding and the empty-body-means-not-found semantics
// confirmed live against the real API (see internal/ports/fanarttv.go's
// own doc comment).
package fanarttvtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known/unknown MBIDs the fixture server recognizes; any other value
// yields the real API's confirmed empty-{}-body not-found shape.
const (
	KnownArtistMBID   = "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"
	UnknownArtistMBID = "00000000-0000-0000-0000-000000000001"
)

// NewClientFunc constructs a fresh ports.FanartTVClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.FanartTVClient

// TestFanartTVClient runs the shared FanartTVClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestFanartTVClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupArtist returns the artist for a known MBID", func(t *testing.T) { testLookupArtistKnown(t, newClient, server.URL) })
	t.Run("LookupArtist returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupArtistUnknown(t, newClient, server.URL) })
}

func testLookupArtistKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	a, err := c.LookupArtist(context.Background(), KnownArtistMBID)
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.MBID != KnownArtistMBID || a.Name != "Known Artist" {
		t.Fatalf("LookupArtist = %+v, want MBID=%s Name=Known Artist", a, KnownArtistMBID)
	}
}

func testLookupArtistUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupArtist(context.Background(), UnknownArtistMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want wrapping ports.ErrNotFound", err)
	}
}

// fixtureHandler is a minimal, in-process fanart.tv REST server: it
// dispatches on path, answering with canned data for KnownArtistMBID — the
// real API's confirmed empty-{}-body shape (HTTP 200, not a 404) for
// anything else.
func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /music/{mbid}", handleLookupArtist)
	return mux
}

func handleLookupArtist(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("mbid") != KnownArtistMBID {
		writeJSON(w, map[string]any{})
		return
	}
	writeJSON(w, ports.FanartArtist{
		Name: "Known Artist",
		MBID: KnownArtistMBID,
		Albums: map[string]ports.FanartAlbumImages{
			"de208292-8db5-3aed-a14a-b37a84d8c521": {
				AlbumCover: []ports.FanartImage{{ID: "1", URL: "https://example.invalid/cover.jpg", Likes: "3"}},
			},
		},
	})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
