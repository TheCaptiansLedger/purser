# Music Data Model

> This document specifies every entity required for the music module. Field definitions use a neutral notation — no SQL syntax — so both the BadgerDB and SQL adapters implement from the same spec.
>
> See `docs/technical/music_organization.md` for the pipeline and identification logic that populates these entities.

---

## Guiding Constraints

Before the entity definitions, three rules that shape every decision in this document:

1. **No new fields on shared structs for music-specific data.** `Item`, `MediaFile`, and `LibraryEntry` are shared across all content types. Music-specific data goes in their existing `Metadata` bags. This keeps the struct contract stable for all adapters.
2. **Reuse existing shared entities.** `Person` (with its `Aliases` field), `Tag`, `ExternalID`, `EntryPerson`, `ItemPerson` — all are used by music without modification.
3. **New entities are first-class domain types with their own ports.** `MusicRelease` and `MusicScanGroup` are not bolted onto existing types; they get their own repository interfaces so adapters implement them cleanly.

---

## What We Reuse vs. What Is New

| Concept | Purser entity | Change |
|---|---|---|
| Artist | `LibraryEntry` (Kind=KindArtist) | None to the struct; aliases stored in `Metadata["aliases"]` |
| Band members / solo artist link | `EntryPerson` + `Person` | None |
| Release Group | `Group` | None to the struct; type stored in `Metadata["album_type"]` |
| Release | `MusicRelease` | **New entity** |
| Track | `Item` | None to the struct; music fields stored in `Metadata` keys |
| Audio file | `MediaFile` | Add `Metadata map[string]string` (no other new fields) |
| Genres/styles | `Tag` | None |
| External IDs | `ExternalID` | None |
| Import queue entry | `MusicScanGroup` | **New type** (lives in `domain/scan.go` alongside `UnmatchedFile`) |
| Confidence signals | `MusicConfidenceSignals` | **New type** |

---

## Entity: Artist

Stored as a `LibraryEntry` with `Kind = KindArtist`. No structural change to `LibraryEntry`.

### Artist Metadata keys

Music-specific artist data is stored in `LibraryEntry.Metadata map[string]any` under these keys.

| Key | Type | Description |
|---|---|---|
| `artist_type` | string | `person` \| `group` \| `orchestra` \| `choir` \| `other` — from MBZ |
| `aliases` | []string | All known alternate names from MBZ. Stored in the metadata bag; no new struct field. Indexed by the adapter for alias lookup. |
| `founded_date` | string | ISO 8601 |
| `founded_location` | string | City/region |
| `dissolved_date` | string | ISO 8601; empty if still active |
| `isni` | string | International Standard Name Identifier |
| `official_url` | string | Official homepage |
| `lastfm_url` | string | Last.fm artist page |
| `wikipedia_url` | string | From MBZ relationships |

### Why aliases live in Metadata, not a new struct field

`Person` already has `Aliases []string` because people have aliases across all content types. We do not add a parallel `Aliases []string` to `LibraryEntry` — that would be a new snowflaked field with its own persistence mechanism. Instead, artist aliases are stored in the existing `Metadata` bag. The music-specific adapters know to create a secondary index on this key for lookup efficiency.

### Members and Solo Artist Handling

**Band:** Members are `EntryPerson` records, each pointing to a `Person`. Roles: `member` | `former_member` | `vocalist` | `guitarist` | `bassist` | `drummer` | `keyboardist` | `producer`. Unknown role → `member`.

**Solo artist:** Create a `Person` record for the human (with `Person.Aliases`, birthdate, birthplace etc.). Create the `LibraryEntry` (Artist). Link them via an `EntryPerson` with role `member`. The `Person.Aliases` field handles the solo artist's aliases — no duplication needed.

