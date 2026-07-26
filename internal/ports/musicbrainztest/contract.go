// Package musicbrainztest is the shared contract test suite for the
// ports.MusicBrainzClient port. Unlike internal/ports/filewalkertest or
// internal/ports/imagestoretest — both self-contained (tmp dir, memory) —
// this port is network-backed, so NewClientFunc takes a baseURL: the
// suite runs its own httptest.Server serving small canned fixtures and
// hands the adapter constructor that server's URL. Richer field-mapping
// assertions against real recorded MusicBrainz responses belong in the
// adapter's own package (docs/adr/0003-go-testing-standards.md) — this
// suite only proves the port's contract: decoding, ErrNotFound mapping,
// and "empty is not an error" search/list semantics.
package musicbrainztest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known MBIDs/identifiers the fixture server recognizes; any other value
// on the same route yields a 404, exactly like the real MusicBrainz API.
const (
	KnownArtistMBID         = "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"
	UnknownArtistMBID       = "00000000-0000-0000-0000-000000000001"
	KnownReleaseGroupMBID   = "de208292-8db5-3aed-a14a-b37a84d8c521"
	UnknownReleaseGroupMBID = "00000000-0000-0000-0000-000000000002"
	KnownReleaseMBID        = "ade577f6-6087-4a4f-8e87-38b0f8169814"
	UnknownReleaseMBID      = "00000000-0000-0000-0000-000000000003"
	KnownBarcode            = "094638241621"
	KnownISRC               = "GBAYE0600350"
	UnknownISRC             = "XXXX0000000"
)

// NewClientFunc constructs a fresh ports.MusicBrainzClient pointed at
// baseURL for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.MusicBrainzClient

// TestMusicBrainzClient runs the shared MusicBrainzClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestMusicBrainzClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupArtist returns the artist for a known MBID", func(t *testing.T) { testLookupArtistKnown(t, newClient, server.URL) })
	t.Run("LookupArtist returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupArtistUnknown(t, newClient, server.URL) })
	t.Run("SearchArtists returns matches for a query", func(t *testing.T) { testSearchArtistsMatch(t, newClient, server.URL) })
	t.Run("SearchArtists returns an empty, non-error result for no matches", func(t *testing.T) { testSearchArtistsNoMatch(t, newClient, server.URL) })
	t.Run("LookupReleaseGroup returns the release group for a known MBID", func(t *testing.T) { testLookupReleaseGroupKnown(t, newClient, server.URL) })
	t.Run("LookupReleaseGroup returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupReleaseGroupUnknown(t, newClient, server.URL) })
	t.Run("ListReleaseGroupsForArtist returns the artist's release groups", func(t *testing.T) { testListReleaseGroupsForArtist(t, newClient, server.URL) })
	t.Run("SearchReleaseGroups returns matches for artist/album names", func(t *testing.T) { testSearchReleaseGroupsMatch(t, newClient, server.URL) })
	t.Run("SearchReleaseGroups returns an empty, non-error result for no matches", func(t *testing.T) { testSearchReleaseGroupsNoMatch(t, newClient, server.URL) })
	t.Run("LookupRelease returns the release for a known MBID", func(t *testing.T) { testLookupReleaseKnown(t, newClient, server.URL) })
	t.Run("LookupRelease returns ErrNotFound for an unknown MBID", func(t *testing.T) { testLookupReleaseUnknown(t, newClient, server.URL) })
	t.Run("ListReleasesForReleaseGroup returns the release group's releases", func(t *testing.T) { testListReleasesForReleaseGroup(t, newClient, server.URL) })
	t.Run("SearchReleaseByBarcode returns the release carrying the barcode", func(t *testing.T) { testSearchReleaseByBarcodeMatch(t, newClient, server.URL) })
	t.Run("SearchReleaseByBarcode returns an empty, non-error result for an unknown barcode", func(t *testing.T) { testSearchReleaseByBarcodeNoMatch(t, newClient, server.URL) })
	t.Run("LookupRecordingByISRC returns the recording(s) sharing the ISRC", func(t *testing.T) { testLookupRecordingByISRCKnown(t, newClient, server.URL) })
	t.Run("LookupRecordingByISRC returns ErrNotFound for an unknown ISRC", func(t *testing.T) { testLookupRecordingByISRCUnknown(t, newClient, server.URL) })
}

func testLookupArtistKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	a, err := c.LookupArtist(context.Background(), KnownArtistMBID)
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.ID != KnownArtistMBID || a.Name != "The Beatles" {
		t.Fatalf("LookupArtist = %+v, want ID=%s Name=The Beatles", a, KnownArtistMBID)
	}
}

func testLookupArtistUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupArtist(context.Background(), UnknownArtistMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupArtist error = %v, want wrapping ErrNotFound", err)
	}
}

func testSearchArtistsMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchArtists(context.Background(), "Beatles")
	if err != nil {
		t.Fatalf("SearchArtists returned error: %v", err)
	}
	if len(results) != 1 || results[0].ID != KnownArtistMBID {
		t.Fatalf("SearchArtists = %+v, want one result with ID=%s", results, KnownArtistMBID)
	}
}

func testSearchArtistsNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchArtists(context.Background(), "no-such-artist")
	if err != nil {
		t.Fatalf("SearchArtists returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchArtists = %+v, want empty", results)
	}
}

func testLookupReleaseGroupKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rg, err := c.LookupReleaseGroup(context.Background(), KnownReleaseGroupMBID)
	if err != nil {
		t.Fatalf("LookupReleaseGroup returned error: %v", err)
	}
	if rg.ID != KnownReleaseGroupMBID || rg.Title != "Please Please Me" {
		t.Fatalf("LookupReleaseGroup = %+v, want ID=%s Title=Please Please Me", rg, KnownReleaseGroupMBID)
	}
}

func testLookupReleaseGroupUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupReleaseGroup(context.Background(), UnknownReleaseGroupMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupReleaseGroup error = %v, want wrapping ErrNotFound", err)
	}
}

func testListReleaseGroupsForArtist(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rgs, err := c.ListReleaseGroupsForArtist(context.Background(), KnownArtistMBID)
	if err != nil {
		t.Fatalf("ListReleaseGroupsForArtist returned error: %v", err)
	}
	if len(rgs) != 1 || rgs[0].ID != KnownReleaseGroupMBID {
		t.Fatalf("ListReleaseGroupsForArtist = %+v, want one result with ID=%s", rgs, KnownReleaseGroupMBID)
	}
}

func testSearchReleaseGroupsMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rgs, err := c.SearchReleaseGroups(context.Background(), "The Beatles", "Please Please Me")
	if err != nil {
		t.Fatalf("SearchReleaseGroups returned error: %v", err)
	}
	if len(rgs) != 1 || rgs[0].ID != KnownReleaseGroupMBID {
		t.Fatalf("SearchReleaseGroups = %+v, want one result with ID=%s", rgs, KnownReleaseGroupMBID)
	}
}

func testSearchReleaseGroupsNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rgs, err := c.SearchReleaseGroups(context.Background(), "Nobody", "Nothing")
	if err != nil {
		t.Fatalf("SearchReleaseGroups returned error: %v", err)
	}
	if len(rgs) != 0 {
		t.Fatalf("SearchReleaseGroups = %+v, want empty", rgs)
	}
}

func testLookupReleaseKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	r, err := c.LookupRelease(context.Background(), KnownReleaseMBID)
	if err != nil {
		t.Fatalf("LookupRelease returned error: %v", err)
	}
	if r.ID != KnownReleaseMBID || len(r.Media) != 1 {
		t.Fatalf("LookupRelease = %+v, want ID=%s and one medium", r, KnownReleaseMBID)
	}
}

func testLookupReleaseUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupRelease(context.Background(), UnknownReleaseMBID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupRelease error = %v, want wrapping ErrNotFound", err)
	}
}

func testListReleasesForReleaseGroup(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rs, err := c.ListReleasesForReleaseGroup(context.Background(), KnownReleaseGroupMBID)
	if err != nil {
		t.Fatalf("ListReleasesForReleaseGroup returned error: %v", err)
	}
	if len(rs) != 1 || rs[0].ID != KnownReleaseMBID {
		t.Fatalf("ListReleasesForReleaseGroup = %+v, want one result with ID=%s", rs, KnownReleaseMBID)
	}
}

func testSearchReleaseByBarcodeMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rs, err := c.SearchReleaseByBarcode(context.Background(), KnownBarcode)
	if err != nil {
		t.Fatalf("SearchReleaseByBarcode returned error: %v", err)
	}
	if len(rs) != 1 || rs[0].Barcode != KnownBarcode {
		t.Fatalf("SearchReleaseByBarcode = %+v, want one result with Barcode=%s", rs, KnownBarcode)
	}
}

func testSearchReleaseByBarcodeNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	rs, err := c.SearchReleaseByBarcode(context.Background(), "0000000000000")
	if err != nil {
		t.Fatalf("SearchReleaseByBarcode returned error: %v", err)
	}
	if len(rs) != 0 {
		t.Fatalf("SearchReleaseByBarcode = %+v, want empty", rs)
	}
}

func testLookupRecordingByISRCKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	recs, err := c.LookupRecordingByISRC(context.Background(), KnownISRC)
	if err != nil {
		t.Fatalf("LookupRecordingByISRC returned error: %v", err)
	}
	if len(recs) != 1 || recs[0].Title != "Please Please Me" {
		t.Fatalf("LookupRecordingByISRC = %+v, want one recording titled Please Please Me", recs)
	}
}

func testLookupRecordingByISRCUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupRecordingByISRC(context.Background(), UnknownISRC)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupRecordingByISRC error = %v, want wrapping ErrNotFound", err)
	}
}

func fixtureHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/artist/", func(w http.ResponseWriter, r *http.Request) {
		mbid := r.URL.Path[len("/artist/"):]
		if mbid != KnownArtistMBID {
			writeNotFound(w)
			return
		}
		writeJSON(w, ports.Artist{ID: KnownArtistMBID, Name: "The Beatles", SortName: "Beatles, The"})
	})

	mux.HandleFunc("/artist", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		artists := []ports.Artist{}
		if q == "Beatles" {
			artists = append(artists, ports.Artist{ID: KnownArtistMBID, Name: "The Beatles"})
		}
		writeJSON(w, struct {
			Artists []ports.Artist `json:"artists"`
		}{artists})
	})

	mux.HandleFunc("/release-group/", func(w http.ResponseWriter, r *http.Request) {
		mbid := r.URL.Path[len("/release-group/"):]
		if mbid != KnownReleaseGroupMBID {
			writeNotFound(w)
			return
		}
		writeJSON(w, ports.ReleaseGroup{ID: KnownReleaseGroupMBID, Title: "Please Please Me"})
	})

	mux.HandleFunc("/release-group", func(w http.ResponseWriter, r *http.Request) {
		rgs := []ports.ReleaseGroup{}
		switch {
		case r.URL.Query().Get("artist") == KnownArtistMBID:
			rgs = append(rgs, ports.ReleaseGroup{ID: KnownReleaseGroupMBID, Title: "Please Please Me"})
		case r.URL.Query().Get("query") == `artist:"The Beatles" AND releasegroup:"Please Please Me"`:
			rgs = append(rgs, ports.ReleaseGroup{ID: KnownReleaseGroupMBID, Title: "Please Please Me"})
		}
		writeJSON(w, struct {
			ReleaseGroups []ports.ReleaseGroup `json:"release-groups"`
		}{rgs})
	})

	mux.HandleFunc("/release/", func(w http.ResponseWriter, r *http.Request) {
		mbid := r.URL.Path[len("/release/"):]
		if mbid != KnownReleaseMBID {
			writeNotFound(w)
			return
		}
		writeJSON(w, ports.Release{
			ID:    KnownReleaseMBID,
			Title: "Please Please Me",
			Media: []ports.Medium{{Format: "12\" Vinyl", TrackCount: 14}},
		})
	})

	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		releases := []ports.Release{}
		switch {
		case r.URL.Query().Get("release-group") == KnownReleaseGroupMBID:
			releases = append(releases, ports.Release{ID: KnownReleaseMBID, Title: "Please Please Me"})
		case r.URL.Query().Get("query") == "barcode:"+KnownBarcode:
			releases = append(releases, ports.Release{ID: "d59f458f-4492-4f1f-bd2c-808156c2710f", Barcode: KnownBarcode})
		}
		writeJSON(w, struct {
			Releases []ports.Release `json:"releases"`
		}{releases})
	})

	mux.HandleFunc("/isrc/", func(w http.ResponseWriter, r *http.Request) {
		isrc := r.URL.Path[len("/isrc/"):]
		if isrc != KnownISRC {
			writeNotFound(w)
			return
		}
		writeJSON(w, struct {
			Recordings []ports.Recording `json:"recordings"`
		}{[]ports.Recording{{ID: "b720b11f-2c00-4f48-83b9-4aa9d3207e8c", Title: "Please Please Me"}}})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"Not Found"}`))
}
