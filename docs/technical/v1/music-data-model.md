# Music Data Model (v1 snapshot)

> Snapshot of `docs/technical/music_data_model.md` as of commit `5fcd657`,
> before the reset. See [README.md](README.md) for how to use this document.
> Cross-references the general schema in [data-model.md](data-model.md).

> This document specifies every entity used by the music module. Field
> definitions use a neutral notation — no SQL syntax — so both the BadgerDB
> and SQL adapters could implement from the same spec. See
> [metadata-providers.md](metadata-providers.md) for the pipeline/provider
> side that populated these entities.

---

## Guiding Constraints

Three rules shaped every decision in this document:

1. **No new fields on shared structs for music-specific data.** `Item`, `MediaFile`, and `LibraryEntry` are shared across all content types. Music-specific data went in their existing `Metadata` bags. This kept the struct contract stable for all adapters.
2. **Reuse existing shared entities.** `Person` (with its `Aliases` field), `Tag`, `ExternalID`, `EntryPerson`, `ItemPerson` — all were used by music without modification.
3. **New entities are first-class domain types with their own ports.** `MusicRelease` and `MusicScanGroup` were not bolted onto existing types; they got their own repository interfaces so adapters implemented them cleanly.

---

## What Was Reused vs. What Was New

| Concept | Purser entity | Change |
|---|---|---|
| Artist | `LibraryEntry` (Kind=KindArtist) | None to the struct; aliases stored in `Metadata["aliases"]` |
| Band members / solo artist link | `EntryPerson` + `Person` | None |
| Release Group | `Group` | None to the struct; type stored in `Metadata["album_type"]` |
| Release | `MusicRelease` | **New entity** |
| Track | `Item` | None to the struct; music fields stored in `Metadata` keys |
| Audio file | `MediaFile` | Added `Metadata map[string]string` (no other new fields) |
| Genres/styles | `Tag` | None |
| External IDs | `ExternalID` | None |
| Import queue entry | `MusicScanGroup` | **New type** (lived in `domain/scan.go` alongside `UnmatchedFile`) |
| Confidence signals | `MusicConfidenceSignals` | **New type** |

---

## Entity: Artist

Stored as a `LibraryEntry` with `Kind = KindArtist`. No structural change to `LibraryEntry`.

### Artist Metadata keys

Music-specific artist data was stored in `LibraryEntry.Metadata map[string]any` under these keys.

| Key | Type | Description |
|---|---|---|
| `artist_type` | string | `person` \| `group` \| `orchestra` \| `choir` \| `other` — from MBZ |
| `aliases` | []string | All known alternate names from MBZ. Stored in the metadata bag; no dedicated struct field. Indexed by the adapter for alias lookup. |
| `founded_date` | string | ISO 8601 |
| `founded_location` | string | City/region |
| `dissolved_date` | string | ISO 8601; empty if still active |
| `isni` | string | International Standard Name Identifier |
| `official_url` | string | Official homepage |
| `lastfm_url` | string | Last.fm artist page — captured from an MBZ `url-rels` relation, not from a Last.fm API call. There was no dedicated Last.fm adapter. |
| `wikipedia_url` | string | From MBZ relationships |

For solo artists (MBZ type `Person`), the equivalent keys were `born_date`, `born_location`, `died_date` instead of the founded/dissolved set.

### Why aliases lived in Metadata, not a new struct field

`Person` already had `Aliases []string` because people have aliases across all content types. A parallel `Aliases []string` was not added to `LibraryEntry` — that would have been a new snowflaked field with its own persistence mechanism. Instead, artist aliases were stored in the existing `Metadata` bag.

### Members and Solo Artist Handling

**Band:** Members were `EntryPerson` records, each pointing to a `Person`. Roles: `member` | `former_member` | `vocalist` | `guitarist` | `bassist` | `drummer` | `keyboardist` | `producer`. Unknown role → `member`.

**Solo artist:** MusicBrainz types a solo act's artist record as `Person` — the artist record *is* the person. The adapter linked the artist to itself via `FetchEntryPeople`, so `PersonDetail` shows the solo entry under "Member of" and the artist page shows the person in its members list, with no branch in shared code.

The distinction between a band and a solo artist was entirely in `artist_type` metadata and the presence/absence of additional member `EntryPerson` records.

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz` | MBZ Artist MBID |
| `audiodb` | TheAudioDB artist ID (same as the MBID — TheAudioDB is looked up by MBID) |

---

## Entity: Release Group

Stored as a `Group`. No structural change to `Group`.

The Release Group is the logical work — the conceptual album independent of its pressings. In the UI it was labelled "Album."

### Release Group Metadata keys

| Key | Type | Description |
|---|---|---|
| `album_type` | string | `studio` \| `live` \| `compilation` \| `ep` \| `single` \| `other`. Derived from MBZ `primary-type` and `secondary-types`. |

### Monitoring

Monitoring was set at the Release Group level. By default, monitoring a Release Group activated only the default Release (the one `MusicRelease.IsDefault = true`). The user explicitly opted additional Releases in. `MonitorMode` on the Group drove what happened when new Releases were discovered during a metadata refresh.

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz` | MBZ Release Group MBID |