The distinction between a band and a solo artist is entirely in `artist_type` metadata and the presence/absence of additional member `EntryPerson` records. No branch in shared code.

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz` | MBZ Artist MBID |
| `audiodb` | TheAudioDB artist ID |
| `lastfm` | Last.fm artist name key |

---

## Entity: Release Group

Stored as a `Group`. No structural change to `Group`.

The Release Group is the logical work — the conceptual album independent of its pressings. In the UI it is labelled "Album."

### Release Group Metadata keys

| Key | Type | Description |
|---|---|---|
| `album_type` | string | `studio` \| `live` \| `compilation` \| `ep` \| `single` \| `other`. Drives the discography chip in the UI. Derived from MBZ `primary-type` and `secondary-types` via the existing `AlbumFilterToken()` method on `ExternalGroup`. |

This key is already threaded through the existing `AlbumFilterToken()` logic and is not new.

### Monitoring

Monitoring is set at the Release Group level. By default, monitoring a Release Group activates only the default Release (the one `MusicRelease.IsDefault = true`). The user explicitly opts additional Releases in. `MonitorMode` on the Group drives what happens when new Releases are discovered during a metadata refresh.

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz` | MBZ Release Group MBID |

---

## Entity: Release (MusicRelease)

A specific pressing or edition of a Release Group. The thing on disk.

New first-class domain entity. No existing equivalent.

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
| IsDefault | bool | MBZ canonical release for this Release Group |
| Monitored | bool | Whether to acquire this specific Release |
| Status | ReleaseStatus | `stub` \| `partial` \| `imported` |
| ExternalIDs | []ExternalID | MBZ Release MBID under source `mbz` |
| CoverPath | string | Local path to cover art for this edition |
| AddedAt | time.Time | |
| UpdatedAt | time.Time | |

`stub` = known from MBZ but no files on disk. `partial` = some tracks present. `imported` = all tracks present.

When a Release Group is first created, stubs are created for all known Releases from MBZ. This is what lets the user see which editions they have and which they don't.

### Storage notes

**BadgerDB key:** `mrel:{id}`

Secondary indexes:
- `mrel:grp:{group_id}:{id}` — all releases for a Release Group
- `mrel:entry:{library_entry_id}:{id}` — all releases for an artist
- `mrel:mbid:{mbid}` → `{id}` — MBZ MBID lookup
- `mrel:barcode:{barcode}` �� `{id}` — barcode lookup

**SQL:** New table `music_releases`.

---

## Entity: Track

Stored as an `Item` with `ContentType = music`. No new struct fields.

Music-specific data is stored in `Item.Metadata map[string]any` under these keys.

### Track Metadata keys

| Key | Type | Description |
|---|---|---|
| `release_id` | string | ID of the `MusicRelease` this track belongs to. The adapter maintains a secondary index on this key for efficient "all tracks on a release" queries. |
| `disc_number` | int | Which disc/medium this track is on. 1 for single-disc. |
| `isrc` | string | International Standard Recording Code from embedded file tags |
| `composer` | string | Composer credit from tags |
| `lyricist` | string | Lyricist credit from tags |

### ExternalIDs in use

| Source | Value |
|---|---|
| `mbz_recording` | MBZ Recording MBID |

This uses a new `ExternalIDSource` constant `mbz_recording` to distinguish recording-level MBZ IDs from artist/release-level MBZ IDs. The `external_ids` table/index entry carries `entity_type = "item"`.

### Track people

Featured/guest artist credits use the existing `ItemPerson` mechanism. Valid roles for music: `artist` | `featured_artist` | `producer` | `songwriter`. These are already declared in `ContentType.ItemPersonRoles()` for `ContentTypeMusic`.

### Existing Item fields used for tracks

`Sequence` = track number within the disc. `RuntimeSeconds` = duration. `GroupID` → Release Group. `Date` = original recording date when known. All existing, no change.

---

## Entity: Media File

`MediaFile` gets one new field: `Metadata map[string]string`.

