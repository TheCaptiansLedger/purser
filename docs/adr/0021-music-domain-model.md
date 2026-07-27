# 0021. Music Domain Model: Artist/Release Group/Release/Track on the Shared Kernel

Status: Accepted

## Context

The Music module needs a concrete domain model before any API/storage code
can be written. The prior attempt at this (epic
[#376](https://github.com/TheCaptiansLedger/purser/issues/376) and its
sub-issues) predates the `chore!: reset codebase and rebuild architecture
docs from scratch` commit (2026-07-11) and describes types
(`internal/app/scan/`, `docs/technical/music_implementation_tasks.md`) that
no longer exist — it is closed out as stale, not used as a source of truth
here.

Two documents *are* live inputs:

- `docs/technical/music-data_model.md` — a research pass re-verifying
  provider schemas (MusicBrainz, TheAudioDB, fanart.tv, Last.fm, AcoustID)
  against live APIs, explicitly marked "proposal, not yet an ADR-backed
  decision."
- `docs/technical/shared-domain-model.md` — the cross-module synthesis
  defining the shared kernel (`Person`, `Tag`, `ExternalID`, `Image`,
  `Collection`, `Group`, `Item`, `LibraryEntry`, `EntryPerson`/`ItemPerson`,
  `MediaFile`) all five modules sit on. Also proposal-status, but the kernel
  types it describes already exist verbatim in `internal/domain` — that part
  has effectively already been ratified by implementation.

`docs/technical/v1/music-data-model.md` — a frozen, descriptive-only
snapshot of the pre-reset system — is the source for the confidence-scoring
history referenced below. It documents what the old system did, not what
this one must do; where it disagrees with this ADR, this ADR wins.

**No kernel struct changes are needed for Music**, confirmed against the
live code, not assumed: `ContentType.music`, `Kind.artist`,
`ExternalIDSource.mbz`/`mbz_recording` already exist; `Group.Number` is
already a string (vinyl side-lettering); `Item.Metadata`/`MediaFile.Metadata`
already exist as the bags v1 used for music-specific fields;
`ItemRepository.List` already filters by `(libraryEntryID, contentType,
groupID)`, so "tracks in this Release Group" already works.

The one genuinely new piece is a fourth tier: **`MusicRelease`** — a
specific pressing/edition, between `Group` (the logical album) and the
`Item`s (tracks) on disk. Every other module fits the three-tier
`LibraryEntry → Group → Item` skeleton; Music is the one that needs a fourth.

## Decision

### Reuse, unmodified

| Concept | Purser entity | Metadata/notes |
|---|---|---|
| Artist | `LibraryEntry(Kind=KindArtist)` | `Metadata`: `artist_type`, `aliases`, `founded_date`/`dissolved_date` (or `born_date`/`died_date` for solo), `isni`, `official_url`, `lastfm_url`, `wikipedia_url` |
| Release Group (Album) | `Group` | `Metadata`: `album_type` (`studio`\|`live`\|`compilation`\|`ep`\|`single`\|`other`) |
| Track | `Item(ContentType=music)` | `Metadata`: `release_id`, `disc_number`, `isrc`, `composer`, `lyricist`. `Sequence` = track/disc position (string). `ExternalID` source `mbz_recording` at `EntityType=item`. `LibraryEntryID` (kernel-required on every `Item`) = the track's Artist, the same value as its `Group.LibraryEntryID` — denormalized, not derived, matching every other content type's `Item`/`Group` pairing. |
| Audio file | `MediaFile` | `Metadata`: `acoustid` (populated by a future async job, not built here) |
| Band members / solo self-link | `EntryPerson` + `Person` | Roles: `member`\|`former_member`\|`vocalist`\|`guitarist`\|`bassist`\|`drummer`\|`keyboardist`\|`producer` |
| Track credits | `ItemPerson` | Roles: `artist`\|`featured_artist`\|`producer`\|`songwriter`. `CreditedAs` carries an artist-credit name distinct from `Person.Name`. |
| Genre/mood/style | `Tag` | `Scope=metadata`, attached at `Group` level |
| Cover art | `Image` | See "Cover art" below |

**Solo artists get two records, never one collapsed record**: a
`LibraryEntry(Kind=Artist)` for the act and a separate `Person` for the
human, linked via an ordinary `EntryPerson` row — carried forward from v1
and the synthesis doc unchanged. Collapsing them would make solo artists a
special case in every cross-module "who's linked to this person" query.

### New entity: `MusicRelease`

A module-owned type (`internal/domain/music`, mirroring
`internal/domain/afterdark`'s convention) — a specific pressing/edition:

| Field | Notes |
|---|---|
| ID | server-generated, per [0020](0020-server-generated-kernel-entity-ids.md) |
| GroupID | → Release Group, required |
| LibraryEntryID | → Artist, required, denormalized for an efficient "all releases by this artist" query |
| Title | edition-qualified, e.g. "Hi Infidelity (2024 Remaster)" |
| Country, Date, Label, CatalogNumber, Barcode | |
| Format | `CD`\|`Digital Media`\|`Vinyl`\|`Cassette`\|`SACD`\|etc. |
| MediumCount, TrackCount | |
| IsDefault | MBZ's canonical release for the Release Group |
| Monitored | |
| Status | `stub` (known from MBZ, nothing on disk) \| `partial` (some tracks) \| `imported` (all tracks) — closed, Music-owned enum |
| MBID | plain field — see below, not routed through shared `ExternalID` |
| AddedAt, UpdatedAt | |

### `MBID`/`Barcode` are plain fields, not routed through shared `ExternalID`

`ExternalID.EntityType` is a closed, validated `oneof` —
`library_entry|group|item|person` — and [0020](0020-server-generated-kernel-entity-ids.md)'s
Context explicitly frames it as "a fixed, small set of kernel-structural
attach points, not something new content types extend." Extending it for
`music_release` would contradict that stated intent for one caller's
benefit. Separately, `ExternalIDRepository` has no lookup-by-value — finding
a release by barcode or MBID needs a dedicated indexed lookup either way,
which a generic join-row repository doesn't provide. `MusicRelease.MBID`
and `.Barcode` are therefore plain, indexed fields on `MusicRelease` itself.

This is a precedent, stated explicitly so it isn't re-litigated per module:
**a module's own sub-entity (not one of the 7 kernel single-ID types) that
needs a stable external identifier gets a plain indexed field, not an
`EntityType` extension.** The next module facing this (a Books `Edition`
sub-entity, say) follows the same pattern.

### Various Artists compilations: a sentinel `LibraryEntry`, not a nullable `LibraryEntryID`

MusicBrainz does not null this out — it has a real, well-known artist
record for exactly this case (MBID `89ad4ac3-39f7-470e-963a-56509c546377`,
name "Various Artists"), and every VA compilation's release-group is owned
by that one artist in MBZ's own graph.

**Decision:** import that record once as an ordinary
`LibraryEntry(Kind=Artist)` (with an `mbz` `ExternalID` pointing at that
MBID), and every VA compilation's `Group.LibraryEntryID` points at it like
any other artist. `LibraryEntryID` stays required, never nullable, on both
`Group` and `MusicRelease`.

This works because per-track artist attribution never lived at the
Group/Artist level — it's `ItemPerson{Role: artist|featured_artist,
CreditedAs}` per track, already designed to carry an artist-credit name
distinct from the canonical `Person`. "What has artist X appeared on"
(including guest spots on a VA compilation) is answered by querying
`ItemPerson` by `PersonID` — Part 4, use case 3 of the synthesis doc — not
by which `LibraryEntry` owns the containing `Group`.

**Rejected alternative — nullable `LibraryEntryID`, or `Collection`.**
Nullable would force a nil-branch into every artist-scoped query
(list-by-entry, cascade delete, discography browse) and break the synthesis
doc's rule that a `Group` belongs to exactly one `LibraryEntry` — the exact
property that distinguishes `Group` from `Collection`. `Collection` itself
doesn't fit either: it models an ordered set of otherwise-independent
sibling top-level entries (a movie franchise, a book series); a VA
compilation is one album with heterogeneous per-track artists, not a set of
siblings.

### Cover art: `Image`, not a dedicated field

`MusicRelease` cover art is a kernel `Image` row owned by the release (same
`OwnerType`/`OwnerID` polymorphic-attachment mechanism every other owner
type already uses, per `docs/technical/shared-domain-model.md` Part 3). No
dedicated cover-art field on `MusicRelease`. This inherits the existing gap
noted in [0013](0013-image-blob-storage.md): the local-disk `ImageStore`
blob adapter exists and is tested, but no RPC calls it yet, so no owner type
— person, performer profile, or music release alike — can actually receive
uploaded bytes today. That gap is pre-existing and orthogonal to this ADR;
Music inherits it rather than reintroducing a Music-specific workaround.

### Track ↔ Release linkage is owned by the Music module, not the shared Item port

The link from a track to the specific edition it's on
(`Item.Metadata["release_id"]` → `MusicRelease.ID`) is indexed and queried
by the Music module's own release repository, not by adding a
music-specific filter to the shared item port. v1 made this same call, to
avoid a `ReleaseID` filter leaking into `ItemFilter` for every other
content type; that reasoning still holds under
[0002](0002-solid-design-principles.md)'s Interface Segregation Principle.

### Storage shape

The Music release repository needs lookups no existing generic storage
shape in [0012](0012-datastore-persistence.md) covers in one type: filtered
listing by both Release Group and Artist independently, two unique
point-lookups (MBID, Barcode), and the track-by-release query above. It is
therefore its own hand-written translator against the generic `Datastore`
— the same category `Image`'s and `Tag`'s translators already are, not a
new kind of backend or a `Datastore` interface change. Every index it needs
is the same `Document.Index` mechanism [0012](0012-datastore-persistence.md)
already provides.

### Services

The release gets ordinary single-port CRUD service, per
[0011](0011-api-design.md)'s "no God service" rule, plus a composing
deletion service per [0015](0015-deletion-impact-and-composing-services.md):
`Unlink` (default) clears `Item.Metadata["release_id"]` on every track
pointing at the deleted release, leaving the tracks and their Release Group
intact — the same "detach, don't destroy" pattern `GroupService` already
uses for `Item.GroupID`.

API surface follows [0011](0011-api-design.md)'s standard per-entity shape
(Create/Get/Update-with-field-mask/Delete/List) plus a deletion-impact
lookup, in a new `purser.music.v1` proto package — mirroring
`GroupService`'s shape and AfterDark's module-package precedent exactly.
`id` is server-generated per [0020](0020-server-generated-kernel-entity-ids.md)
— an 8th single-ID entity following that pattern, not an exception to it.

### Ripple effects into existing kernel composing services

`MusicRelease` is a new referrer two already-built deletion-impact services
don't yet know about:

- **`GroupDeletionService`** must add the release repository to its
  referrer set — deleting a Release Group without this leaves `MusicRelease`
  rows orphaned, exactly the failure mode [0015](0015-deletion-impact-and-composing-services.md)'s
  self-audit item 2 names. `MusicRelease.GroupID` is required (unlike
  `Item.GroupID`, which is nullable), so "detach" isn't a valid state for
  it the way it is for `Item` — a `MusicRelease` can't be left pointing at
  nothing. `Unlink`, applied to `Group`, therefore means the same thing it
  already means for `Person`'s referrers (`EntryPerson`/`Image`/
  `ExternalID`/`TagAssignment`): delete the referencing rows themselves —
  here, the `MusicRelease` rows — while leaving what they point at further
  downstream intact. Concretely: deleting a Group deletes its
  `MusicRelease` rows (each going through the same Unlink step described
  above — clearing `Item.Metadata["release_id"]` on their tracks), while
  the tracks themselves and the Group's other referrers are untouched.
  This is a real behavior, not just bookkeeping, and needs its own
  acceptance criteria when built, not just "referrer added."
- **`LibraryEntryDeletionService`** must do the same one level up, since
  `MusicRelease.LibraryEntryID` is denormalized specifically so an artist
  delete can reach it. In practice this mostly falls out of the Group-level
  fix above once `LibraryEntryDeletionService`'s existing cascade into
  `GroupDeletionService` runs — but `GetLibraryEntryDeletionImpact`
  (Unlink mode, no cascade) still needs the release repository directly to
  report an accurate count without deleting anything.

Both are existing, already-wired kernel services that must be edited as
part of building this module — a required change to code that predates
Music, not new code, and called out here so it isn't discovered mid-build.

### Explicitly deferred — the scanning/confidence-score seam

Not built in this pass, and this ADR takes no position on their eventual
shape: the import queue, per-file/per-folder tag consensus, ranked release
candidates, and the weighted multi-signal confidence scoring (barcode,
ISRC, fuzzy name, track count, track title set, duration, acoustic
fingerprint) v1 used, plus the plurality-vote album-context reconciliation
logic built on top of them. Per `docs/technical/shared-domain-model.md`
Part 5, this is pipeline/acquisition-core territory, not module data model,
and gets its own design pass later. Real Last.fm/AcoustID metadata-provider
adapters are a separate, ordinary adapter-layer backlog item — neither
decision changes anything in this ADR.

What this ADR deliberately keeps open for that future work, so it isn't
foreclosed:

- `MusicRelease.Status` is a plain, field-mask-updatable field — the future
  pipeline transitions `stub → partial → imported` via the same update RPC
  a human would use, no special endpoint.
- `MediaFile.Metadata` (already generic) is where a future match-detail
  record and the AcoustID fingerprint land — no schema change needed when
  that work starts.
- `Item.Metadata["release_id"]` is indexed by the Music release repository
  from the start (see "Storage shape" above) specifically because
  retrofitting an index later needs a backfill —
  [0012](0012-datastore-persistence.md)'s self-audit already names this as
  the expensive-to-defer case.

## Consequences

- Building Music's CRUD layer requires touching two files outside the new
  module (`GroupDeletionService`, `LibraryEntryDeletionService`) — a known,
  scoped cost, not scope creep, called out explicitly so it isn't discovered
  mid-implementation.
- `MusicRelease.MBID`/`.Barcode` as plain fields (not `ExternalID` rows)
  means Music's external-identity story is asymmetric with Artist/Release
  Group/Track, which all use the shared `ExternalID` join. This is a
  deliberate, documented split, not an inconsistency — see the rationale
  above — but worth knowing when a future reader looks for a Release's MBID
  in the `external_id` collection and doesn't find it there.
- A one-time data-seeding step (import the MBZ "Various Artists" sentinel
  `LibraryEntry`) is required before any VA compilation can be created —
  this needs to exist before the first VA import, not be discovered as a
  runtime error when one is attempted.
- The confidence-scoring/import-queue pipeline is explicitly not designed
  here. Anyone picking that work up next reads this ADR's "Explicitly
  deferred" section first rather than re-deriving where the seams are.

## Self-Audit Checklist

1. Does any code add a music-specific field to `Item`, `MediaFile`, `Group`,
   or `LibraryEntry` instead of using their existing `Metadata` bags? If
   yes — fix it; per this ADR, zero kernel struct changes are needed.
2. Does the Music release repository get forced into the generic
   single-ID/no-filter storage shape by dropping its filter/point-lookup
   methods, instead of being its own hand-written translator? If yes — fix
   it; it doesn't fit that shape, per [0012](0012-datastore-persistence.md)
   self-audit item 7.
3. Does `ItemRepository`/`ItemFilter` gain a release-ID filter instead of
   the Music module owning that index itself? If yes — fix it; that was a
   deliberate ISP decision, not an oversight.
4. Does any code route `MusicRelease`'s MBID/Barcode through the shared
   `ExternalIDRepository`, or extend `EntityType`'s closed enum for
   `music_release`? If yes — stop; that was explicitly rejected above.
5. Does a Various Artists compilation get created with a null/empty
   `LibraryEntryID` instead of pointing at the imported "Various Artists"
   sentinel `LibraryEntry`? If yes — fix it.
6. Does `GroupDeletionService` or `LibraryEntryDeletionService` still omit
   the Music release repository from its referrer set once `MusicRelease`
   exists? If yes — fix it before merging; this is the exact gap
   [0015](0015-deletion-impact-and-composing-services.md) warns will
   otherwise ship silently.
7. Does `GroupDeletionService`'s `Unlink` path try to null/clear
   `MusicRelease.GroupID` instead of deleting the referencing `MusicRelease`
   rows outright (each going through its own Unlink, per item 8 below)? If
   yes — fix it; `GroupID` is required, "detach" isn't a valid state for it.
8. Does the Music release deletion service's `Unlink` path delete tracks
   instead of clearing `Item.Metadata["release_id"]`? If yes — fix it;
   `Cascade` must stay explicit opt-in per
   [0015](0015-deletion-impact-and-composing-services.md).
9. Does any code build the import-queue/confidence-scoring machinery as
   part of this work? If yes — that's out of scope for this ADR; stop and
   raise whether a new ADR is needed first.
