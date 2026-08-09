// Package downloadclienttest is the shared contract test suite for the
// ports.DownloadClient port. Like musicbrainztest, this port is
// network-backed: the suite runs its own httptest.Server serving small
// canned fixtures and hands the adapter constructor that server's URL.
// This suite only proves the port's contract — Add/Status/Remove
// round-tripping and ErrNotFound mapping for an unknown external ID — not
// any real client backend's full request/response shape; richer
// field-mapping and error-path assertions against a real backend's
// responses belong in the adapter's own package
// (docs/adr/0003-go-testing-standards.md).
//
// TestDownloadClient takes the protocol under test and picks the matching
// fixture server shape: qBittorrent's real endpoint shape
// (/api/v2/auth/login, /api/v2/torrents/{add,info,delete}) for
// ports.ProtocolTorrent, or SABnzbd's real endpoint shape (a single
// /api routed by a "mode" query parameter) for ports.ProtocolUsenet — the
// same way indexertest's fixture mirrors Prowlarr's real /search endpoint.
// ADR 0003 requires every adapter for a port to run that port's shared
// contract test against its real, production request-building code, and
// neither backend's distinctive request shape (qBittorrent's
// session-cookie login and nested /api/v2/torrents/* paths; SABnzbd's
// single mode-routed endpoint) can be reached by one vendor-agnostic
// placeholder path set.
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
// newClient, using a fixture HTTP server shaped for protocol
// (ports.ProtocolTorrent or ports.ProtocolUsenet) that this suite owns.
func TestDownloadClient(t *testing.T, protocol ports.Protocol, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler(protocol))
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

func fixtureHandler(protocol ports.Protocol) http.Handler {
	if protocol == ports.ProtocolUsenet {
		return sabnzbdFixtureHandler()
	}
	return qbittorrentFixtureHandler()
}

// fixtureTorrent is qBittorrent's own /api/v2/torrents/info wire shape —
// see internal/adapters/qbittorrent's qbTorrent for the real adapter's
// identical decode target.
type fixtureTorrent struct {
	Hash     string  `json:"hash"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	SavePath string  `json:"save_path"`
	ETA      int64   `json:"eta"`
}

var knownTorrent = fixtureTorrent{
	Hash:     KnownExternalID,
	State:    "downloading",
	Progress: 0.5,
	SavePath: "/downloads/some-release",
	ETA:      3600,
}

func qbittorrentFixtureHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		// Set-Cookie written directly rather than http.Cookie+http.SetCookie
		// — an http.Cookie literal missing Secure/HttpOnly/SameSite trips
		// gosec's G124, and this plain-http httptest.Server fixture has no
		// real session to protect.
		w.Header().Set("Set-Cookie", "SID=contract-test-session; Path=/")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Ok."))
	})

	mux.HandleFunc("/api/v2/torrents/add", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Ok."))
	})

	mux.HandleFunc("/api/v2/torrents/info", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// A post-Add tag lookup always resolves to the one known torrent
		// this fixture models — this suite doesn't track per-request tags,
		// only the port's Add/Status/Remove contract.
		if q.Get("tag") != "" {
			writeJSON(w, []fixtureTorrent{knownTorrent})
			return
		}
		if q.Get("hashes") != KnownExternalID {
			writeJSON(w, []fixtureTorrent{})
			return
		}
		writeJSON(w, []fixtureTorrent{knownTorrent})
	})

	mux.HandleFunc("/api/v2/torrents/delete", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Ok."))
	})

	return mux
}

// fixtureQueueSlot is SABnzbd's own "mode=queue" per-job wire shape — see
// internal/adapters/sabnzbd's queueSlot for the real adapter's identical
// decode target. The known job is modeled as actively downloading so
// testStatusKnown's expected ports.DownloadStateDownloading holds
// regardless of which protocol's fixture is in play.
var knownQueueSlot = map[string]any{
	"nzo_id":     KnownExternalID,
	"status":     "Downloading",
	"percentage": "50",
	"timeleft":   "0:10:00",
}

// sabnzbdFixtureHandler serves SABnzbd's real single-endpoint,
// mode-routed API shape (see internal/adapters/sabnzbd's package doc
// comment) rather than a made-up generic one. The known job always lives
// in the queue, never history — this suite only needs one known job to
// prove the port's contract; richer queue-vs-history routing assertions
// belong in the adapter's own package.
func sabnzbdFixtureHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch q.Get("mode") {
		case "addurl":
			writeJSON(w, map[string]any{"status": true, "nzo_ids": []string{KnownExternalID}})

		case "queue":
			if q.Get("name") == "delete" {
				if q.Get("value") == KnownExternalID {
					writeJSON(w, map[string]any{"status": true})
					return
				}
				writeJSON(w, map[string]any{"status": false, "error": "not found"})
				return
			}
			if q.Get("nzo_ids") == KnownExternalID {
				writeJSON(w, map[string]any{"queue": map[string]any{"slots": []any{knownQueueSlot}}})
				return
			}
			writeJSON(w, map[string]any{"queue": map[string]any{"slots": []any{}}})

		case "history":
			if q.Get("name") == "delete" {
				writeJSON(w, map[string]any{"status": true})
				return
			}
			// The known job never appears in history for this fixture —
			// it's modeled as still actively downloading (see
			// knownQueueSlot).
			writeJSON(w, map[string]any{"history": map[string]any{"slots": []any{}}})

		default:
			writeJSON(w, map[string]any{"status": false, "error": "unknown mode"})
		}
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
