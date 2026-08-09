// Package downloadclienttest is the shared contract test suite for the
// ports.DownloadClient port. Like musicbrainztest, this port is
// network-backed: the suite runs its own httptest.Server serving small
// canned fixtures and hands the adapter constructor that server's URL.
// This suite only proves the port's contract — Add/Status/Remove
// round-tripping and ErrNotFound mapping for an unknown external ID — not
// any real client backend's actual request/response shape; richer
// field-mapping assertions against a real qBittorrent/SABnzbd response
// belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md).
package downloadclienttest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// KnownExternalID is the external ID the fixture server recognizes for
// Status/Remove; any other value yields "not found," exactly like a real
// client backend queried about a stale or already-removed download.
const (
	KnownExternalID   = "known-download-id"
	UnknownExternalID = "unknown-download-id"
)

// NewClientFunc constructs a fresh ports.DownloadClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.DownloadClient

// TestDownloadClient runs the shared DownloadClient contract against
// newClient, using a fixture HTTP server this suite owns.
func TestDownloadClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("Protocol reports a valid protocol", func(t *testing.T) { testProtocol(t, newClient, server.URL) })
	t.Run("Add returns an external ID", func(t *testing.T) { testAdd(t, newClient, server.URL) })
	t.Run("Status returns the download status for a known external ID", func(t *testing.T) { testStatusKnown(t, newClient, server.URL) })
	t.Run("Status returns ErrNotFound for an unknown external ID", func(t *testing.T) { testStatusUnknown(t, newClient, server.URL) })
	t.Run("Remove succeeds for a known external ID", func(t *testing.T) { testRemoveKnown(t, newClient, server.URL) })
	t.Run("Remove returns ErrNotFound for an unknown external ID", func(t *testing.T) { testRemoveUnknown(t, newClient, server.URL) })
}

func testProtocol(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	p := c.Protocol()
	if p != ports.ProtocolTorrent && p != ports.ProtocolUsenet {
		t.Fatalf("Protocol() = %q, want one of %q/%q", p, ports.ProtocolTorrent, ports.ProtocolUsenet)
	}
}

func testAdd(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	id, err := c.Add(context.Background(), ports.AddDownloadRequest{
		DownloadURL: "magnet:?xt=urn:btih:known",
		Title:       "Some Release",
		Category:    "music",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id != KnownExternalID {
		t.Fatalf("Add id = %q, want %q", id, KnownExternalID)
	}
}

func testStatusKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	status, err := c.Status(context.Background(), KnownExternalID)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ExternalID != KnownExternalID || status.State != ports.DownloadStateDownloading {
		t.Fatalf("Status = %+v, want ExternalID=%s State=%s", status, KnownExternalID, ports.DownloadStateDownloading)
	}
}

func testStatusUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.Status(context.Background(), UnknownExternalID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func testRemoveKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	if err := c.Remove(context.Background(), KnownExternalID, false); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
}

func testRemoveUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	err := c.Remove(context.Background(), UnknownExternalID, false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Remove error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func fixtureHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/add", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, struct {
			ID string `json:"id"`
		}{KnownExternalID})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != KnownExternalID {
			writeNotFound(w)
			return
		}
		writeJSON(w, ports.DownloadStatus{
			ExternalID: KnownExternalID,
			State:      ports.DownloadStateDownloading,
			Progress:   0.5,
			SavePath:   "/downloads/some-release",
		})
	})

	mux.HandleFunc("/remove", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != KnownExternalID {
			writeNotFound(w)
			return
		}
		w.WriteHeader(http.StatusOK)
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
