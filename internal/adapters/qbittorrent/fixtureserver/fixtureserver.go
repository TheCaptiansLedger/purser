// Package fixtureserver is a small, realistic, entirely fictitious
// qBittorrent WebUI API served as a canned pkg/httpclient/httpmock route
// table — the same real request shapes internal/adapters/qbittorrent.Client
// issues (session-cookie login, multipart add, hash/tag-filtered info,
// form-encoded delete), so a Client constructed with
// qbittorrent.WithBaseTransport(Transport()) behaves exactly as it would
// against a real qBittorrent instance, just against fixed, known data
// instead of a live network call — no real socket, not even loopback.
//
// This exists specifically so k6 CI can exercise DownloadService's
// Submit/GetStatus/Remove RPCs (#585) for the torrent protocol end to end
// without a real, operator-run qBittorrent instance. cmd/purser's serve
// command builds this Transport in-process and injects it via
// qbittorrent.WithBaseTransport when PURSER_QBITTORRENT_MOCK is set; no
// separate server process is involved — mirrors
// internal/adapters/prowlarr/fixtureserver's identical convention. Unlike
// that read-only fixture, this one holds mutable state (mutex-protected):
// a submit records its tag so the subsequent tag-filtered info lookup
// resolves it to KnownHash, and a remove marks KnownHash absent so a
// following status check reports not-found, same "submit, check, remove,
// check again" lifecycle
// docs/adr/0011-api-design.md's k6 testing section describes. See
// docs/technical/acquisition-download-client.md and
// internal/ports/downloadclienttest's contract-test equivalent, which this
// mirrors as a fuller, k6-facing dataset.
package fixtureserver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"purser/pkg/httpclient/httpmock"
	"sync"
)

// KnownHash is the external ID every submit this fixture answers
// eventually resolves to — test/k6/grpc/download_test.js and
// test/k6/http/download_test.js reference this same value by string
// literal — Go and JS can't share constants directly, keep any change to
// this in sync with those two files.
const KnownHash = "k6-fixture-torrent-hash"

const (
	loginPath  = "/api/v2/auth/login"
	addPath    = "/api/v2/torrents/add"
	infoPath   = "/api/v2/torrents/info"
	deletePath = "/api/v2/torrents/delete"

	sessionCookie = "k6-fixture-sid"
)

// torrentFixture mirrors internal/adapters/qbittorrent's own (unexported)
// qbTorrent wire shape field-for-field — kept as its own independent type
// here, same reasoning prowlarr/fixtureserver's prowlarrReleaseFixture
// gives for not reusing the adapter's internal decoding type.
type torrentFixture struct {
	Hash     string  `json:"hash"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	SavePath string  `json:"save_path"`
	ETA      int64   `json:"eta"`
}

var knownTorrent = torrentFixture{
	Hash:     KnownHash,
	State:    "downloading",
	Progress: 0.42,
	SavePath: "/downloads/k6-fixture",
	ETA:      1800,
}

// state tracks the one thing across calls this fixture needs to: the tag
// most recently submitted via Add (so the adapter's own post-add
// tag-filtered lookup resolves it) and whether KnownHash has since been
// removed (so a status check afterward reports not-found, same as a real
// qBittorrent instance).
type state struct {
	mu      sync.Mutex
	lastTag string
	removed bool
}

func (s *state) setTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTag = tag
	s.removed = false
}

func (s *state) tagMatches(tag string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return tag != "" && tag == s.lastTag
}

func (s *state) markRemoved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = true
}

func (s *state) isRemoved() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removed
}

// Transport returns the fixture qBittorrent WebUI API as a
// *httpmock.Transport. Login always succeeds (this fixture doesn't
// validate credentials, same simplicity prowlarr/fixtureserver's Search
// route uses for its API key). Add always succeeds and remembers the
// submitted tag. The info endpoint resolves either a tag-filtered lookup
// (Add's own post-submit hash discovery) or a hashes-filtered lookup
// (Status) to knownTorrent, unless it has been removed. Delete marks
// KnownHash removed.
func Transport() *httpmock.Transport {
	st := &state{}

	return httpmock.New(
		httpmock.Route{
			Method: http.MethodPost,
			Path:   loginPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				return textResponse(req, http.StatusOK, "Ok.", map[string]string{
					"Set-Cookie": "SID=" + sessionCookie + "; Path=/",
				}), nil
			},
		},
		httpmock.Route{
			Method: http.MethodPost,
			Path:   addPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				if err := req.ParseMultipartForm(1 << 20); err != nil {
					return nil, fmt.Errorf("fixtureserver: parsing add request: %w", err)
				}
				st.setTag(req.FormValue("tags"))
				return textResponse(req, http.StatusOK, "Ok.", nil), nil
			},
		},
		httpmock.Route{
			Method: http.MethodGet,
			Path:   infoPath,
			Responder: func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				known := !st.isRemoved() && (st.tagMatches(q.Get("tag")) || q.Get("hashes") == KnownHash)
				if !known {
					return jsonResponse(req, http.StatusOK, []torrentFixture{})
				}
				return jsonResponse(req, http.StatusOK, []torrentFixture{knownTorrent})
			},
		},
		httpmock.Route{
			Method: http.MethodPost,
			Path:   deletePath,
			Responder: func(req *http.Request) (*http.Response, error) {
				if err := req.ParseForm(); err != nil {
					return nil, fmt.Errorf("fixtureserver: parsing delete request: %w", err)
				}
				if req.FormValue("hashes") == KnownHash {
					st.markRemoved()
				}
				return textResponse(req, http.StatusOK, "Ok.", nil), nil
			},
		},
	)
}

func textResponse(req *http.Request, status int, body string, extraHeaders map[string]string) *http.Response {
	header := http.Header{"Content-Type": []string{"text/plain"}}
	for k, v := range extraHeaders {
		header.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Request:    req,
	}
}

func jsonResponse(req *http.Request, status int, body any) (*http.Response, error) {
	return httpmock.JSON(status, body)(req)
}
