// Package indexertest is the shared contract test suite for the
// ports.IndexerSearcher port. Like musicbrainztest, this port is
// network-backed: the suite runs its own httptest.Server serving small
// canned fixtures and hands the adapter constructor that server's URL.
// This suite only proves the port's contract — decoding and "empty is not
// an error" search semantics — not any real indexer backend's actual
// response shape; richer field-mapping assertions against a real Prowlarr
// response belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md).
package indexertest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known/unknown queries the fixture server recognizes.
const (
	KnownQuery   = "Known Release"
	UnknownQuery = "no-such-release"
	ErrorQuery   = "boom"
)

// KnownGUID is the GUID of the single release the fixture server returns
// for KnownQuery.
const KnownGUID = "known-release-guid"

// NewClientFunc constructs a fresh ports.IndexerSearcher pointed at
// baseURL for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.IndexerSearcher

// TestIndexerSearcher runs the shared IndexerSearcher contract against
// newClient, using a fixture HTTP server this suite owns.
func TestIndexerSearcher(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("Search returns matches for a query", func(t *testing.T) { testSearchMatch(t, newClient, server.URL) })
	t.Run("Search returns an empty, non-error result for no matches", func(t *testing.T) { testSearchNoMatch(t, newClient, server.URL) })
	t.Run("Search returns an error for a backend failure", func(t *testing.T) { testSearchError(t, newClient, server.URL) })
}

func testSearchMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: KnownQuery})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].GUID != KnownGUID {
		t.Fatalf("Search = %+v, want one result with GUID=%s", results, KnownGUID)
	}
}

func testSearchNoMatch(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: UnknownQuery})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search = %+v, want empty", results)
	}
}

func testSearchError(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: ErrorQuery})
	if err == nil {
		t.Fatal("Search for a backend failure returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Search error = %v, want a plain error, not ErrNotFound", err)
	}
}

func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("query") {
		case KnownQuery:
			writeJSON(w, []ports.IndexerRelease{{
				GUID:        KnownGUID,
				Title:       "Some Known Release",
				IndexerName: "fixture-indexer",
				Protocol:    ports.ProtocolTorrent,
				Categories:  []ports.Category{{ID: 3000, Name: "Music"}},
			}})
		case ErrorQuery:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			writeJSON(w, []ports.IndexerRelease{})
		}
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
