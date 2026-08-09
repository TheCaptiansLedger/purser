# Indexer Search (Prowlarr adapter)

Design for the `IndexerSearcher` port and its Prowlarr adapter — see
[acquisition-pipeline.md](acquisition-pipeline.md) for why this is an
ordinary capability-shaped port rather than a
[0027](../adr/0027-provider-independence.md)-style provider exception,
and for how this fits into the broader acquisition slice alongside
[acquisition-download-client.md](acquisition-download-client.md).

## Scope

In scope: a single `Search` capability against whichever indexer backend
is configured, returning a provider-neutral release list. Explicitly
out of scope: submitting a release for download (a separate port/
service, see the sibling doc above), an automatic search loop over
monitored library items, and any quality/relevance scoring of results —
Purser returns the backend's own order, never ranks or filters
server-side, the same discipline
[0027](../adr/0027-provider-independence.md) already establishes for
provider results.

## Required infrastructure reuse

- **HTTP client:** `pkg/httpclient.New(cfg)` (never a bare
  `http.Client`), the same convention `internal/adapters/stashdb` and
  `internal/adapters/theaudiodb` already follow — telemetry hooks
  (`httpclient.WithTracerProvider`/`WithMeterProvider`/`WithLogger`)
  wired the same way.
- **Config:** `internal/config.Prowlarr{Enabled, BaseURL, APIKey}`, per
  [0010](../adr/0010-configuration.md) — `BaseURL` is required, not a
  soft override with a fallback default the way `config.MusicBrainz`'s
  is, because there is no public default Prowlarr instance; every
  deployment runs its own.
- **Errors:** `ports.ErrNotFound` (`internal/ports/errors.go`) for a
  404, per [0011](../adr/0011-api-design.md)'s sentinel-error
  convention — though per the port shape below, a zero-result search is
  a valid empty slice, never `ErrNotFound`, matching every other
  Search/List method in this codebase (`ports.MusicBrainzClient`,
  `ports.TheAudioDBClient`).

## Prowlarr-specific gaps

Prowlarr's real API (`GET /api/v1/search`, `X-Api-Key` header) returns a
flat JSON array of release objects — `guid`, `title`, `size`,
`indexerId`, `indexer`, `publishDate`, `downloadUrl`, `magnetUrl`
(torrent only), `infoUrl`, `infoHash`, `seeders`, `leechers`,
`protocol` (`"torrent"`/`"usenet"`), and `categories` (`[]{id, name}`).
Category filtering is by Prowlarr's own numeric category IDs (Newznab/
Torznab standard category numbers, e.g. `2000` for movies, `3000` for
music) — `IndexerSearchParams.Categories` passes these through as
opaque `int`s; knowing what they mean is the adapter's job, not the
port's. `protocol` maps directly onto the shared `Protocol` type both
this port and `DownloadClient` use.

## Error mapping

Prowlarr returns HTTP 401/403 for a bad or missing API key and HTTP 5xx
for an indexer-level failure it can't recover from; both map to a plain
Go error (not `ports.ErrNotFound`) since neither is "the thing you
searched for doesn't exist," the same "only 404 becomes the sentinel"
rule `music-musicbrainz-adapter.md`'s error-mapping section already
uses.

## Port shape

`internal/ports.IndexerSearcher` exposes:

| Method | Purpose |
|---|---|
| `Search(ctx, params IndexerSearchParams) ([]IndexerRelease, error)` | Free-text search across every indexer the backend has enabled, returned in the backend's own order — no server-side ranking. |

`IndexerSearchParams`: `Query string`, `Categories []int`,
`IndexerIDs []int` (optional — restrict to specific configured
indexers; empty means "all enabled").

`IndexerRelease`: `Guid, Title, IndexerName string`, `Size int64`,
`Protocol Protocol`, `PublishDate time.Time`, `Seeders, Leechers int`,
`DownloadURL, MagnetURL, InfoURL, InfoHash string`,
`Categories []Category` (`{ID int, Name string}`). Deliberately named
`IndexerRelease`, not `Release` — `ports.Release` already names
MusicBrainz's own DTO in `internal/ports/musicbrainz.go`, and reusing
it here would collide two genuinely different shapes in one package.

## Testing

Per [0003](../adr/0003-go-testing-standards.md): the default suite runs
against a fixture-backed fake Prowlarr HTTP server
(`httptest.Server`/`pkg/httpclient/httpmock`), no live network
required — same convention this project's k6 suites already use for
external adapters. A `live`-build-tag-gated test, run manually against a
real Prowlarr instance and API key, mirrors
`internal/adapters/musicbrainz/musicbrainz_test.go`'s convention for
catching real-API drift the fixtures can't.
