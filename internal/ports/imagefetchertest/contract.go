// Package imagefetchertest is the shared contract test suite for the
// ports.ImageFetcher port. Like theporndbtest/stashdbtest, this port is
// network-backed: the suite runs its own httptest.Server serving small
// canned fixtures and hands the adapter full URLs pointed at it — unlike
// those two, ImageFetcher.Fetch already takes the full URL as its own
// argument (no adapter-configured BaseURL to prefix onto it), so
// NewFetcherFunc needs no baseURL parameter.
package imagefetchertest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// knownImageBody is served at "/known.jpg" — arbitrary bytes, this
// contract doesn't care about real image format sniffing (that's
// ImageStore's job, not ImageFetcher's).
var knownImageBody = []byte{0xFF, 0xD8, 0xFF, 0xE0, 'k', 'n', 'o', 'w', 'n'}

// NewFetcherFunc constructs a fresh ports.ImageFetcher for the duration of
// a single subtest.
type NewFetcherFunc func(t *testing.T) ports.ImageFetcher

// TestImageFetcher runs the shared ImageFetcher contract against
// newFetcher, using a fixture HTTP server this suite owns.
func TestImageFetcher(t *testing.T, newFetcher NewFetcherFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("Fetch returns the bytes for a known URL", func(t *testing.T) { testFetchKnown(t, newFetcher, server.URL) })
	t.Run("Fetch returns ErrNotFound for an unknown URL", func(t *testing.T) { testFetchUnknown(t, newFetcher, server.URL) })
	t.Run("Fetch returns an error for a server error", func(t *testing.T) { testFetchServerError(t, newFetcher, server.URL) })
}

func fixtureHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/known.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(knownImageBody)
	})
	mux.HandleFunc("/broken.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return mux
}

func testFetchKnown(t *testing.T, newFetcher NewFetcherFunc, baseURL string) {
	f := newFetcher(t)
	rc, err := f.Fetch(context.Background(), baseURL+"/known.jpg")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading Fetch result returned error: %v", err)
	}
	if string(got) != string(knownImageBody) {
		t.Fatalf("Fetch body = %v, want %v", got, knownImageBody)
	}
}

func testFetchUnknown(t *testing.T, newFetcher NewFetcherFunc, baseURL string) {
	f := newFetcher(t)
	_, err := f.Fetch(context.Background(), baseURL+"/missing.jpg")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Fetch error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testFetchServerError(t *testing.T, newFetcher NewFetcherFunc, baseURL string) {
	f := newFetcher(t)
	_, err := f.Fetch(context.Background(), baseURL+"/broken.jpg")
	if err == nil {
		t.Fatal("Fetch for a server error returned nil error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Fetch error = %v, want a plain error, not ErrNotFound", err)
	}
}