---

## Entity: Release (MusicRelease)

A specific pressing or edition of a Release Group. The thing on disk. First-class new domain entity, no prior equivalent.

### Fields

| Field | Type | Notes |
|---|---|---|
| ID | string (UUID) | |
| GroupID | string | → Release Group (Group.ID) |
| LibraryEntryID | string | → Artist (LibraryEntry.ID). Denormalized for efficient "all releases by artist" queries. |
| Title | string | Edition-qualified title, e.g. "Hi Infidelity (2024 Remaster)" |
| Country | string | ISO 3166-1 alpha-2; empty if not country-specific |
| Date | time.Time | Release date; year-only dates stored as Jan 1 of that year |
| Label | string | Primary label name |
| CatalogNumber | string | Label catalog number |
| Barcode | string | EAN/UPC as a string, no formatting |
| Format | string | `CD` \| `Digital Media` \| `Vinyl` \| `Cassette` \| `SACD` \| etc. |
| MediumCount | int | Number of discs/sides |
| TrackCount | int | Total tracks across all media |
| IsDefault | bool | MBZ canonical release for this Release Group — earliest `Official` release, ties broken by MBZ result order |
| Monitored | bool | Whether to acquire this specific Release |
| Status | ReleaseStatus | `stub` \| `partial` \| `imported` |
| ExternalIDs | []ExternalID | MBZ Release MBID under source `mbz` |
| CoverPath | string | Local path to cover art for this edition |
| AddedAt | time.Time | |
| UpdatedAt | time.Time | |

`stub` = known from MBZ but no files on disk. `partial` = some tracks present. `imported` = all tracks present. When a Release Group was first created, stubs were created for all known Releases from MBZ.

### Storage notes

**BadgerDB key:** `mrel:{id}`

Secondary indexes:
- `mrel:grp:{group_id}:{id}` — all releases for a Release Group
- `mrel:entry:{library_entry_id}:{id}` — all releases for an artist
- `mrel:mbid:{mbid}` → `{id}` — MBZ MBID lookup
- `mrel:barcode:{barcode}` → `{id}` — barcode lookup

