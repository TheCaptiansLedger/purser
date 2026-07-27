// Package fixtureserver is a small, realistic, entirely fictitious
// MusicBrainz dataset served as canned pkg/httpclient/httpmock routes — the
// same real request shapes internal/adapters/musicbrainz.Client issues
// (/artist/{mbid}, /release/{mbid}, search routes, etc.), so a Client
// constructed with musicbrainz.WithBaseTransport(Transport()) behaves
// exactly as it would against the real API, just against fixed, known data
// instead of a live network call — no real socket, not even loopback.
//
// This exists specifically so k6 CI can exercise the Music Persister's
// full cascade (docs/technical/pipeline-music-persist.md) — both the
// automatic decide/persist path and the manual AcceptCandidate path —
// without ever touching the real MusicBrainz API. cmd/purser's serve
// command builds this Transport in-process and injects it via
// musicbrainz.WithBaseTransport when PURSER_MUSICBRAINZ_MOCK is set; no
// separate server process is involved. See
// docs/adr/0003-go-testing-standards.md's "recorded request/response
// fixtures" convention and internal/ports/musicbrainztest's contract-test
// equivalent, which this mirrors as a fuller, k6-facing dataset.
package fixtureserver

import (
	"net/http"
	"purser/internal/ports"
	"purser/pkg/httpclient/httpmock"
)

// Known MBIDs/values this fixture set recognizes — deliberately
// sequential, obviously-fake UUIDs (never real MusicBrainz identifiers),
// mirroring internal/ports/musicbrainztest's Known.../Unknown... contract-test
// convention. test/k6/flow/accept_candidate_test.js (and its HTTP variant)
// reference these same values by string literal — Go and JS can't share
// constants directly, so keep any change to these in sync with that file.
//
// Two releases exist:
//   - ReleaseMBID: a two-track album, its tracks carrying MUSICBRAINZ_ALBUMID
//     tags — resolves via M7's direct-ID short-circuit with zero search
//     calls, exercising the automatic decide/persist path end to end.
//   - ReleaseMBID2: a one-track "album" whose fixture audio file carries no
//     MusicBrainz tags at all and won't fuzzy-match anything (every search
//     route below always returns empty) — identify finds nothing, decide
//     never persists, the group stays in the review queue, and the k6 flow
//     calls AcceptCandidate(groupKey, ReleaseMBID2) directly to exercise the
//     manual, human-supplied-MBID path.
const (
	ArtistMBID          = "11111111-1111-1111-1111-111111111111"
	ReleaseGroupMBID    = "22222222-2222-2222-2222-222222222222"
	ReleaseMBID         = "33333333-3333-3333-3333-333333333333"
	Track1RecordingMBID = "44444444-4444-4444-4444-444444444444"
	Track2RecordingMBID = "55555555-5555-5555-5555-555555555555"

	ArtistMBID2         = "66666666-6666-6666-6666-666666666666"
	ReleaseGroupMBID2   = "77777777-7777-7777-7777-777777777777"
	ReleaseMBID2        = "88888888-8888-8888-8888-888888888888"
	Track3RecordingMBID = "99999999-9999-9999-9999-999999999999"

	ArtistName  = "K6 Fixture Artist"
	AlbumTitle  = "K6 Fixture Album"
	Track1Title = "K6 Fixture Track One"
	Track2Title = "K6 Fixture Track Two"

	ArtistName2 = "K6 Ambiguous Artist"
	AlbumTitle2 = "K6 Ambiguous Album"
	Track3Title = "K6 Ambiguous Track"
)

// mbAPIPrefix is internal/adapters/musicbrainz's real, unmodified default
// BaseURL's path root — routes below match against it directly since
// cmd/purser never overrides BaseURL for the mock path: RoundTrip
// intercepts before any DNS/dial happens, so pointing at the real hostname
// is harmless and one less thing to keep in sync.
const mbAPIPrefix = "/ws/2/"

