// Package theaudiodbtest is the shared contract test suite for the
// ports.TheAudioDBClient port. Like theporndbtest, this port is
// network-backed, so NewClientFunc takes a baseURL: the suite runs its own
// httptest.Server serving small canned fixtures and hands the adapter
// constructor that server's URL. Richer field-mapping assertions against
// real recorded TheAudioDB responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md) — this suite only proves the
// port's contract: decoding, ErrNotFound mapping, and the
// null-field-means-not-found semantics confirmed live against the real
// API (see internal/ports/theaudiodb.go's own doc comment).
package theaudiodbtest

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
// yields the real API's confirmed "null field" not-found shape.
const (
	KnownArtistMBID         = "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"
	UnknownArtistMBID       = "00000000-0000-0000-0000-000000000001"
	KnownReleaseGroupMBID   = "de208292-8db5-3aed-a14a-b37a84d8c521"
	UnknownReleaseGroupMBID = "00000000-0000-0000-0000-000000000002"
)

// NewClientFunc constructs a fresh ports.TheAudioDBClient pointed at
// baseURL for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.TheAudioDBClient

// TestTheAudioDBClient runs the shared TheAudioDBClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestTheAudioDBClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupArtist returns the artist for a known MBID", func(t *testing.T) { testLookupArtistKnown(t, newClient, server.URL) })
	t.Run("LookupArtist returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupArtistUnknown(t, newClient, server.URL) })
	t.Run("LookupAlbum returns the album for a known MBID", func(t *testing.T) { testLookupAlbumKnown(t, newClient, server.URL) })
	t.Run("LookupAlbum returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupAlbumUnknown(t, newClient, server.URL) })
}

func testLookupArtistKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	a, err := c.LookupArtist(context.Background(), KnownArtistMBID)
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.MusicBrainzID != KnownArtistMBID || a.Name != "Known Artist" {
		t.Fatalf("LookupArtist = %+v, want MusicBrainzID=%s Name=Known Artist", a, KnownArtistMBID)
	}
}

func testLookupArtistUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupArtist(context.Background(), UnknownArtistMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testLookupAlbumKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	a, err := c.LookupAlbum(context.Background(), KnownReleaseGroupMBID)
	if err != nil {
		t.Fatalf("LookupAlbum returned error: %v", err)
	}
	if a.MusicBrainzID != KnownReleaseGroupMBID || a.Title != "Known Album" {
		t.Fatalf("LookupAlbum = %+v, want MusicBrainzID=%s Title=Known Album", a, KnownReleaseGroupMBID)
	}
}

func testLookupAlbumUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupAlbum(context.Background(), UnknownReleaseGroupMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupAlbum error = %v, want wrapping ports.ErrNotFound", err)
	}
}

// fixtureHandler is a minimal, in-process TheAudioDB REST server: it
// dispatches on path + the "i" query param, answering with canned data for
// the Known* constants above — the real API's confirmed
// {"artists":null}/{"album":null} shape (HTTP 200, not a 404) for anything
// else. The {key} path segment is a wildcard, deliberately unchecked —
// this suite only proves the port's contract through NewClientFunc's
// black-box ports.TheAudioDBClient, and has no visibility into whatever
// APIKey the concrete adapter constructor it's given chose to configure.
func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/artist-mb.php", handleLookupArtist)
	mux.HandleFunc("GET /{key}/album-mb.php", handleLookupAlbum)
	return mux
}

func handleLookupArtist(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("i") != KnownArtistMBID {
		writeJSON(w, map[string]any{"artists": nil})
		return
	}
	writeJSON(w, map[string]any{"artists": []ports.TADBArtist{{
		Name:          "Known Artist",
		MusicBrainzID: KnownArtistMBID,
	}}})
}

func handleLookupAlbum(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("i") != KnownReleaseGroupMBID {
		writeJSON(w, map[string]any{"album": nil})
		return
	}
	writeJSON(w, map[string]any{"album": []ports.TADBAlbum{{
		Title:         "Known Album",
		MusicBrainzID: KnownReleaseGroupMBID,
	}}})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
