# v1 — Pre-Reset Reference Snapshot

The documents in this directory are a **frozen historical snapshot** of the
data model and metadata-provider integrations as they existed immediately
before the full code/docs reset performed on 2026-07-11. They are pinned to:

```
commit 5fcd6579adba64f56383726419476cb57b011d7c
```

Every file in this directory can be re-derived or cross-checked against that
commit with `git show 5fcd657:<path>` for as long as that commit remains
reachable in history (it always will — history is never rewritten by the
reset).

## Why this exists

The reset removed all application code and prior docs so the project could
be rebuilt against the new ADRs in [`docs/adr/`](../../adr/0000-index.md)
without carrying forward structural drift. That does not mean the domain
knowledge captured in the old implementation was wrong — the data model and
the provider integration details below were the result of real
investigation (API quirks, rate limits, field mappings) that would otherwise
have to be rediscovered by trial and error. This directory exists so that
knowledge isn't lost.

## How to use these documents

- Treat everything here as **descriptive**, not **prescriptive**. This is
  "what the old system did," not "what the new system must do."
- The [ADRs](../../adr/0000-index.md) are the prescriptive rules for any new
  code — hexagonal boundaries, SOLID checks, testing standards. If something
  in this snapshot conflicts with an ADR, the ADR wins and the conflict is
  worth noting explicitly when it's found, not silently resolved.
- When rebuilding a specific area (e.g. the music import pipeline), read the
  relevant file here first so provider quirks and schema decisions don't
  have to be rediscovered, then design the new port/adapter against
  [0001-hexagonal-architecture.md](../../adr/0001-hexagonal-architecture.md)
  and [0002-solid-design-principles.md](../../adr/0002-solid-design-principles.md).

## Contents

- [`data-model.md`](data-model.md) — the general storage schema shared across
  all content types (library entries, groups, items, people, tags, media
  files, releases, downloads, config tables, monitoring logic).
- [`music-data-model.md`](music-data-model.md) — the music-specific entities
  layered on top of the general schema (Artist, Release Group, Release,
  Track, import-queue types, confidence signals, the ports they required).
- [`metadata-providers.md`](metadata-providers.md) — real endpoints, auth,
  rate limits, and request/response shapes for the three metadata sources
  that were actually integrated for music: MusicBrainz, TheAudioDB, and
  fanart.tv.
