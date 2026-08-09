// Package fixtureserver is a small, realistic, entirely fictitious
// Prowlarr search result served as a canned pkg/httpclient/httpmock route
// — the same real request shape internal/adapters/prowlarr.Client issues
// (GET {base_url}/search), so a Client constructed with
// prowlarr.WithBaseTransport(Transport()) behaves exactly as it would
// against a real Prowlarr instance, just against fixed, known data instead
// of a live network call — no real socket, not even loopback.
//
// This exists specifically so k6 CI can exercise IndexerService.Search
// (issue #584) end to end without a real, operator-run Prowlarr instance.
// cmd/purser's serve command builds this Transport in-process and injects
// it via prowlarr.WithBaseTransport when PURSER_PROWLARR_MOCK is set; no
// separate server process is involved — mirrors
// internal/adapters/musicbrainz/fixtureserver's identical convention. See
// docs/adr/0003-go-testing-standards.md's "recorded request/response
// fixtures" convention and internal/ports/indexertest's contract-test
// equivalent, which this mirrors as a fuller, k6-facing dataset.
package fixtureserver

import (
	"net/http"
	"purser/pkg/httpclient/httpmock"
)

// Known query/values this fixture set recognizes — deliberately distinct
// from internal/ports/indexertest's own KnownQuery/KnownGUID (a different,
// contract-test-only fixture set) since this one is k6-facing.
// test/k6/grpc/indexer_test.js and test/k6/http/indexer_test.js reference
// these same values by string literal — Go and JS can't share constants
// directly, keep any change to these in sync with those two files.
const (
	KnownQuery = "K6 Fixture Release"
	KnownGUID  = "k6-fixture-release-guid"
	KnownTitle = "K6 Fixture Release 1080p"
)

// searchPath is the path component the real request hits under Make's
// _k6-app-start config (prowlarr.base_url: "http://prowlarr.invalid/api/v1")
// — RoundTrip intercepts before any DNS/dial happens, so pointing at a
// fake hostname is harmless, same convention
// musicbrainz/fixtureserver's mbAPIPrefix uses.
const searchPath = "/api/v1/search"

// prowlarrReleaseFixture mirrors internal/adapters/prowlarr's own
// (unexported) wire-shape struct field-for-field, kept as its own
// independent type here — a fixture is a recorded response shape in its
// own right, not a reuse of the adapter's internal decoding type.
type prowlarrReleaseFixture struct {
	GUID        string                    `json:"guid"`
	Title       string                    `json:"title"`
	Size        int64                     `json:"size"`
	IndexerID   int                       `json:"indexerId"`
	Indexer     string                    `json:"indexer"`
	PublishDate string                    `json:"publishDate"`
	DownloadURL string                    `json:"downloadUrl"`
	MagnetURL   string                    `json:"magnetUrl"`
	InfoURL     string                    `json:"infoUrl"`
	InfoHash    string                    `json:"infoHash"`
	Seeders     int                       `json:"seeders"`
	Leechers    int                       `json:"leechers"`
	Protocol    string                    `json:"protocol"`
	Categories  []prowlarrCategoryFixture `json:"categories"`
}

type prowlarrCategoryFixture struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Transport returns the fixture Prowlarr /search endpoint as a
// *httpmock.Transport. KnownQuery returns one release; any other query
// returns an empty result set — never a 404, matching
// ports.IndexerSearcher's own "empty is not an error" search contract.
func Transport() *httpmock.Transport {
	known := []prowlarrReleaseFixture{{
		GUID:        KnownGUID,
		Title:       KnownTitle,
		Size:        734003200,
		IndexerID:   1,
		Indexer:     "K6 Fixture Indexer",
		PublishDate: "2024-01-01T00:00:00Z",
		DownloadURL: "http://prowlarr.invalid/download/k6-fixture",
		InfoURL:     "http://prowlarr.invalid/info/k6-fixture",
		InfoHash:    "0123456789abcdef0123456789abcdef01234567",
		Seeders:     42,
		Leechers:    3,
		Protocol:    "torrent",
		Categories:  []prowlarrCategoryFixture{{ID: 3000, Name: "Music"}},
	}}

	return httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   searchPath,
		Responder: func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("query") == KnownQuery {
				return httpmock.JSON(http.StatusOK, known)(req)
			}
			return httpmock.JSON(http.StatusOK, []prowlarrReleaseFixture{})(req)
		},
	})
}