**SQL:** table `music_releases` (distinct from the acquisition-pipeline `releases` table in [data-model.md](data-model.md#releases--indexer-search-results)).

---

## Entity: Track

Stored as an `Item` with `ContentType = music`. No new struct fields.

### Track Metadata keys

| Key | Type | Description |
|---|---|---|
| `release_id` | string | ID of the `MusicRelease` this track belongs to. Secondary-indexed for "all tracks on a release" queries. |
| `disc_number` | int | Which disc/medium this track is on. 1 for single-disc. |
| `isrc` | string | International Standard Recording Code from embedded file tags |
| `composer` | string | Composer credit from tags |
| `lyricist` | string | Lyricist credit from tags |

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz_recording` | MBZ Recording MBID — a distinct `ExternalIDSource` constant from the artist/release-level `mbz` source, so recording-level IDs don't collide with entry/group-level ones. `entity_type = "item"`. |

### Track people

Featured/guest artist credits used the existing `ItemPerson` mechanism. Valid roles for music: `artist` | `featured_artist` | `producer` | `songwriter`.

### Existing Item fields used for tracks

`Sequence` = track number within the disc (MBZ track position, e.g. `"1"` or `"A1"` for vinyl — kept as a string, not coerced to int). `RuntimeSeconds` = duration. `GroupID` → Release Group. `Date` = original recording date when known.

---

## Entity: Media File

`MediaFile` gained two fields for general and music-specific use:

| Field | Type | Notes |
|---|---|---|
| `SHA1` | string | SHA-1 hex digest of the file. First-class field — applies to all content types, alongside the existing `OSHash` and `MD5`. |
| `Metadata` | map[string]string | Content-type-specific file attributes. Nil for non-music files. |

### MediaFile Metadata keys for music

| Key | Description |
|---|---|
| `acoustid` | AcoustID acoustic fingerprint. Computed asynchronously after import; absent until the background job runs, and submitted to the AcoustID service if no existing submission matched. |

---

## Import Queue Types

These types lived in `internal/domain/scan.go` alongside the existing `UnmatchedFile`.

### MusicScanGroup

One queue entry per folder. Non-music content continued to use `UnmatchedFile`. No branch in the scan service — the music `FileIdentifier` adapter handled grouping and wrote `MusicScanGroup` records itself; the scan service fanned out to all registered `FileIdentifier` implementations uniformly.

| Field | Type | Notes |
|---|---|---|
| ID | string (UUID) | |
| FolderPath | string | Source folder path |
| Files | []ScannedFile | Audio files in this group |
| TotalTracks | int | |
| TotalDiscs | int | Number of disc sub-folders detected; 1 if none |
| Tags | MusicTagSummary | Consensus tag values from all files |
| Candidates | []MusicReleaseCandidate | Ranked by OverallConfidence desc |
| Status | UnmatchedStatus | Reused existing `pending` \| `matched` \| `dismissed` |
| DiscoveredAt | time.Time | |

**BadgerDB key:** `msg:{id}`, secondary index `msg:status:{status}:{id}`. **SQL:** table `music_scan_groups`.

### MusicTagSummary

Consensus tags extracted from the files in a scan group. Where tags disagreed across files, the majority value won.

| Field | Type | Notes |
|---|---|---|
| AlbumArtist | string | Consensus ALBUMARTIST tag |
| AlbumTitle | string | Consensus ALBUM tag |
| Year | int | 0 if absent |
| Barcode | string | UPC/EAN from tags; empty if absent |
| Label | string | |
| CatalogNumber | string | |
| TotalTracks | int | From TRACKTOTAL tag; falls back to file count |
| TotalDiscs | int | From DISCTOTAL tag; falls back to sub-folder count |
| MBZReleaseID | string | From MUSICBRAINZ_ALBUMID tag; triggered a re-scan shortcut when present |
| TrackTitles | []string | Ordered by track number |
| TrackDurations | []time.Duration | Read from audio bitstream, not from tags; ordered by track number |
| ISRCs | []string | One per track; empty string where absent |

### MusicReleaseCandidate

One candidate produced by the identification pipeline.

| Field | Type | Notes |
|---|---|---|
| ArtistMBID / ArtistName | string | |
| ReleaseGroupMBID / ReleaseGroupTitle / ReleaseGroupType | string | |
| ReleaseMBID | string | Empty if Release not resolved |
| ReleaseTitle / ReleaseDate / ReleaseLabel / ReleaseCountry / ReleaseBarcode / ReleaseFormat | string | |
| ReleaseMediumCount / ReleaseTrackCount | int | |
| OverallConfidence | float64 | Weighted sum of signal scores; 0.0–1.0 |
| Signals | MusicConfidenceSignals | Per-signal breakdown shown in the queue UI |

### MusicConfidenceSignals

All values 0.0–1.0. Zero means the signal was unavailable or did not fire.

| Field | Signal | Weight |
|---|---|---|
| Barcode | UPC/EAN tag matched a MBZ Release barcode | 1.0 |
| ISRC | Track ISRCs reached consensus on a single RG | 0.95 |
| RGNameFuzzy | Album tag (suffixes stripped) fuzzy-matched a Release Group name | 0.40–0.60 |
| TrackCount | Total track count matched the MBZ Release | 0.20 |
| TrackTitleSet | All track titles matched the MBZ Release tracklist | 0.25 |
| Duration | Fraction of tracks within ±2s of MBZ durations | 0.30 |
| AcoustID | Fraction of tracks whose fingerprint matched | 0.35 |

---

## Ports That Existed for This Model

### MusicReleaseRepository

```
Get(ctx, id) (*MusicRelease, error)
GetByMBID(ctx, mbid) (*MusicRelease, error)
GetByBarcode(ctx, barcode) (*MusicRelease, error)
ListByGroup(ctx, groupID) ([]*MusicRelease, error)
ListByEntry(ctx, entryID) ([]*MusicRelease, error)
ListTracksByRelease(ctx, releaseID) ([]*domain.Item, error)
Save(ctx, r *MusicRelease) error
Delete(ctx, id) error
```

`ListTracksByRelease` lived on this repository, not on `ItemRepository`, to avoid adding a music-specific `ReleaseID` filter to the shared `ItemFilter`. The music release repository owned the secondary index on `Item.Metadata["release_id"]`.

### MusicScanGroupRepository

```
Get(ctx, id) (*MusicScanGroup, error)
List(ctx, status UnmatchedStatus) ([]*MusicScanGroup, error)
Save(ctx, g *MusicScanGroup) error
Delete(ctx, id) error
```

### ReleaseGroupContentSource (optional MetadataSource capability)

```
type ReleaseGroupContentSource interface {
    FetchReleaseGroupReleases(ctx, rgMBID string) ([]*ExternalMusicRelease, error)
}
```

Checked via type assertion in the aggregator. `ExternalMusicRelease` was a data-transfer type carrying the edition-level fields defined in `MusicRelease` above. Only the MusicBrainz adapter implemented it — see
[metadata-providers.md](metadata-providers.md#musicbrainz) for the actual endpoint. Every other adapter simply didn't implement the interface (Interface Segregation — no `ErrNotSupported` stub needed for a capability an adapter never claimed).

### One open design question noted at the time

`ItemFilter` in `ports/repository.go` had no generic metadata filter (`MetadataKey`/`MetadataValue`). `ListTracksByRelease` worked around this for the one query that needed it. If other music queries needed to filter Items by Metadata keys, the options considered were: add a generic `MetadataKey`/`MetadataValue` pair to `ItemFilter` (works for all content types), or give each case its own repository method. This was left unresolved — worth deciding early if the rebuild reaches this point again, since it's exactly the kind of narrow-vs-generic-interface tradeoff [0002 (ISP)](../adr/0002-solid-design-principles.md) is meant to catch.
