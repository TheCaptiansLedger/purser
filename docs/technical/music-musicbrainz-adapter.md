# MusicBrainz Adapter (M2)

Design for the MusicBrainz client adapter that backs Music identification
([0025](../adr/0025-music-identification-confidence-scoring.md)) — see
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M2
sits in the overall build order.

## Scope

**In scope:** a single adapter, used as the scan pipeline's identification
source (M7 — candidate generation/scoring). One port, one implementation,
no branching inside it for who's calling.

**Explicitly out of scope for this effort:** a manual "user adds an
artist/release and pulls MusicBrainz metadata to populate it" flow. This is
a real use case and a real future consumer of this same adapter — noted
here so it isn't rediscovered from scratch later — but it is not being
built now. The adapter's port is not narrowed to only what the scan
pipeline needs (see "Port shape" below); that's deliberate so the manual
flow can be added later as a second caller without reshaping the adapter,
but no service/API work for that flow is happening as part of M2.

## Required infrastructure reuse

- **HTTP client:** `pkg/httpclient.New(cfg)` — the adapter must not
  construct its own `http.Client`.
- **Caching:** `httpclient.NewCachingTransport(client.Transport, cache)`
  wrapping the adapter's own named `pkg/cache.Cache` instance (per
  [0010](../adr/0010-configuration.md)'s one-config-component convention).
  MusicBrainz metadata is close to static, so this cache's `DefaultTTL`
  should be configured well above the global default (15 min) — a starting
  point of several hours to a day, tunable, not asserted correct here.

## Two things the shared packages don't give you, both MusicBrainz-specific

1. **Client-side rate limiting.** MusicBrainz enforces 1 request/second and
   will 503 above it. Neither `pkg/httpclient` nor `pkg/cache` rate-limit —
   this is MB's own policy, not a generic HTTP concern, so the adapter owns
   a limiter (e.g. `golang.org/x/time/rate`, `Wait(ctx)` before every
   request) in front of the client.
2. **The `User-Agent` header MB requires by policy.** Fixed, not
   configurable: `"Purser/" + version.Version` (`internal/version`, per
   [0017](../adr/0017-build-and-release-goreleaser.md)) — build metadata
   (commit/date) is not included, just the app name and version. No
   `PURSER_SOURCES_MUSICBRAINZ_USER_AGENT` config key. Set once at
   `httpclient.Config.UserAgent` when the adapter constructs its client.

## Error mapping

MB's 404 (unknown MBID) maps to `ports.ErrNotFound` — the existing
sentinel (`internal/ports/errors.go`) — not a leaked HTTP status. Callers
branch on the sentinel, never on `*http.Response`.

## Port shape

Returns MusicBrainz's own data shapes (artist/release-group/release/
recording DTOs, roughly mirroring the MB JSON responses documented in
`docs/technical/music-data_model.md` §2), not Purser domain types. Mapping
a DTO into `LibraryEntry`/`Group`/`MusicRelease`/`Item` is the caller's
job — the scan identifier (M7) for the scan path — not the adapter's. This
keeps the adapter single-purpose and testable against fixture HTTP
responses without touching domain code.

| Method | Used by scan pipeline (M7) for |
|---|---|
| `LookupArtist(ctx, mbid)` | Resolving an embedded/candidate artist MBID |
| `SearchArtists(ctx, query)` | (available; not exercised by the scan path today) |
| `LookupReleaseGroup(ctx, mbid)` | Resolving an embedded/candidate release-group MBID |
| `ListReleaseGroupsForArtist(ctx, artistMBID)` | Discography listing for a known artist |
| `SearchReleaseGroups(ctx, artistName, albumName string)` | Free-text fuzzy candidate generation — no MBID needed on either side. Fed by tag-derived guesses (`ALBUMARTIST`/`ALBUM`) and by [pipeline-music-filename-parser.md](pipeline-music-filename-parser.md) (M6)'s filename-derived guesses — same method, different string sources |
| `LookupRelease(ctx, mbid)` | Full track listing for scoring (track count, title set, durations) |
| `ListReleasesForReleaseGroup(ctx, rgMBID)` | Enumerating candidate editions/pressings |
| `SearchReleaseByBarcode(ctx, barcode)` | Barcode signal |
| `LookupRecordingByISRC(ctx, isrc)` | ISRC signal |

`SearchReleaseGroups` and `ListReleaseGroupsForArtist` were originally one
conflated method (`SearchReleaseGroups(ctx, artistMBID, query)`) — split
once M6's design made clear that fuzzy candidate generation never has an
artist MBID to hand in, tag-derived or filename-derived. "List a known
artist's release groups" and "fuzzy-search by free-text name" are
different MusicBrainz queries, not two shapes of the same call.

`SearchArtists` has no caller in this effort — it exists because the port
isn't scoped narrowly to M7's needs (see "Scope" above), not because
something currently uses it. Kept, not stubbed out, since it costs nothing
to leave in a hand-written client and removing it would just mean adding
it back for the deferred manual-add flow.

## Testing

Per [0003](../adr/0003-go-testing-standards.md): the default test suite for
this adapter (every method in the table above) runs against recorded MB
JSON response fixtures, not the real API — `go test ./...` and CI never
touch the network or the 1 req/sec limiter. A separate, `live`-build-tag-gated
verification test may hit the real MusicBrainz API to catch schema drift;
it is not part of the default suite and is run manually, not on every
commit. M7/M8/M9's own tests never construct this adapter — they depend on
the port and use a fake, per [0003](../adr/0003-go-testing-standards.md)'s
"services test against fakes" rule.
