// Package fixtureserver is a small, realistic, entirely fictitious SABnzbd
// API served as a canned pkg/httpclient/httpmock route table — the same
// real request shape internal/adapters/sabnzbd.Client issues (a single
// "GET {base_url}/api" endpoint routed by a "mode" query parameter), so a
// Client constructed with sabnzbd.WithBaseTransport(Transport()) behaves
// exactly as it would against a real SABnzbd instance, just against fixed,
// known data instead of a live network call — no real socket, not even
// loopback.
//
// This exists specifically so k6 CI can exercise DownloadService's
// Submit/GetStatus/Remove RPCs (#585) for the usenet protocol end to end
// without a real, operator-run SABnzbd instance. cmd/purser's serve
// command builds this Transport in-process and injects it via
// sabnzbd.WithBaseTransport when PURSER_SABNZBD_MOCK is set; no separate
// server process is involved — mirrors
// internal/adapters/qbittorrent/fixtureserver's identical convention.
// Holds mutable state (mutex-protected): a submit puts KnownNzoID in the
// queue, and a remove takes it back out, so a following status check
// reports not-found — same "submit, check, remove, check again"
// lifecycle docs/adr/0011-api-design.md's k6 testing section describes.
// See docs/technical/acquisition-download-client.md and
// internal/ports/downloadclienttest's contract-test equivalent, which this
// mirrors as a fuller, k6-facing dataset.
package fixtureserver

import (
	"net/http"
	"purser/pkg/httpclient/httpmock"
	"sync"
)

// KnownNzoID is the external ID every submit this fixture answers
// resolves to — test/k6/grpc/download_test.js and
// test/k6/http/download_test.js reference this same value by string
// literal — Go and JS can't share constants directly, keep any change to
// this in sync with those two files.
const KnownNzoID = "SABnzbd_nzo_k6fixture"

const apiPath = "/api"

// queueSlotFixture mirrors internal/adapters/sabnzbd's own (unexported)
// queueSlot wire shape field-for-field — kept as its own independent type
// here, same reasoning internal/adapters/qbittorrent/fixtureserver's
// torrentFixture gives for not reusing the adapter's internal decoding
// type.
type queueSlotFixture struct {
	NzoID      string `json:"nzo_id"`
	Status     string `json:"status"`
	Percentage string `json:"percentage"`
	TimeLeft   string `json:"timeleft"`
}

var knownQueueSlot = queueSlotFixture{
	NzoID:      KnownNzoID,
	Status:     "Downloading",
	Percentage: "37",
	TimeLeft:   "0:16:44",
}

// state tracks whether KnownNzoID is currently "in the queue" — set on a
// successful addurl, cleared on a successful delete — so a status check
// after remove reports not-found, same as a real SABnzbd instance.
type state struct {
	mu      sync.Mutex
	inQueue bool
}

func (s *state) add() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inQueue = true
}

func (s *state) remove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inQueue = false
}

func (s *state) present() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inQueue
}

// Transport returns the fixture SABnzbd API as a *httpmock.Transport. This
// fixture doesn't validate the apikey query parameter, same simplicity
// prowlarr/fixtureserver's Search route uses for its own API key. Every
// call routes through the same single "/api" endpoint, differentiated by
// "mode" — see the package doc comment.
func Transport() *httpmock.Transport {
	st := &state{}

	return httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   apiPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			switch q.Get("mode") {
			case "addurl":
				st.add()
				return httpmock.JSON(http.StatusOK, map[string]any{"status": true, "nzo_ids": []string{KnownNzoID}})(req)

			case "queue":
				if q.Get("name") == "delete" {
					if q.Get("value") == KnownNzoID && st.present() {
						st.remove()
						return httpmock.JSON(http.StatusOK, map[string]any{"status": true})(req)
					}
					return httpmock.JSON(http.StatusOK, map[string]any{"status": false, "error": "not found"})(req)
				}
				return httpmock.JSON(http.StatusOK, queueResponse(q.Get("nzo_ids") == KnownNzoID && st.present()))(req)

			case "history":
				// KnownNzoID never reaches history in this fixture — it's
				// modeled as always still actively downloading (see
				// knownQueueSlot) until removed. A delete against history
				// always reports not found, mirroring a real SABnzbd
				// instance asked to delete an ID it doesn't hold there.
				if q.Get("name") == "delete" {
					return httpmock.JSON(http.StatusOK, map[string]any{"status": false, "error": "not found"})(req)
				}
				return httpmock.JSON(http.StatusOK, map[string]any{"history": map[string]any{"slots": []any{}}})(req)

			default:
				return httpmock.JSON(http.StatusOK, map[string]any{"status": false, "error": "unknown mode"})(req)
			}
		},
	})
}

func queueResponse(includeKnown bool) map[string]any {
	slots := []any{}
	if includeKnown {
		slots = []any{knownQueueSlot}
	}
	return map[string]any{"queue": map[string]any{"slots": slots}}
}
