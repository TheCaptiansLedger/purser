# Download Client (qBittorrent + SABnzbd adapters)

Design for the `DownloadClient` port and its two adapters — see
[acquisition-pipeline.md](acquisition-pipeline.md) for why this is a
capability-shaped port with a protocol-routed adapter registry rather
than a [0027](../adr/0027-provider-independence.md)-style provider
exception, and
[acquisition-indexer-search.md](acquisition-indexer-search.md) for the
sibling port that produces the release this port's `Add` consumes.

## Scope

In scope: submitting a release for download, querying its status, and
removing it, against exactly one configured client per protocol
(torrent → qBittorrent, usenet → SABnzbd). Explicitly out of scope: an
automatic polling loop that watches a submitted download to completion
(a future task, tracked through [0023](../adr/0023-job-queue.md)'s
`Job` model once it exists) and handing a completed download off to
[0024](../adr/0024-pipeline-core.md)'s discover/organize flow. Also out
of scope: priority/tag routing when more than one client shares a
protocol — this slice hard-requires exactly one client per protocol.

## Required infrastructure reuse

- **HTTP client:** `pkg/httpclient.New(cfg)`, same convention as the
  indexer adapter and every existing provider adapter.
- **Config:** `internal/config.QBittorrent{Enabled, BaseURL, Username,
  Password}` and `internal/config.SABnzbd{Enabled, BaseURL, APIKey}`,
  per [0010](../adr/0010-configuration.md) — both `BaseURL`s required,
  same reasoning as Prowlarr's (self-hosted only, no public default).
- **Errors:** `ports.ErrNotFound` for "no download with this external
  ID" on `Status`/`Remove`, per [0011](../adr/0011-api-design.md).

## Adapter-specific gaps

**qBittorrent** has no static API key. Its WebUI API authenticates via
`POST /api/v2/auth/login` with `Username`/`Password`, which returns a
session cookie (`SID`) that must ride every subsequent request; the
adapter owns re-authenticating when a call fails with an
expired-session response. Downloads are added via
`POST /api/v2/torrents/add` (magnet or `.torrent` URL) with an optional
`category` field mapping to a pre-configured save path inside
qBittorrent itself — Purser passes `AddDownloadRequest.Category`
through as-is, no path logic on Purser's side.

**SABnzbd** uses a static API key as a query parameter (`&apikey=...`)
on every call, no login step. Downloads are added via `mode=addurl` (an
`.nzb` URL) with an optional `cat` parameter, the same
category-as-passthrough-string shape as qBittorrent's.

Both adapters normalize their own client-specific state vocabulary
(qBittorrent's `downloading`/`stalledDL`/`pausedDL`/... vs. SABnzbd's
`Downloading`/`Paused`/`Failed`/...) into the shared
`DownloadStatus.State` values below — this mapping is adapter-owned,
per [0001](../adr/0001-hexagonal-architecture.md)'s "adapter-specific
quirks never leak upward into ports" rule.

## Error mapping

Both clients return a plain Go error for a connectivity/auth failure
and `ports.ErrNotFound` only when the client itself reports "no such
download" for a `Status`/`Remove` call against a stale or
already-removed external ID.

## Port shape

`internal/ports.DownloadClient` exposes:

| Method | Purpose |
|---|---|
| `Protocol() Protocol` | Self-declares which protocol (`torrent`/`usenet`) this adapter handles — the composition root uses this to build the routing registry, no switch statement. |
| `Add(ctx, req AddDownloadRequest) (externalID string, err error)` | Submits a release for download; returns the client's own ID for later `Status`/`Remove` calls. |
| `Status(ctx, externalID string) (DownloadStatus, error)` | Reports current progress/state for a previously submitted download. |
| `Remove(ctx, externalID string, deleteFiles bool) error` | Cancels/deletes a submitted download, optionally deleting any partially-downloaded files. |

`AddDownloadRequest`: `Protocol Protocol`, `DownloadURL string`
(magnet, `.torrent` URL, or `.nzb` URL depending on protocol),
`Title string`, `Category string`.

`DownloadStatus`: `ExternalID string`, `State` (normalized:
`queued`/`downloading`/`paused`/`completed`/`failed`),
`Progress float64` (0–1), `SavePath string`, `ETA *time.Duration`.

`Protocol` is the same closed string type
[acquisition-indexer-search.md](acquisition-indexer-search.md)'s
`IndexerRelease.Protocol` uses — shared between both ports so a
release's protocol maps directly onto which registered client handles
it, no translation layer.

## Testing

Per [0003](../adr/0003-go-testing-standards.md): fixture-backed fake
HTTP servers for both qBittorrent's and SABnzbd's APIs, no live network
required for the default suite — including a fixture covering
qBittorrent's login/session-cookie flow. A `live`-build-tag-gated test
per adapter, run manually against a real instance, mirrors the same
convention as the indexer adapter's.
