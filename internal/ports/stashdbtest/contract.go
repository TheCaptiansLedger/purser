// Package stashdbtest is the shared contract test suite for the
// ports.StashDBClient port. Like musicbrainztest, this port is
// network-backed, so NewClientFunc takes a baseURL: the suite runs its own
// httptest.Server serving small canned fixtures and hands the adapter
// constructor that server's URL. Richer field-mapping assertions against
// real recorded StashDB responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md) — this suite only proves the
// port's contract: decoding, ErrNotFound mapping, and "empty is not an
// error" search semantics.
//
// Unlike MusicBrainz's REST API (one path per resource), StashDB is
// GraphQL: every request hits the same endpoint with a different query
// document. The fixture server below dispatches on the query document's
// operation name (e.g. "FindPerformer") rather than the request path.
package stashdbtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"strings"
	"testing"
)

// Known/unknown IDs and a known fingerprint hash the fixture server
// recognizes; any other value on the same operation yields a null result,
// exactly like a real StashDB "not found" GraphQL response.
const (
	KnownPerformerID   = "11111111-1111-1111-1111-111111111111"
	UnknownPerformerID = "00000000-0000-0000-0000-000000000001"
	KnownStudioID      = "22222222-2222-2222-2222-222222222222"
	UnknownStudioID    = "00000000-0000-0000-0000-000000000002"
	KnownSceneID       = "33333333-3333-3333-3333-333333333333"
	UnknownSceneID     = "00000000-0000-0000-0000-000000000003"
	KnownFingerprint   = "aaaaaaaaaaaaaaaa"
	UnknownFingerprint = "bbbbbbbbbbbbbbbb"
)

// NewClientFunc constructs a fresh ports.StashDBClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.StashDBClient

// TestStashDBClient runs the shared StashDBClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestStashDBClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupPerformer returns the performer for a known ID", func(t *testing.T) { testLookupPerformerKnown(t, newClient, server.URL) })
	t.Run("LookupPerformer returns ErrNotFound for an unknown ID", func(t *testing.T) { testLookupPerformerUnknown(t, newClient, server.URL) })
	t.Run("SearchPerformers returns matches for a query", func(t *testing.T) { testSearchPerformersMatch(t, newClient, server.URL) })
	t.Run("SearchPerformers returns an empty, non-error result for no matches", func(t *testing.T) { testSearchPerformersNoMatch(t, newClient, server.URL) })
	t.Run("LookupStudio returns the studio for a known ID", func(t *testing.T) { testLookupStudioKnown(t, newClient, server.URL) })
	t.Run("LookupStudio returns ErrNotFound for an unknown ID", func(t *testing.T) { testLookupStudioUnknown(t, newClient, server.URL) })
	t.Run("LookupScene returns the scene for a known ID", func(t *testing.T) { testLookupSceneKnown(t, newClient, server.URL) })
	t.Run("LookupScene returns ErrNotFound for an unknown ID", func(t *testing.T) { testLookupSceneUnknown(t, newClient, server.URL) })
	t.Run("SearchScenes returns matches for a query", func(t *testing.T) { testSearchScenesMatch(t, newClient, server.URL) })
	t.Run("SearchScenes returns an empty, non-error result for no matches", func(t *testing.T) { testSearchScenesNoMatch(t, newClient, server.URL) })
	t.Run("FindScenesByFingerprints returns the scene(s) matching a known fingerprint", func(t *testing.T) { testFindScenesByFingerprintsMatch(t, newClient, server.URL) })
	t.Run("FindScenesByFingerprints returns an empty, non-error result for an unknown fingerprint", func(t *testing.T) { testFindScenesByFingerprintsNoMatch(t, newClient, server.URL) })
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

func testLookupStudioKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	s, err := c.LookupStudio(context.Background(), KnownStudioID)
	if err != nil {
		t.Fatalf("LookupStudio returned error: %v", err)
	}
	if s.ID != KnownStudioID || s.Name != "Known Studio" {
		t.Fatalf("LookupStudio = %+v, want ID=%s Name=Known Studio", s, KnownStudioID)
	}
}

func testLookupStudioUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupStudio(context.Background(), UnknownStudioID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupStudio error = %v, want wrapping ports.ErrNotFound", err)
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

func testFindScenesByFingerprintsMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	scenes, err := c.FindScenesByFingerprints(context.Background(), []ports.SceneFingerprint{
		{Hash: KnownFingerprint, Algorithm: ports.FingerprintAlgorithmOSHash},
	})
	if err != nil {
		t.Fatalf("FindScenesByFingerprints returned error: %v", err)
	}
	if len(scenes) != 1 || scenes[0].ID != KnownSceneID {
		t.Fatalf("FindScenesByFingerprints = %+v, want one result with ID=%s", scenes, KnownSceneID)
	}
}

func testFindScenesByFingerprintsNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	scenes, err := c.FindScenesByFingerprints(context.Background(), []ports.SceneFingerprint{
		{Hash: UnknownFingerprint, Algorithm: ports.FingerprintAlgorithmOSHash},
	})
	if err != nil {
		t.Fatalf("FindScenesByFingerprints returned error: %v", err)
	}
	if len(scenes) != 0 {
		t.Fatalf("FindScenesByFingerprints = %+v, want empty", scenes)
	}
}

// reqVars is the union of every operation's GraphQL variables this fixture
// server needs to inspect — decoding is permissive (unknown/absent JSON
// fields are simply left zero), so one struct covers every operation.
type reqVars struct {
	ID           string                     `json:"id"`
	Term         string                     `json:"term"`
	Fingerprints [][]ports.SceneFingerprint `json:"fingerprints"`
}

// fixtureHandler is a minimal, in-process StashDB GraphQL server: it
// decodes the incoming query's operation name and variables, and answers
// with canned data for the Known* constants above — a null result (no
// GraphQL "errors") for anything else, mirroring how the real API answers
// an unknown ID. Dispatch is split one function per operation purely to
// keep each handler's own branching trivial to follow.
func fixtureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := operationName(r.URL.Query().Get("query"))

		var vars reqVars
		if raw := r.URL.Query().Get("variables"); raw != "" {
			_ = json.Unmarshal([]byte(raw), &vars)
		}

		handler, ok := fixtureOperations[op]
		if !ok {
			http.Error(w, "unknown operation: "+op, http.StatusBadRequest)
			return
		}
		handler(w, vars)
	})
}

var fixtureOperations = map[string]func(w http.ResponseWriter, vars reqVars){
	"FindPerformer":                 handleFindPerformer,
	"SearchPerformer":               handleSearchPerformer,
	"FindStudio":                    handleFindStudio,
	"FindScene":                     handleFindScene,
	"SearchScene":                   handleSearchScene,
	"FindScenesBySceneFingerprints": handleFindScenesBySceneFingerprints,
}

func handleFindPerformer(w http.ResponseWriter, vars reqVars) {
	if vars.ID != KnownPerformerID {
		writeData(w, "findPerformer", nil)
		return
	}
	writeData(w, "findPerformer", ports.Performer{ID: KnownPerformerID, Name: "Known Performer"})
}

func handleSearchPerformer(w http.ResponseWriter, vars reqVars) {
	performers := []ports.Performer{}
	if vars.Term == "Known" {
		performers = append(performers, ports.Performer{ID: KnownPerformerID, Name: "Known Performer"})
	}
	writeData(w, "searchPerformer", performers)
}

func handleFindStudio(w http.ResponseWriter, vars reqVars) {
	if vars.ID != KnownStudioID {
		writeData(w, "findStudio", nil)
		return
	}
	writeData(w, "findStudio", ports.Studio{ID: KnownStudioID, Name: "Known Studio"})
}

func handleFindScene(w http.ResponseWriter, vars reqVars) {
	if vars.ID != KnownSceneID {
		writeData(w, "findScene", nil)
		return
	}
	writeData(w, "findScene", ports.Scene{ID: KnownSceneID, Title: "Known Scene"})
}

func handleSearchScene(w http.ResponseWriter, vars reqVars) {
	scenes := []ports.Scene{}
	if vars.Term == "Known" {
		scenes = append(scenes, ports.Scene{ID: KnownSceneID, Title: "Known Scene"})
	}
	writeData(w, "searchScene", scenes)
}

func handleFindScenesBySceneFingerprints(w http.ResponseWriter, vars reqVars) {
	result := [][]ports.Scene{}
	if len(vars.Fingerprints) == 1 {
		if len(vars.Fingerprints[0]) == 1 && vars.Fingerprints[0][0].Hash == KnownFingerprint {
			result = append(result, []ports.Scene{{ID: KnownSceneID, Title: "Known Scene"}})
		} else {
			result = append(result, []ports.Scene{})
		}
	}
	writeData(w, "findScenesBySceneFingerprints", result)
}

// operationName extracts "FindPerformer" from a query document opening
// with "query FindPerformer($id: ID!) { ... }" — every query this
// adapter/contract mirrors follows that shape (see
// internal/adapters/stashdb/queries.go).
func operationName(query string) string {
	query = strings.TrimSpace(query)
	query = strings.TrimPrefix(query, "query ")
	if i := strings.IndexAny(query, "( "); i >= 0 {
		return query[:i]
	}
	return query
}

func writeData(w http.ResponseWriter, field string, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{field: value},
	})
}
