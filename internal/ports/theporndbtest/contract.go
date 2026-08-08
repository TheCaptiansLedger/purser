// Package theporndbtest is the shared contract test suite for the
// ports.ThePornDBClient port. Like stashdbtest, this port is
// network-backed, so NewClientFunc takes a baseURL: the suite runs its own
// httptest.Server serving small canned fixtures and hands the adapter
// constructor that server's URL. Richer field-mapping assertions against
// real recorded ThePornDB responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md) — this suite only proves the
// port's contract: decoding, ErrNotFound mapping, and "empty is not an
// error" search/resolve semantics.
//
// Unlike StashDB's single GraphQL endpoint, ThePornDB is REST — one path
// per resource, dispatched below the same way musicbrainztest's fixture
// server would (path + query string), confirmed against the real API's
// live-verified endpoint shapes (see internal/adapters/theporndb's package
// doc comment).
package theporndbtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known/unknown IDs and a known hash/code the fixture server recognizes;
// any other value on the same operation yields a 404, exactly like the
// real API's confirmed "performer not found"/"scene not found"/"hash not
// found" responses.
const (
	KnownPerformerID   = "11111111-1111-1111-1111-111111111111"
	UnknownPerformerID = "00000000-0000-0000-0000-000000000001"
	KnownSceneID       = "22222222-2222-2222-2222-222222222222"
	UnknownSceneID     = "00000000-0000-0000-0000-000000000002"
	KnownHash          = "aaaaaaaaaaaaaaaa"
	UnknownHash        = "bbbbbbbbbbbbbbbb"
	KnownJAVCode       = "SSIS-001"
)

// NewClientFunc constructs a fresh ports.ThePornDBClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.ThePornDBClient

// TestThePornDBClient runs the shared ThePornDBClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestThePornDBClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupPerformer returns the performer for a known ID", func(t *testing.T) { testLookupPerformerKnown(t, newClient, server.URL) })
	t.Run("LookupPerformer returns ErrNotFound for an unknown ID", func(t *testing.T) { testLookupPerformerUnknown(t, newClient, server.URL) })
	t.Run("SearchPerformers returns matches for a query", func(t *testing.T) { testSearchPerformersMatch(t, newClient, server.URL) })
	t.Run("SearchPerformers returns an empty, non-error result for no matches", func(t *testing.T) { testSearchPerformersNoMatch(t, newClient, server.URL) })
	t.Run("LookupScene returns the scene for a known ID", func(t *testing.T) { testLookupSceneKnown(t, newClient, server.URL) })
	t.Run("LookupScene returns ErrNotFound for an unknown ID", func(t *testing.T) { testLookupSceneUnknown(t, newClient, server.URL) })
	t.Run("SearchScenes returns matches for a query", func(t *testing.T) { testSearchScenesMatch(t, newClient, server.URL) })
	t.Run("SearchScenes returns an empty, non-error result for no matches", func(t *testing.T) { testSearchScenesNoMatch(t, newClient, server.URL) })
	t.Run("LookupSceneByHash returns the scene for a known hash", func(t *testing.T) { testLookupSceneByHashKnown(t, newClient, server.URL) })
	t.Run("LookupSceneByHash returns ErrNotFound for an unknown hash", func(t *testing.T) { testLookupSceneByHashUnknown(t, newClient, server.URL) })
	t.Run("ResolveJAVCode returns candidates for a known code", func(t *testing.T) { testResolveJAVCodeMatch(t, newClient, server.URL) })
	t.Run("ResolveJAVCode returns an empty, non-error result for no candidates", func(t *testing.T) { testResolveJAVCodeNoMatch(t, newClient, server.URL) })
}

func testLookupPerformerKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	p, err := c.LookupPerformer(context.Background(), KnownPerformerID)
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}
	if p.ID != KnownPerformerID || p.Name != "Known Performer" {
		t.Fatalf("LookupPerformer = %+v, want ID=%s Name=Known Performer", p, KnownPerformerID)
	}
}

func testLookupPerformerUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupPerformer(context.Background(), UnknownPerformerID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupPerformer error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testSearchPerformersMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchPerformers(context.Background(), "Known")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(results) != 1 || results[0].ID != KnownPerformerID {
		t.Fatalf("SearchPerformers = %+v, want one result with ID=%s", results, KnownPerformerID)
	}
}

func testSearchPerformersNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchPerformers(context.Background(), "no-such-performer")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchPerformers = %+v, want empty", results)
	}
}

func testLookupSceneKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	s, err := c.LookupScene(context.Background(), KnownSceneID)
	if err != nil {
		t.Fatalf("LookupScene returned error: %v", err)
	}
	if s.ID != KnownSceneID || s.Title != "Known Scene" {
		t.Fatalf("LookupScene = %+v, want ID=%s Title=Known Scene", s, KnownSceneID)
	}
}

func testLookupSceneUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupScene(context.Background(), UnknownSceneID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupScene error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testSearchScenesMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchScenes(context.Background(), "Known")
	if err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}
	if len(results) != 1 || results[0].ID != KnownSceneID {
		t.Fatalf("SearchScenes = %+v, want one result with ID=%s", results, KnownSceneID)
	}
}

func testSearchScenesNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.SearchScenes(context.Background(), "no-such-scene")
	if err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchScenes = %+v, want empty", results)
	}
}

func testLookupSceneByHashKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	s, err := c.LookupSceneByHash(context.Background(), KnownHash)
	if err != nil {
		t.Fatalf("LookupSceneByHash returned error: %v", err)
	}
	if s.ID != KnownSceneID {
		t.Fatalf("LookupSceneByHash = %+v, want ID=%s", s, KnownSceneID)
	}
}

func testLookupSceneByHashUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupSceneByHash(context.Background(), UnknownHash)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupSceneByHash error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testResolveJAVCodeMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.ResolveJAVCode(context.Background(), KnownJAVCode)
	if err != nil {
		t.Fatalf("ResolveJAVCode returned error: %v", err)
	}
	if len(results) != 1 || results[0].ID != KnownSceneID || results[0].Type != "JAV" {
		t.Fatalf("ResolveJAVCode = %+v, want one JAV result with ID=%s", results, KnownSceneID)
	}
}

func testResolveJAVCodeNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.ResolveJAVCode(context.Background(), "no-such-code")
	if err != nil {
		t.Fatalf("ResolveJAVCode returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("ResolveJAVCode = %+v, want empty", results)
	}
}

// fixtureHandler is a minimal, in-process ThePornDB REST server: it
// dispatches on method+path (and the "q"/"hash"/"parse" query params for
// search-shaped endpoints), answering with canned data for the Known*
// constants above — a 404 (matching the real API's
// {"message": "... not found"} body) for anything else.
func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /performers/{id}", handleLookupPerformer)
	mux.HandleFunc("GET /performers", handleSearchPerformers)
	mux.HandleFunc("GET /scenes/hash/{hash}", handleLookupSceneByHash)
	mux.HandleFunc("GET /scenes/{id}", handleLookupScene)
	mux.HandleFunc("GET /scenes", handleSearchScenes)
	mux.HandleFunc("GET /jav", handleResolveJAVCode)
	return mux
}

func handleLookupPerformer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != KnownPerformerID {
		writeNotFound(w, "performer not found")
		return
	}
	writeData(w, ports.TPDBPerformer{ID: KnownPerformerID, Name: "Known Performer"})
}

func handleSearchPerformers(w http.ResponseWriter, r *http.Request) {
	performers := []ports.TPDBPerformer{}
	if r.URL.Query().Get("q") == "Known" {
		performers = append(performers, ports.TPDBPerformer{ID: KnownPerformerID, Name: "Known Performer"})
	}
	writeDataList(w, performers)
}

func handleLookupScene(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != KnownSceneID {
		writeNotFound(w, "scene not found")
		return
	}
	writeData(w, ports.TPDBScene{ID: KnownSceneID, Title: "Known Scene"})
}

func handleSearchScenes(w http.ResponseWriter, r *http.Request) {
	scenes := []ports.TPDBScene{}
	if r.URL.Query().Get("q") == "Known" {
		scenes = append(scenes, ports.TPDBScene{ID: KnownSceneID, Title: "Known Scene"})
	}
	writeDataList(w, scenes)
}

func handleLookupSceneByHash(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if hash != KnownHash {
		writeNotFound(w, "hash not found")
		return
	}
	writeData(w, ports.TPDBScene{ID: KnownSceneID, Title: "Known Scene"})
}

func handleResolveJAVCode(w http.ResponseWriter, r *http.Request) {
	scenes := []ports.TPDBScene{}
	if r.URL.Query().Get("parse") == KnownJAVCode {
		scenes = append(scenes, ports.TPDBScene{ID: KnownSceneID, Title: "Known JAV Title", Type: "JAV"})
	}
	writeDataList(w, scenes)
}

func writeNotFound(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func writeData(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": value})
}

// writeDataList wraps value the same way writeData does, plus the empty
// links/meta envelope the real API's list-shaped endpoints carry — not
// consumed by this adapter, included only so the fixture's shape matches
// the real response closely.
func writeDataList(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  value,
		"links": map[string]any{},
		"meta":  map[string]any{},
	})
}
