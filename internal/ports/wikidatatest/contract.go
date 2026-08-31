// Package wikidatatest is the shared contract test suite for the
// ports.WikidataClient port. Like fanarttvtest/theaudiodbtest, this port
// is network-backed, so NewClientFunc takes a baseURL: the suite runs its
// own httptest.Server serving small canned fixtures and hands the adapter
// constructor that server's URL. Richer field-mapping assertions against
// real recorded Wikidata responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md) — this suite only proves the
// port's contract: an entity with an image resolves, an entity with no
// image or no such entity both answer ErrNotFound (see
// internal/ports/wikidata.go's own doc comment for why the two collapse
// to one outcome).
package wikidatatest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known/unknown entity URLs the fixture server recognizes; any other
// syntactically valid QID yields the real API's confirmed
// "no-such-entity" error shape.
const (
	KnownEntityURL   = "https://www.wikidata.org/wiki/Q845084"
	NoImageEntityURL = "https://www.wikidata.org/wiki/Q107350405"
	UnknownEntityURL = "https://www.wikidata.org/wiki/Q999999999"
)

// NewClientFunc constructs a fresh ports.WikidataClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.WikidataClient

// TestWikidataClient runs the shared WikidataClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestWikidataClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("LookupImage returns images for a known entity", func(t *testing.T) { testLookupImageKnown(t, newClient, server.URL) })
	t.Run("LookupImage returns ErrNotFound for an entity with no P18 claim", func(t *testing.T) {
		testLookupImageNotFound(t, newClient, server.URL, NoImageEntityURL)
	})
	t.Run("LookupImage returns ErrNotFound for an unknown entity", func(t *testing.T) {
		testLookupImageNotFound(t, newClient, server.URL, UnknownEntityURL)
	})
}

func testLookupImageKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	images, err := c.LookupImage(context.Background(), KnownEntityURL)
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(images) == 0 || images[0].URL == "" {
		t.Fatalf("LookupImage = %+v, want at least one image with a non-empty URL", images)
	}
}

func testLookupImageNotFound(t *testing.T, newClient NewClientFunc, baseURL, entityURL string) {
	c := newClient(t, baseURL)
	_, err := c.LookupImage(context.Background(), entityURL)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupImage(%s) error = %v, want wrapping ports.ErrNotFound", entityURL, err)
	}
}

// fixtureHandler is a minimal, in-process Wikidata action-API server: it
// dispatches on the "entity" query param, answering with canned
// wbgetclaims shapes for the three constants above.
func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /w/api.php", handleWBGetClaims)
	mux.HandleFunc("GET /", handleWBGetClaims) // baseURL may point directly at the fixture root
	return mux
}

func handleWBGetClaims(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")
	w.Header().Set("Content-Type", "application/json")
	switch entity {
	case "Q845084":
		_, _ = fmt.Fprint(w, `{"claims":{"P18":[{"mainsnak":{"datavalue":{"value":"Known Entity.jpg"}}}]}}`)
	case "Q107350405":
		_, _ = fmt.Fprint(w, `{"claims":{}}`)
	default:
		_, _ = fmt.Fprint(w, `{"error":{"code":"no-such-entity"}}`)
	}
}
