# 0027. Provider Independence: One RPC per Provider, No Server-Side Merge

Status: Proposed

## Context

Planning AfterDark surfaced a question Music never had to answer explicitly:
what happens when **two** external providers cover the same ground?
StashDB and ThePornDB are direct competitors — both cover performers and
scenes, both are crowd-sourced, neither is authoritative over the other.
Music's providers never forced this question because MusicBrainz has no
competitor (it's the only source for artist/release identity), so
[0001](0001-hexagonal-architecture.md) only ever had to carve out one
narrow exception (`MusicBrainzClient`) for a provider with no rival.

A pre-reset version of this codebase did face this, for images: TheAudioDB
and fanart.tv both supplied artist/album art, and the server picked a
winner itself via a hardcoded `ImagePriority()` ranking (TheAudioDB beat
fanart.tv). That pattern has not been rebuilt in the current codebase, and
this ADR deliberately decides not to rebuild it.

[0011](0011-api-design.md) already anticipated the shape of the fix,
without building it: a write RPC "does not reach out to a metadata
provider... If a caller needs metadata to build a complete `Person`,
that's a separate call to a separate (future) metadata-provider RPC — the
caller assembles the entity, the write RPC only persists what it's given."
`MusicBrainzService` ([internal/api/connect/musicbrainz_search.go](../../internal/api/connect/musicbrainz_search.go))
already builds that RPC for MusicBrainz today: a thin, read-only
passthrough that answers a query and persists nothing, leaving the actual
decision to a separate `AcceptCandidate` call made after a human (or CLI
logic) chooses. This ADR generalizes that shape into a rule for every
provider, including the AfterDark case of two competing providers, and
formally retires the old priority-ranking pattern as something this
codebase will build again.

## Decision

Every external provider — metadata or image, one competitor or several —
gets:

1. **Its own port and its own adapter.** No shared interface that more
   than one provider implements interchangeably. `StashDBClient` and
   `TheAudioDBClient` are separate ports, not two implementations of a
   `MetadataSource`/`ImageSource` interface.
2. **Its own Connect service**, shaped around what that provider actually
   supports (search by fragment, lookup by hash, lookup by JAV code,
   whatever is real for that provider) — not a generic
   `SearchMetadata(provider_name, query)` RPC.
3. **Read-only, passthrough behavior only.** The RPC asks the provider a
   question and returns that provider's own answer, translated into a
   proto message shaped like that provider's data. It never calls another
   provider, never ranks or merges results, and never writes anything.
4. **No opinion on which provider is "better."** The server does not pick
   a winner between StashDB and ThePornDB, or between any two providers,
   ever. There is no `Priority()`/rank concept on any provider port.

The caller — the Web UI or the CLI — is the only place multi-provider
behavior happens: call each provider's RPC, present or compare the
results, decide what to use, then call the existing entity `Create*`/
`Update*` RPCs to persist the assembled result. This is unchanged from
[0011](0011-api-design.md)'s existing write-path rule; this ADR only adds
the read side.

This applies the same way regardless of how many real competitors a
provider has. MusicBrainz has one implementation and needs no merge logic
because there is nothing to merge against — it already follows this shape
today (`MusicBrainzService`), unchanged by this ADR. StashDB/ThePornDB
have two, so the UI/CLI is where a human (or logic) picks between them.
Music's future image providers (TheAudioDB, fanart.tv) follow the same
shape if/when they're rebuilt — no priority ranking baked into the server.

**Not in scope:** [0024](0024-pipeline-core.md)/[0025](0025-music-identification-confidence-scoring.md)'s
automatic scan-time identification (grouping → candidate generation →
confidence scoring) is a different, already-decided flow — an unattended
scan legitimately uses provider data server-side to produce ranked
`MatchCandidate`s, with a human still confirming via `AcceptCandidate`
before anything is persisted. This ADR governs the *interactive, manual*
lookup surface (a human or CLI explicitly asking "what does StashDB say
about this"), not the automatic pipeline.

**Not resolved here:** how a chosen image URL's bytes actually get
fetched and written into `ImageStore` ([0013](0013-image-blob-storage.md))
stays a separate, still-unbuilt piece — that ADR already flagged a
"remote-image fetcher" as future work. This ADR only fixes what feeds
into it: a URL the client picked, never one the server picked for them.

## Consequences

- Adding a new provider (a third AfterDark source, TMDb, TVDb, whatever)
  is purely additive: new port, new adapter, new Connect service. No
  existing provider's code or any entity CRUD service changes.
- No provider port ever grows a rank/priority field again — that decision
  moved to the client permanently.
- More RPCs to maintain long-term (one per provider, not one shared
  "search everything" call) — accepted, since it keeps providers
  independently testable and swappable per
  [0001](0001-hexagonal-architecture.md).
- The UI and CLI now own orchestration work (calling several endpoints,
  presenting/comparing results) that a server-side auto-merge would have
  hidden from them. Deliberate: the server should not decide.
- k6 coverage grows one endpoint suite per provider service, same
  convention [0011](0011-api-design.md) already uses for every Connect
  service.

## Self-Audit Checklist

1. Does a new provider adapter implement a shared interface that another
   provider adapter also implements, instead of getting its own port? If
   yes — fix it; every provider is its own port.
2. Does any service or handler compare, rank, merge, or auto-pick between
   two providers' results before returning to the caller? If yes — fix
   it; that decision belongs to the client, never the server.
3. Does a provider lookup RPC persist anything, or accept a caller-supplied
   ID meant to be saved? If yes — fix it; these RPCs are read-only,
   exactly like `MusicBrainzService` today.
4. Is a new provider's RPC shaped as a generic
   `SearchMetadata(provider_name, query)` instead of exposing that
   provider's own real capabilities as distinct methods? If yes — fix it.
5. Does scan-time automatic identification ([0025](0025-music-identification-confidence-scoring.md))
   get rerouted through one of these manual lookup RPCs instead of its
   own internal port usage, or vice versa? If yes — that's scope creep
   this ADR didn't intend; flag it.