`SHA1` is a general-purpose file identity hash useful across all content types (not just music), so it is elevated to a first-class field on `MediaFile` alongside the existing `OSHash` and `MD5`. `AcoustID` is music-specific and belongs in the `Metadata` bag.

### New fields

| Field | Type | Notes |
|---|---|---|
| `SHA1` | string | SHA-1 hex digest of the file. First-class field — applies to all content types. |
| `Metadata` | map[string]string | Content-type-specific file attributes. Nil for non-music files. |

### MediaFile Metadata keys for music

| Key | Description |
|---|---|
| `acoustid` | AcoustID acoustic fingerprint. Computed asynchronously after import; absent until the background job runs. |

AcoustID is queued as an async job at import time and written back to this map when complete. It is also submitted to the AcoustID service if no existing submission matches.

---

## Import Queue Types

These types live in `internal/domain/scan.go` alongside the existing `UnmatchedFile`.

### MusicScanGroup

One queue entry per folder. Non-music content continues to use `UnmatchedFile`. No branch in the scan service — the music `FileIdentifier` adapter handles grouping and writes `MusicScanGroup` records itself; the scan service fans out to all registered `FileIdentifier` implementations uniformly.

| Field | Type | Notes |
|---|---|---|
| ID | string (UUID) | |
| FolderPath | string | Source folder path |
| Files | []ScannedFile | Audio files in this group |
| TotalTracks | int | |
| TotalDiscs | int | Number of disc sub-folders detected; 1 if none |
| Tags | MusicTagSummary | Consensus tag values from all files |
| Candidates | []MusicReleaseCandidate | Ranked by OverallConfidence desc |
| Status | UnmatchedStatus | Reuses existing `pending` \| `matched` \| `dismissed` |
| DiscoveredAt | time.Time | |

**BadgerDB key:** `msg:{id}`
Secondary index: `msg:status:{status}:{id}`
**SQL:** New table `music_scan_groups`

---

### MusicTagSummary

Consensus tags extracted from the files in a scan group. Where tags disagree across files, the majority value wins.

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
| MBZReleaseID | string | From MUSICBRAINZ_ALBUMID tag; triggers re-scan shortcut when present |
| TrackTitles | []string | Ordered by track number |
| TrackDurations | []time.Duration | Read from audio bitstream, not from tags; ordered by track number |
| ISRCs | []string | One per track; empty string where absent |

---

### MusicReleaseCandidate

One candidate produced by the identification pipeline.

| Field | Type | Notes |
|---|---|---|
| ArtistMBID | string | |
| ArtistName | string | |
| ReleaseGroupMBID | string | |
| ReleaseGroupTitle | string | |
| ReleaseGroupType | string | |
| ReleaseMBID | string | Empty if Release not resolved |
| ReleaseTitle | string | |
| ReleaseDate | string | ISO 8601 |
| ReleaseLabel | string | |
| ReleaseCountry | string | |
| ReleaseBarcode | string | |
| ReleaseFormat | string | |
| ReleaseMediumCount | int | |
| ReleaseTrackCount | int | |
| OverallConfidence | float64 | Weighted sum of signal scores; 0.0–1.0 |
| Signals | MusicConfidenceSignals | Per-signal breakdown shown in the queue UI |

---

### MusicConfidenceSignals

All values 0.0–1.0. Zero means the signal was unavailable or did not fire. Displayed individually in the import queue so the user can see exactly what matched and why.

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

## New Ports Required

The following new port interfaces are needed. Each must be implemented by all storage adapters (BadgerDB, SQLite, PostgreSQL).

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

Note: `ListTracksByRelease` is on this repository, not on `ItemRepository`. This avoids adding a music-specific `ReleaseID` filter to the shared `ItemFilter`. The music release repository owns the secondary index on `Item.Metadata["release_id"]` and resolves it directly.

### MusicScanGroupRepository