// Transport returns the fixture MusicBrainz API as a *httpmock.Transport —
// every route internal/adapters/musicbrainz.Client issues, keyed on the
// MBID constants above. An unknown MBID on a direct-lookup route isn't
// registered at all (a bug calling one is a loud httpmock "no route
// registered" failure, not a silently-plausible 404); the search routes
// always return an empty result set (never 404 — matching
// ports.MusicBrainzClient's own "empty is not an error" search contract)
// since this fixture set never needs to fuzzy-match anything: the fixture
// data is designed to resolve entirely through direct-ID lookups or the
// manual AcceptCandidate path.
func Transport() *httpmock.Transport {
	release := releaseFixture()
	ambiguousRelease := ambiguousReleaseFixture()

	return httpmock.New(
		route(ArtistMBID, ports.Artist{ID: ArtistMBID, Name: ArtistName, SortName: ArtistName, Type: "Group"}),
		route(ArtistMBID2, ports.Artist{ID: ArtistMBID2, Name: ArtistName2, SortName: ArtistName2, Type: "Group"}),
		httpmock.Route{Method: http.MethodGet, Path: mbAPIPrefix + "artist", Responder: httpmock.JSON(http.StatusOK, struct {
			Artists []ports.Artist `json:"artists"`
		}{[]ports.Artist{}})},

		httpmock.Route{Method: http.MethodGet, Path: mbAPIPrefix + "release-group", Responder: httpmock.JSON(http.StatusOK, struct {
			ReleaseGroups []ports.ReleaseGroup `json:"release-groups"`
		}{[]ports.ReleaseGroup{}})},

		httpmock.Route{Method: http.MethodGet, Path: mbAPIPrefix + "release/" + ReleaseMBID, Responder: httpmock.JSON(http.StatusOK, release)},
		httpmock.Route{Method: http.MethodGet, Path: mbAPIPrefix + "release/" + ReleaseMBID2, Responder: httpmock.JSON(http.StatusOK, ambiguousRelease)},
		httpmock.Route{Method: http.MethodGet, Path: mbAPIPrefix + "release", Responder: httpmock.JSON(http.StatusOK, struct {
			Releases []ports.Release `json:"releases"`
		}{[]ports.Release{}})},
	)
}

func route(mbid string, artist ports.Artist) httpmock.Route {
	return httpmock.Route{
		Method:    http.MethodGet,
		Path:      mbAPIPrefix + "artist/" + mbid,
		Responder: httpmock.JSON(http.StatusOK, artist),
	}
}

// releaseFixture builds ReleaseMBID: a single-disc, two-track release with
// a full artist credit and release group, real-shaped Recording MBIDs/
// ISRCs — everything the Music Persister's cascade
// (docs/technical/pipeline-music-persist.md) needs to resolve Artist ->
// Release Group -> MusicRelease -> Items/MediaFiles end to end. Track
// numbers/positions match test/k6/fixtures/musicbrainz-audio/01-*.flac and
// 02-*.flac (see that directory's generate.sh).
func releaseFixture() ports.Release {
	rg := ports.ReleaseGroup{ID: ReleaseGroupMBID, Title: AlbumTitle, PrimaryType: "Album"}
	artistCredit := []ports.ArtistCredit{{Name: ArtistName, Artist: ports.RelationArtist{ID: ArtistMBID, Name: ArtistName}}}

	return ports.Release{
		ID:           ReleaseMBID,
		Title:        AlbumTitle,
		Country:      "XW",
		Date:         "2024-01-01",
		ReleaseGroup: &rg,
		ArtistCredit: artistCredit,
		Media: []ports.Medium{
			{
				Position:   1,
				Format:     "Digital Media",
				TrackCount: 2,
				Tracks: []ports.Track{
					{
						Position: 1, Number: "1", Title: Track1Title, Length: 5000,
						Recording: &ports.Recording{ID: Track1RecordingMBID, Title: Track1Title, ISRCs: []string{"XXK6F0000001"}},
					},
					{
						Position: 1, Number: "2", Title: Track2Title, Length: 5000,
						Recording: &ports.Recording{ID: Track2RecordingMBID, Title: Track2Title, ISRCs: []string{"XXK6F0000002"}},
					},
				},
			},
		},
	}
}

// ambiguousReleaseFixture builds ReleaseMBID2: a single-track release with
// no embedded-tag path to it at all — only reachable via a human supplying
// ReleaseMBID2 directly to AcceptCandidate. Matches
// test/k6/fixtures/musicbrainz-audio/03-*.flac's (disc, track) position.
func ambiguousReleaseFixture() ports.Release {
	rg := ports.ReleaseGroup{ID: ReleaseGroupMBID2, Title: AlbumTitle2, PrimaryType: "Album"}
	artistCredit := []ports.ArtistCredit{{Name: ArtistName2, Artist: ports.RelationArtist{ID: ArtistMBID2, Name: ArtistName2}}}

	return ports.Release{
		ID:           ReleaseMBID2,
		Title:        AlbumTitle2,
		Country:      "XW",
		Date:         "2024-01-01",
		ReleaseGroup: &rg,
		ArtistCredit: artistCredit,
		Media: []ports.Medium{
			{
				Position:   1,
				Format:     "Digital Media",
				TrackCount: 1,
				Tracks: []ports.Track{
					{
						Position: 1, Number: "1", Title: Track3Title, Length: 5000,
						Recording: &ports.Recording{ID: Track3RecordingMBID, Title: Track3Title},
					},
				},
			},
		},
	}
}
