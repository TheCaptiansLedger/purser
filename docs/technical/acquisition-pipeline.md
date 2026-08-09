# Acquisition Pipeline: Indexer Search + Download Submission

> Status: proposal, not yet an ADR-backed decision — the ADR for the
> `IndexerSearcher`/`DownloadClient` port shapes this doc describes gets
> written once the design below is implemented and the pattern is
> proven, not before.

This is the first implementation slice of Purser's acquisition
pipeline, the "go get what you're missing" half of the tool description
in the project README, layered on top of
[0024](../adr/0024-pipeline-core.md)'s existing scan/identify/organize
core the same way that ADR's discover step feeds identification — here,
the input is a chosen network release instead of a file already on
disk. Two focused component docs cover the actual port/adapter design:
[acquisition-indexer-search.md](acquisition-indexer-search.md) and
[acquisition-download-client.md](acquisition-download-client.md); this
doc covers the shape decisions that span both.

## Scope

In scope: searching a configured indexer backend for releases matching
a query, submitting a chosen release to a configured download client,
and manually querying/removing a submitted download. Explicitly out of
scope for this slice: an automatic search loop over `Monitored`
`LibraryEntry`/`Group`/`Item` rows (a future task, once this session's
pieces exist to drive it), the quality-profile scoring that loop would
need to auto-pick a release without a human, the handoff of a completed
download into [0024](../adr/0024-pipeline-core.md)'s discover/hash/
organize flow, and priority/tag routing when more than one download
client shares a protocol (this slice assumes exactly one client per
protocol). The ADR for the two new port shapes below is also
deliberately deferred — written once this design is implemented and
proven, per the project's convention that a technical doc can precede
its ADR.

## Data flow

A caller (UI or CLI) calls `IndexerService.Search` with a free-text
query, gets back an unranked list of `IndexerRelease`s (the indexer's
own order — Purser never ranks or merges these, matching the "no
server-side ranking" convention
[0027](../adr/0027-provider-independence.md) already established for
provider results), picks one, and calls `DownloadService.SubmitDownload`
with the fields it needs off that release (protocol, download URL,
title, category). That call returns an adapter-assigned external ID
immediately — the download itself runs asynchronously against whichever
client backend handles that protocol. The caller can later call
`DownloadService.GetDownloadStatus` with that external ID to poll
progress, or `RemoveDownload` to cancel/delete it. Nothing in this slice
watches that external ID automatically; that's the deferred monitoring
task.

## Design decisions

**`IndexerSearcher` and `DownloadClient` are ordinary capability-shaped
ports, not [0027](../adr/0027-provider-independence.md)-shaped provider
exceptions.** 0027's one-port-per-provider rule exists to stop the
*server* from merging two simultaneously-consulted, competing sources
(StashDB vs. ThePornDB, both answering the same question at once, a
human picks between them). Neither indexer search nor download
submission is that shape: an operator configures exactly one active
indexer backend and one client per protocol, swappable, never two
competing answers to merge. That's the same shape
`internal/ports/image_fetcher.go`'s `ImageFetcher` already uses in this
codebase ("provider-agnostic... shared infrastructure... not one
adapter per provider the way 0027 requires") and the same shape
[0012](../adr/0012-datastore-persistence.md)'s `Datastore` uses for
Badger vs. SQL. Prowlarr fulfills `IndexerSearcher` today; a future
Jackett or NZBHydra2 adapter could fulfill the identical interface with
zero change to the port, the service, or the proto surface — the whole
point of building it this way rather than naming the port
`ProwlarrClient`.

**`IndexerRelease` is a provider-neutral DTO, not `ProwlarrRelease`.**
Beyond following from the point above, this is grounded in how these
tools actually work: Prowlarr, Jackett, and NZBHydra2 all sit on top of
the same underlying Torznab (torrent) / Newznab (usenet) protocol, so
"a release search result" is already a reasonably standardized shape
across indexer-aggregator tools, not a bespoke per-provider schema the
way MusicBrainz's identity graph is. The name `IndexerRelease` (not
`Release`) also avoids colliding with `ports.Release`, MusicBrainz's own
DTO in `internal/ports/musicbrainz.go`.

**`DownloadClient.Protocol()` is a self-declaring capability, the same
fan-out shape 0024 already validates.** The composition root builds a
`map[Protocol]DownloadClient` from every configured adapter's declared
`Protocol()`, the same registry pattern
[0024](../adr/0024-pipeline-core.md) uses for `FileFingerprinter`/
grouping's `ContentTypes()` — adding a third download-client adapter
later is a new implementation, never an edit to `Download`'s routing
logic.

**Search and submit are two separate `internal/service` types, never
one.** `IndexerSearch` depends only on `ports.IndexerSearcher`;
`Download` depends only on the `DownloadClient` registry. Neither
imports the other's port, per [0011](../adr/0011-api-design.md)'s
no-God-service self-audit rule — the caller is what chains "search, let
a human pick, submit," the same place 0027 already puts multi-step
orchestration.

**All three `DownloadService` RPCs ship now, even though nothing
auto-polls yet.** `GetDownloadStatus`/`RemoveDownload` exist as
manually-triggered RPCs alongside `SubmitDownload` from the start,
mirroring [0024](../adr/0024-pipeline-core.md)'s Organizer precedent: a
capability being manually-triggered-only today never blocks the RPC
that exposes it from existing before the automatic path does.

## Proto package

`proto/purser/acquisition/v1/` is a new top-level package, not nested
under `music/v1` or a shared `domain/v1` — this is content-type-agnostic
pipeline-core work per [0024](../adr/0024-pipeline-core.md), the same
category as a future `pipeline/v1` package for scan/identify/organize
RPCs, not a module-specific concern the way `afterdark/v1` is.

## Config

Three new `internal/config` components, one per adapter, following
[0010](../adr/0010-configuration.md)'s one-struct-per-component
convention: `Prowlarr{Enabled, BaseURL, APIKey}`,
`QBittorrent{Enabled, BaseURL, Username, Password}`, and
`SABnzbd{Enabled, BaseURL, APIKey}`. QBittorrent's shape differs from
the other two because its WebUI API has no static API key at all —
authentication is username/password traded for a session cookie
(`POST /api/v2/auth/login`), unlike Prowlarr's and SABnzbd's static
API-key header/query-param auth. See
[acquisition-indexer-search.md](acquisition-indexer-search.md) and
[acquisition-download-client.md](acquisition-download-client.md) for
the adapter-level detail behind each.
