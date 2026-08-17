# Music (and People) Web UI

Technical follow-through on the
[People &amp; Music storyboard](https://claude.ai/code/artifact/45b1e50f-cb8e-4194-b6a1-e09f53da6754) —
maps each mocked screen to the RPCs that already exist
([0011](../adr/0011-api-design.md), [0021](../adr/0021-music-domain-model.md))
and the client-side call sequence that composes them. The backend audit
behind this doc found the domain/CRUD/browse layer for both modules
essentially complete already (`PersonService`, `LibraryEntryService`,
`GroupService`, `ItemService`, `MediaFileService`, `MusicReleaseService`,
`EntryPersonService`, `ItemPersonService`, `TagAssignmentService`,
`MusicBrainzService`/`TheAudioDBService`/`FanartTVService`) — this doc is
almost entirely UI-side wiring, plus the two artwork RPCs from
[image-caching-and-serving.md](image-caching-and-serving.md).

## Scope

**In scope:** screen inventory, the shared component vocabulary additions
the storyboard implies, the real domain fields behind each mocked visual
state, and the per-screen RPC sequence.

**Explicitly out of scope:** the in-browser player (stretch goal — gets
its own doc if/when it's picked up); acquisition-pipeline internals
(already [0024](../adr/0024-pipeline-core.md)/[0025](../adr/0025-music-identification-confidence-scoring.md)-owned,
this doc only consumes `LibraryEntry.Monitored`/`MonitorMode` and
`Item.Status` as read/write fields, it doesn't design the pipeline that
acts on them); visual design (settled by the storyboard artifact, not
restated here).

## Mockup states → real fields

| Storyboard visual | Real field |
|---|---|
| Artist "Monitored · All" toggle | `LibraryEntry.Monitored` + `MonitorMode` (`ALL`/`FUTURE`/`NONE`/`LATEST`), via `UpdateLibraryEntry` field-mask |
| Album "Owned" / "Partial" / "Wanted" badge | Not a `Group` field — `Group` has no status of its own. Derived from its `MusicRelease`(s): the default edition's (`IsDefault=true`) `Status` (`stub`\|`partial`\|`imported`), see "Discography ownership" below |
| Edition format/label/badge | `MusicRelease` fields directly — `Format`, `Label`, `CatalogNumber`, `Country`, `Status`, `Monitored` |
| Track status chip | `Item.Status` (`wanted`\|`grabbed`\|`downloading`\|`imported`\|`missing`\|`skipped`) |
| Genre/mood chips | `TagAssignment` rows on the `Group`, `Scope=metadata`, resolved through `TagService` for display names |
| Member era ("Former · 1970–1989") | `EntryPerson.StartDate`/`EndDate` — already real fields on `domain.EntryPerson`, no metadata bag needed. Populated at Add Artist time from a `Type=="Group"` candidate's MusicBrainz "member of band" relations (`MusicBrainzService.GetArtist(mbid).members[].begin`/`end`), one `EntryPerson` created per member — see `useAddArtist.ts` |
| Bio panel | `TheAudioDBService.LookupArtist(mbid).Biography` — provider passthrough, never persisted server-side per [0027](../adr/0027-provider-independence.md) |
| Backdrop/logo art | `FanartTVService.LookupArtist(mbid)` for `artist_background`/`hd_music_logo` at browse time (Hero's `backdrop`), `artist_thumb` for the grid/sidebar `poster` — **committing** either goes through `image-caching-and-serving.md`'s two-call sequence |
| ISNI / official site / Wikipedia link | `MusicBrainzService.GetArtist(mbid)`'s `isnis`/`official_url`/`wikipedia_url` (relations data, deliberately absent from `SearchArtists`' `MusicBrainzArtist` — see that message's own doc comment) — a structured identity fact, written once into `LibraryEntry.Metadata` at Add Artist time same as `artist_type`/`aliases`, not re-fetched live like the bio/backdrop above |

## Screens

### People (`/people`)

- **Index** — `PersonService.ListPeople`. Card: `PersonCard` (existing
  vocabulary entry), photo from `GET /media/images/{id}` where the `Image`
  row's `owner_type="person"`.
- **Detail** — `PersonService.GetPerson`. "Appears as" list is two calls,
  not a new endpoint: `EntryPersonService.ListEntryPeople(person_id)` for
  artist/band membership, `ItemPersonService.ListItemPeople(person_id)`
  for per-track credits (songwriter, featured artist). Each row's
  `library_entry_id`/`item_id` resolves to a link via the already-fetched
  `LibraryEntry`/`Item`.

### Music Library (`/music`)

`LibraryEntryService.ListLibraryEntries(kind="artist")`, paginated.

**Discography ownership** (the ring/badge) is a bounded client-side
fan-out, not a new aggregate RPC — consistent with the UI being the
composition path rather than the server pre-computing views for it. For
each artist card on one page (page size ~24–48, not the whole library):
`GroupService.ListGroups(library_entry_id)`, then
`MusicReleaseService.ListMusicReleases(group_id)` per group to read each
default edition's `Status`. This is a real cost worth watching — flagged
here rather than solved by inventing a new "artist summary" endpoint. If
it's slow in practice once real data exists, the fix is revisited then
(a cached/materialized field, most likely), not designed speculatively now.

### Artist Detail (`/music/artists/{id}`)

- `LibraryEntryService.GetLibraryEntry`.
- Bio/art: `TheAudioDBService.LookupArtist(mbid)` +
  `FanartTVService.LookupArtist(mbid)`, called with the artist's stored
  MBID (`ExternalIDRepository`, source `mbz`).
- **Discography tab** — `GroupService.ListGroups(library_entry_id)`, then
  per-group `MusicReleaseService.ListMusicReleases(group_id)` for the
  ownership badge (same fan-out as the Library grid, bounded here to one
  artist's albums instead of a page of artists — much cheaper).
- **Members tab** — `EntryPersonService.ListEntryPeople(library_entry_id)`,
  each row resolved against `PersonService.GetPerson` (or a batched
  `ListPeople` if/when one exists — not assumed here).
- Monitor toggle — `LibraryEntryService.UpdateLibraryEntry` with
  `update_mask=[monitored, monitor_mode]`.

### Release Group / Album Detail (`/music/albums/{id}`)

- `GroupService.GetGroup`.
- Genre/mood chips — `TagAssignmentService.ListTagAssignments(owner_type="group", owner_id)`
  + `TagService.GetTag` per assignment (or batched, story-time detail).
- **Editions strip** — `MusicReleaseService.ListMusicReleases(group_id)`.
  Each edition's own `Monitored` toggles independently via
  `MusicReleaseService.UpdateMusicRelease`.
- **Tracklist** (selected edition) —
  `MusicReleaseService.ListMusicReleaseTracks(release_id)`, returns
  `Item`s directly. Per-track status chip is `Item.Status` as returned;
  the play icon only renders when `Item.Status=imported` and a
  `MediaFileService.ListMediaFiles(item_id)` lookup resolves a file (the
  player, when built, streams from there — out of scope here).

### Wanted board (`/music/wanted`)

Cross-artist, not a new composing service — same "one Item/Group's status
already carries this" data, just filtered and listed. Exact filter shape
(likely `ItemService.ListItems` filtered by status, plus
`GroupService`/`MusicReleaseService` reads for the monitored-but-incomplete
albums that have no wanted track yet) gets settled at story time against
what `ItemFilter` actually supports today — not asserted here as already
built.

## Component vocabulary additions

Per [the style guide](../design/style-guide.md#component-vocabulary)'s own
"extend it here as real screens get built" rule:

- **`ArtistCard`/`AlbumCard`** — the storyboard's `Card` variants, config-
  driven per the existing rule (no Music-specific fork of `Card`).
- **`OwnershipRing`** — the fractional-progress ring, generic (not Music-
  only — any "N of M owned" surface can reuse it).
- **`EditionStrip`** — horizontal scroller of edition cards with one
  "selected" state driving the tracklist below it.
- **`ImageLightbox`** — added by
  [image-caching-and-serving.md](image-caching-and-serving.md), not
  repeated here.

## Consequences

- No new domain RPCs beyond the two artwork ones — this module's read/
  browse/write surface was already fully built before this doc existed.
- The Library grid's ownership ring is an accepted, bounded per-page
  fan-out, explicitly not solved by a new aggregate endpoint — a
  documented trade-off, not an oversight, per the "UI composes, server
  doesn't" rule this doc follows throughout.
- Member-era metadata shape (`EntryPerson`'s role bag) is left open for
  story time rather than guessed here.

## Self-Audit Checklist

1. Does any story that comes out of this doc propose a new server-side
   endpoint that composes multiple existing RPCs into one call (an
   "artist summary" endpoint, a "get everything for this album page"
   endpoint)? If yes — stop and check it against this doc's fan-out
   sections first; that composition belongs in the client.
2. Does any screen's status badge get invented as a new field instead of
   being read from the real field named in "Mockup states → real fields"?
   If yes — fix it.
3. Does anything persist TheAudioDB's biography or fanart.tv's image URLs
   server-side instead of treating them as pass-through, fetched fresh, per
   [0027](../adr/0027-provider-independence.md)? If yes — fix it (committing
   a chosen image via `image-caching-and-serving.md`'s flow is the one
   exception — that's a deliberate user/system action, not caching a
   lookup response).