```
Get(ctx, id) (*MusicScanGroup, error)
List(ctx, status UnmatchedStatus) ([]*MusicScanGroup, error)
Save(ctx, g *MusicScanGroup) error
Delete(ctx, id) error
```

### New MetadataSource sub-interface

The MBZ adapter needs one new optional capability, checked via type assertion in the aggregator:

```
type ReleaseGroupContentSource interface {
    FetchReleaseGroupReleases(ctx, rgMBID string) ([]*ExternalMusicRelease, error)
}
```

`ExternalMusicRelease` is a new data-transfer type in the domain (alongside the existing `ExternalGroup`, `ExternalItem`) that carries the edition-level fields defined in `MusicRelease`. No other adapters implement this interface; they return `ErrNotSupported` by default.

---

## Hexagonal and SOLID Check

### Hexagonal (dependency rule)

| Layer | What it touches | Clean? |
|---|---|---|
| `domain` | `MusicRelease`, `MusicScanGroup`, supporting value types — no imports from outside `domain` | ✓ |
| `ports` | New `MusicReleaseRepository`, `MusicScanGroupRepository`, `ReleaseGroupContentSource` interfaces — depend only on `domain` | ✓ |
| `app` | Music import service depends on port interfaces, not concrete adapters | ✓ |
| `adapters/mbz` | Implements `ReleaseGroupContentSource`; returns `ExternalMusicRelease` | ✓ |
| `adapters/badger` and `adapters/db` | Implement `MusicReleaseRepository` and `MusicScanGroupRepository` | ✓ |
| `api` | Music endpoints call app service methods; no direct domain manipulation | ✓ |

No adapter imports another adapter. No domain type imports from ports, app, or adapters.

### SOLID

**Single Responsibility**

- `MusicRelease` — represents one specific pressing. Nothing else.
- `MusicScanGroup` — represents one pending import folder. Nothing else.
- `MusicReleaseRepository` — persistence for `MusicRelease` only.
- `MusicScanGroupRepository` — persistence for `MusicScanGroup` only. Does not extend `UnmatchedFileRepository`.

**Open/Closed**

- Adding music releases extends the system: new domain type, new port, new adapter methods. No existing method signatures on `ItemRepository`, `GroupRepository`, `LibraryEntryRepository`, or `MediaFileRepository` are modified.
- `MediaFile.Metadata map[string]string` is additive — existing code that does not write to it is unaffected.

**Liskov Substitution**

- Both BadgerDB and SQL adapters implement `MusicReleaseRepository` and are substitutable. The app service never queries which backend is active.

**Interface Segregation**

- `MusicReleaseRepository` is its own narrow interface, not merged into `ItemRepository` or `GroupRepository`.
- `MusicScanGroupRepository` is separate from `UnmatchedFileRepository` — they serve different content types with different query patterns.
- `ReleaseGroupContentSource` is a new optional sub-interface on `MetadataSource`, checked via type assertion. Existing adapters are not modified; they simply do not implement it.
- `ItemFilter` is not polluted with a `ReleaseID` field. Track-by-release lookup is owned by `MusicReleaseRepository.ListTracksByRelease`.

**Dependency Inversion**

- The music import app service depends on `MusicReleaseRepository`, `MusicScanGroupRepository`, `GroupRepository`, `LibraryEntryRepository`, `PersonRepository`, `ItemRepository`, `MediaFileRepository` — all interfaces, no concrete types.

### One open question to resolve before implementation

The `ItemFilter` in `ports/repository.go` has no generic metadata filter (`MetadataKey`/`MetadataValue`). The `ListTracksByRelease` method on `MusicReleaseRepository` works around this for the release-to-track query. If other music queries need to filter Items by Metadata keys (e.g., "all tracks with this ISRC"), either a `MetadataKey`/`MetadataValue` pair should be added to `ItemFilter` as a generic mechanism (preferred — works for all content types), or each case gets its own repository method. Resolve this during implementation when the first such query is needed; do not pre-solve it now.
