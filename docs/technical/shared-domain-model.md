# Shared Domain Model — Synthesis

> Status: proposal, not yet an ADR-backed decision. This is the synthesis
> phase promised at the end of the five module docs
> ([Music](music-data_model.md), [AfterDark](afterdark-data_model.md),
> [Movies](movies-data_model.md), [TV](tv-data_model.md),
> [Books](books-data_model.md)) — it defines the actual shared kernel and
> each module's domain model against real provider data already gathered,
> not against assumption. Once agreed, this becomes the spec for the first
> `internal/domain` code.

## Design principles this doc is built against

These came directly from the planning conversation and constrain every
decision below:

1. **Storage is dumb.** No adapter (SQL, BadgerDB, whatever) implements any
   business rule. This is already ADR 0001's contract, not a new rule —
   restated here because it directly rules out designs that would tempt a
   SQL adapter into doing a join the domain should be doing, or a cascade
   the service layer should own. v1's `contract/` conformance-test pattern
   (same test suite run against every backend) is the enforcement
   mechanism, not code review vigilance.
2. **One Person, many roles.** The same human can be an author, an actor,
   and a performer simultaneously, and the system needs to know that's one
   person, not three unrelated records.
3. **Tags are global.** `genre:gonzo` on a scene, a movie, and a book all
   need to be the same tag row, queryable across content types with one
   join shape.
4. **Composition, not shared-struct bloat.** Module-specific rich metadata
   (a performer's measurements, an author's bio-adjacent fields) never
   becomes a new field on a shared struct — it lives in a module-owned type
   that references the shared entity by ID.
5. **API-first, gRPC-primary, composable calls.** No ADR governs the API
   layer yet (flagged as a gap needing its own ADR, separate from this
   doc). What it means *here*: ports should stay narrow enough that they
   map onto small, composable RPCs later, not one bundled call per screen.

## ADR conformance

This touches `internal/domain` shapes directly — [0001](../adr/0001-hexagonal-architecture.md)
and [0002](../adr/0002-solid-design-principles.md) govern it. Conformance
is argued inline where a decision is made, not just asserted here:
domain types carry no adapter/content-type knowledge, module-specific
concerns stay in module packages or metadata bags, and every new
capability (a module's profile type, its own ports) is additive rather than
an edit to shared code.

---

## Part 1 — The Shared Kernel

Lives in `internal/domain/` with zero imports of any module package. Any
type here must be meaningful without knowing whether it's being used for a
movie, a scene, or a book.

### Person

Individual humans only — not acts, bands, or studios (those stay
`LibraryEntry` with a `Kind`, matching v1). A band's members are `Person`
records; the band itself is not.

| Field | Type | Notes |
|---|---|---|
| ID | string | |
| Name, SortName | string | |
| Aliases | []string | v1's proven shape, reused as-is — no reason to change it |
| Gender | enum | `MALE`, `FEMALE`, `TRANSGENDER_MALE`, `TRANSGENDER_FEMALE`, `INTERSEX`, `NON_BINARY`, `UNKNOWN`. Values taken directly from StashDB's live `GenderEnum` (confirmed via GraphQL introspection, not guessed), plus `UNKNOWN` as our own fallback. **Adapters, not the domain, own translation into this enum** — TMDB's numeric `gender` codes and ThePornDB's `"Unknown"` string both map to one of these at ingest time, in the adapter, matching ADR 0001 rule 3 (provider quirks stay out of shared code). A provider value with no confident mapping becomes `UNKNOWN`, not a guess. |
| Pronouns | string | Free text (`"she/her"`, `"they/them"`, `"he/they"`, etc.) — deliberately **not** derived from `Gender`, since pronouns don't map 1:1 onto gender identity even with a six-value inclusive enum. Optional, populated by adapter data where available or user-entered otherwise. |
| BirthDate, DeathDate | *time.Time | Nullable |
| Nationality | string | |
| Overview | string | Bio/biography text |
| Monitored, MonitorMode | bool, string | Existing v1 concept — "everything this person appears in" monitoring, unchanged |
| ExternalIDs | via shared `ExternalID` join | — |
| Images | via shared `Image` join, `OwnerType="person"` | The **kernel image** — see Part 3 |
| AddedAt, UpdatedAt | time.Time | |

### Tag

```
Tag{ ID, Key, Value, Scope ("user"|"metadata"), Category string (optional) }
```
`Category` is new versus v1 — StashDB's tags carry a category/group
(`"Finishers"` under `"ACTION"`) that flat key/value can't express. It's an
optional, additive field: everything that only ever populated `Key`/`Value`
keeps working unchanged (OCP — this is exactly the kind of additive
extension ADR 0002 asks for over a breaking redesign).

Join tables unchanged from v1: `item_tags`, `entry_tags`, `group_tags`.
Cross-module tag queries ("show me every movie, book, and scene tagged
`genre:gonzo`") already fall out of this for free — the join is to a
generic entity ID, and `content_type` lives on the *item*, not the tag.
Nothing new needed here beyond `Category`.

### ExternalID

```
ExternalID{ EntityType, EntityID, Source, ExternalIDValue }
```
Unchanged shape from v1. `Source` enum grows to cover everything the five
module docs actually found in use: `mbz`, `mbz_recording`, `audiodb`,
`stashdb`, `tpdb`, `tmdb`, `tvdb`, `omdb`, `openlibrary`, `hardcover`.
Books had **no entry at all** in the old enum — a real gap this synthesis
closes, not a stylistic addition.

### Image

**New shared kernel type**, needed specifically for the kernel-image /
module-image split — see Part 3 for the full design. Shape:

```
Image{ ID, OwnerType string, OwnerID string, ImageType string
       ("hero"|"poster"|"banner"|"background"|"thumb"),
       URL string, Width, Height int, Source string, Priority int }
```
Same polymorphic-owner pattern as `Tag`/`ExternalID` — attach an image to
any entity by `(OwnerType, OwnerID)` without a dedicated array field
predeclared on every struct that might ever have an image. `ImageType`
values and the priority-ordering behavior (TheAudioDB beats fanart.tv, in
Music's case) are the same convention v1 already validated for Music
images — extended here to be usable by any owner type, not just artists.

### Collection

**New shared kernel type — this is the one place this synthesis pass
overturned something I said out loud in the live planning conversation.**
I'd proposed "one shared `Group`, cardinality varies" as the answer to
Movies' Collection-vs-no-group tension. Mapping every module's hierarchy
concretely (Part 2) shows that's wrong: a `Group` belongs to exactly one
`LibraryEntry` (an Album belongs to one Artist, a Season belongs to one
Series). A movie Collection spans *multiple, independent, sibling*
`LibraryEntry` records (The Matrix / Reloaded / Revolutions are each their
own monitorable movie). A book Series has the same problem one level down
— it spans multiple `Work` (`Group`-level) records, which may even have
different authors. That's not "an optional Group," it's a structurally
different relationship: an ordered, named set that cuts *across* the
normal tree instead of nesting inside it. One new concept covers both:

```
Collection{ ID, Name, Kind string ("franchise"|"book_series"|...), Overview }
CollectionMembership{ CollectionID, EntityType ("library_entry"|"group"),
                       EntityID, Position string }
```
`Position` is a string (not int) for the same reason `Group.Number`
becomes one below — Hardcover's fractional series positions (`"1.5"` for a
novella between books) are real, observed data, not a hypothetical.

Movies use `EntityType="library_entry"` (each movie is independently a
`LibraryEntry`). Books use `EntityType="group"` (each Work is a `Group`
under an Author `LibraryEntry`). Same mechanism, different attach point,
because that's genuinely where each module's unit-of-participation lives —
not a mechanism I'm forcing to fit both.

### Group

```
Group{ ID, LibraryEntryID, Title, SortName, Number string, Year int,
       Overview, Monitored, MonitorMode, Metadata map[string]any }
```
One change from v1: `Number` becomes a `string`, matching the treatment
`Item.Sequence` already got for vinyl side-lettering (`"A1"`). This isn't
new complexity — it's applying a pattern v1 already validated for a
different field, now to fix a real gap (fractional book-series-adjacent
numbering, TV's alternate season numbering living in `Metadata` rather than
`Number`).

### Item

```
Item{ ID, ContentType, LibraryEntryID, GroupID (nullable), Title, Overview,
      Date, Sequence string, RuntimeSeconds, Monitored, Status,
      Metadata map[string]any }
```
Unchanged from v1.

### LibraryEntry

```
LibraryEntry{ ID, ContentType, Kind, Name, SortName, Overview, ParentID
              (nullable, self-referential), Monitored, MonitorMode, Status,
              QualityProfileID, MetadataProfileID, Path,
              Metadata map[string]any }
```
Unchanged from v1. Its existing self-referential `ParentID` (already used
for Network → Studio) is reused as-is for nothing new here — Collections
deliberately did **not** reuse `ParentID`, because a movie in a franchise
still needs its *own* independent monitoring/path/status exactly like a
standalone movie; forcing it under a parent `LibraryEntry` would break
that independence. `Collection` is additive membership, not a hierarchy
change.

### EntryPerson / ItemPerson

```
EntryPerson{ LibraryEntryID, PersonID, Role string, CreditedAs string,
             Character string, StartDate, EndDate }
ItemPerson{ ItemID, PersonID, Role string, CreditedAs string, Character string }
```
Two additions versus v1, both surfaced by actually working through the use
cases in Part 4, not invented speculatively:

- **`Character`** — TMDB's cast credits carry a fictional character name
  (`"Walter White"`) distinct from the person's real name. Neither Music
  nor AfterDark's role join needed this, so v1 never had it. Movies and TV
  both need it for basically every cast credit.
- **`CreditedAs`** — a credited/stage name that differs from the person's
  canonical name for *this specific* credit. StashDB already has this
  exact concept (`performers[].as`), and Music's artist-credit name (an
  artist_credit name distinct from the canonical artist) is the same shape.
  Both optional, both null for modules that don't need them — additive,
  not a breaking change to the join's existing meaning.

`Role` stays an **open string**, not a fixed enum — TMDB's `job` field,
Hardcover's free-text `contribution`, and every module's own role set
pushed toward this independently across all five docs. Each module
validates against its own known-roles list in config/adapter code, per
ADR 0001 rule 3 (role vocabulary is adapter/config knowledge, not shared
domain knowledge).

### MediaFile

```
MediaFile{ ID, ItemID, Path, Size, OSHash, MD5, SHA1, Quality, Resolution,
           Codec, Container, Metadata map[string]string }
```
Unchanged from v1. Content-hash/fingerprint identification (AcoustID,
StashDB/ThePornDB PHash) stays out of the kernel entirely — that's
pipeline/acquisition-core territory per the earlier planning conversation,
not a content-metadata concern, and gets its own design pass later. Until
then, per-module fingerprint values live in this `Metadata` bag exactly as
v1 did for AcoustID.

---

## Part 2 — How each module maps onto the kernel

The single clearest result of this synthesis: five very different domains
fit the *same* three-level skeleton (`LibraryEntry → Group → Item`), with
only Music needing a genuinely new tier.

| Module | LibraryEntry (Kind) | Group | Extra tier | Item (ContentType) | Collection use |
|---|---|---|---|---|---|
| Music | Artist | Release Group (Album) | **MusicRelease** (new entity — a specific pressing) | Track (`music`) | Not used yet — flagged for the various-artists-compilation open question, not resolved here |
| AfterDark | Studio | Series (optional) | none | Scene / JAV Title, discriminated by `Metadata["content_kind"]` (`music`) | Not used |
| Movies | Movie (no Group; one auto-created Item, matching v1's collapsed pattern) | — | none | Movie (`movie`) | **Yes** — franchises, `EntityType="library_entry"` |
| TV | Series | Season | none | Episode (`tv`) | Not used |
| Books | Author | Work | none | Edition (`book`) | **Yes** — book series, `EntityType="group"` |

Two findings worth calling out explicitly:

- **Books needs zero new entities**, unlike Music. Music has four real
  tiers (Artist / Album / specific pressing / Track), so v1's
  `MusicRelease` earns its place as a genuinely new entity. Books only has
  three (Author / Work / Edition) — Edition *is* the leaf `Item` directly,
  there's nothing beneath it the way Tracks sit beneath a Release. I
  initially expected Books might need its own extra tier by analogy to
  Music; it doesn't, and forcing one in would have been unjustified
  complexity.
- **AfterDark and Movies both collapse the middle tier** in the common
  case (AfterDark's Series `Group` is genuinely optional — most scenes
  have none; Movies never has one at all) — but they do it for different
  reasons and Movies additionally needs `Collection`, which AfterDark
  doesn't. They're not the same shape wearing different labels.

### Solo artists are not a schema exception

A solo artist gets **two** records, same as a band: a
`LibraryEntry(Kind=Artist)` for the act, and a separate `Person` for the
human, linked by an ordinary `EntryPerson` row — never one record doing
double duty. This is a deliberate rejection of the tempting shortcut
("there's only one member, why not let the Person record just *be* the
artist entry") because that shortcut would make solo artists a special
case: every join, every "who's linked to this artist" query, and the
Profile/composition mechanism in Part 3 would need to know whether it's
looking at a real `LibraryEntry`/`Person` pair or a collapsed one. Keeping
them separate means a solo artist's human is an ordinary `Person` —
linkable to a book, an AfterDark scene, another band — exactly like anyone
else, with zero branching anywhere in shared code. This matches what v1
already did (the artist record linked to itself via `EntryPerson` so
`PersonDetail` showed the solo entry under "Member of"); it's restated
here explicitly so the new kernel doesn't accidentally "simplify" it away.

---

## Part 3 — Person composition and the kernel-image / module-image split

### The Profile pattern

`domain.Person` (Part 1) carries only what's true regardless of role. Each
module that needs richer, role-specific, often-sparse data defines its own
`Profile` type in its own package, keyed by `PersonID` (one-to-one, not
embedded at the storage layer):

```go
// internal/domain/afterdark/performer.go
type PerformerProfile struct {
    PersonID string
    CupSize, BandSize string
    BreastType string
    Tattoos, Piercings []BodyMark
    CareerStartYear, CareerEndYear int
}
```
```go
// internal/domain/books/author.go
type AuthorProfile struct {
    PersonID string
    // deliberately thin — Books research didn't surface author-specific
    // fields beyond what Person already covers (name, bio, dates).
    // Exists as a type mainly so the Profile pattern is uniform across
    // modules, not because Books needs much extra data today.
}
```
Storing `PersonID` as a reference (not embedding `Person`) is what keeps
storage dumb: nothing forces a backend to denormalize Person data into
every module's profile table, and a service decides when/how to join them.

Composition happens at the **read/API boundary**, assembled by a service,
never persisted as a combined row:

```go
type PerformerView struct {
    domain.Person
    afterdark.PerformerProfile
}
```
One `Person` row can back a `PerformerView`, an `AuthorView`, and an
`ActorView` simultaneously — this is the literal mechanism behind "linked
to a book AND an actress AND a performer." A person with no AfterDark
profile simply has no row in that module's profile store; there's no
null-checking sprinkled through domain code for it, just "profile not
found" at the port level.

Each module owns a narrow `XProfileRepository` port (ISP — no monolithic
`PersonExtendedDataRepository` with a type switch inside it deciding which
fields to populate based on content type, which would be exactly the kind
of violation ADR 0002's self-audit checklist calls out).

### Kernel image vs. module image

This is the same Profile pattern applied to `Image` specifically, per your
requirement: the global People page/API shows the safe, cross-context
image; the AfterDark performer view shows the module-specific one.

- **Kernel image**: `Image` rows with `OwnerType="person", OwnerID=person.ID`.
  Shown on the global People page, in cross-module search results, and
  anywhere a person is referenced without a module context.
- **Module image**: `Image` rows with `OwnerType="afterdark.performer_profile",
  OwnerID=personID` (the profile's key). Shown only when viewing that
  person *through* the AfterDark module.

The selection rule ("which image set to show") is a **service-layer
decision, not a struct field** — consistent with how this doc already
treats "which release date/certification/edition is canonical" elsewhere.
A service assembling a `PerformerView` would default to the module image
set, falling back to the kernel image if the module has none uploaded yet;
a service assembling the global People list would use the kernel image
only, regardless of what module images exist. Neither `Person` nor
`PerformerProfile` needs to know the other's images exist — the service
composing the view is the only place that decision lives, which is exactly
where ADR 0001 says content-type/context-specific behavior belongs.

This generalizes for free to every module: Movies could attach a
module-specific poster crop to an `ActorView` later, Books could attach an
author's publisher-headshot variant — same `OwnerType` convention, zero
change to `Image` or `Person`.

---

## Part 4 — Use cases per module

Each use case names the entities/joins involved, so it's checkable against
Part 1–3 rather than asserted.

### Cross-cutting (the reason this redesign exists)

> **"Show me everything this person is connected to, across every
> module."** Query `EntryPerson`/`ItemPerson` by `PersonID` with no
> `ContentType` filter, join back to `Item`/`LibraryEntry` for display
> data, and separately load whichever `XProfile` rows exist for that
> `PersonID`. One `Person` row, results spanning a book's author credit, a
> movie's cast credit, and an AfterDark performer credit in a single query
> shape — this is the concrete payoff of Part 3's design, not a
> hypothetical.

### Music

1. **Track an artist and browse their discography.** Monitor a
   `LibraryEntry(Kind=Artist)`; list its `Group`s (Release Groups/Albums)
   via `LibraryEntryID`.
2. **Browse every known pressing/edition of an album.**
   `MusicReleaseRepository.ListByGroup(groupID)` — the extra tier Part 2
   flagged as Music-specific.
3. **Find every track featuring a specific guest artist.** `ItemPerson`
   join filtered by `PersonID`, `Role="featured_artist"`.
4. **Filter the library by genre tag.** `Tag` join at `Group` level
   (`scope="metadata"`).
5. **See a band's current and former members.** `EntryPerson` join by
   `LibraryEntryID`, filtered by `Role` (`member`/`former_member`/etc.).

### AfterDark

1. **Query scenes by tag.** `Item(ContentType=adult)` joined through
   `item_tags` to `Tag(Key="genre", Value="gonzo")` — same join shape as
   every other module, per Part 1's Tag design.
2. **Query scenes by performer.** `ItemPerson` join by `PersonID`.
3. **List everything from a studio.** `Item` joined to
   `LibraryEntry(Kind=Studio)` directly, or via the optional `Group`
   (Series) when one exists.
4. **Browse a performer's full filmography, Scene and JAV alike.** Same
   `ItemPerson` join as (2) — the `Metadata["content_kind"]` discriminator
   from Part 2 only changes display label, not the query shape.
5. **Assemble a performer's profile page.** Load `Person` +
   `afterdark.PerformerProfile` (Part 3) + module images + credits from
   (2) in one composed `PerformerView`.

### Movies

1. **Browse a franchise in order.** `CollectionMembership` filtered by
   `CollectionID`, `EntityType="library_entry"`, ordered by `Position`.
2. **Browse cast with character names.** `EntryPerson` join by
   `LibraryEntryID` — cast belongs to the title "at rest," matching v1's
   existing `entry_people` vs. `item_people` distinction — reading
   `Character` and `CreditedAs` (Part 1's new fields) alongside `Role`.
3. **Filter by content rating for a chosen locale.** Certification data
   stays in `Item.Metadata` (the release-date/certification matrix from
   the Movies doc is provider-shaped, not a first-class field); a service
   applies the locale policy at read time — deliberately *not* a domain
   type, per the "canonical selection is a service concern" principle.
4. **Show ratings from all three providers on one page.** `ExternalID`
   resolves the `tmdb`/`tvdb`/`omdb` IDs; provider-specific rating
   snapshots live in `Metadata`, assembled by the service, not modeled as
   a shared "Rating" entity (three incompatible shapes, per the Movies
   doc — not worth forcing into one type).

### TV

1. **Browse seasons and episodes in aired order (the default).**
   `Group(Season)` → `Item(Episode)` via `GroupID`, ordered by `Number`.
2. **Support DVD/absolute ordering for a specific series.** Read the
   alternate numbers from `Group.Metadata`/`Item.Metadata` — no new
   entity, per Part 2's explicit call that TVDB's `seasonTypes` stays out
   of the shared schema.
3. **See a recurring actor's aggregate role plus specific guest
   appearances.** `EntryPerson` (series-level aggregate credit) +
   `ItemPerson` (per-episode guest stars), both by `PersonID` — this
   directly resolves the open question left in the TV doc: yes, this maps
   onto the existing `entry_people`/`item_people` split with zero new
   mechanism.
4. **Filter by network or genre tag.** `Tag` join at `LibraryEntry` level
   for network, `Item` level for episode-specific tags.
5. **Track what's next to air, for monitoring.** `Item.Status` +
   `Item.Date` — already in the kernel `Item` shape, nothing new needed.

### Books

1. **Monitor an author, see everything they've written.** `Group`(Work)
   list via `LibraryEntryID` on `LibraryEntry(Kind=Author)`.
2. **Browse every format of one Work.** `Item`(Edition) list via
   `GroupID`, format read from `Item.Metadata["reading_format"]`.
3. **Browse a book series in (possibly fractional) order.**
   `CollectionMembership` filtered by `CollectionID`,
   `EntityType="group"`, ordered by `Position` — supports Hardcover's
   `"1.5"` case directly.
4. **Find every work a translator or illustrator contributed to.**
   `EntryPerson` join by `PersonID`, `Role="illustrator"` (open string,
   per Part 1).
5. **Filter by subject tag.** `Tag` join at `Group`(Work) level.

---

## Part 5 — Open questions carried forward or newly surfaced

Not resolved here — flagged for whoever implements, or for a follow-up
decision:

1. **Various-artists / compilation albums (Music)** — doesn't have an
   owning Artist the way a normal album does. `Collection` (Part 1) is a
   plausible fit (a compilation as a named, cross-artist ordered set) but
   this doc doesn't force that conclusion — it's noted as the most likely
   answer, not decided.
2. **Fingerprint/content-identification as a shared pipeline concept** —
   still explicitly out of this doc's scope (Part 1, MediaFile section).
   Needs its own design pass once the pipeline core is being defined.
3. **API-design ADR** — this doc assumes narrow, composable ports map
   cleanly onto future gRPC/REST calls, but the actual API contract
   design (proto shapes, REST-via-gateway or hand-rolled, pagination
   conventions) is unaddressed and should get its own ADR before that
   layer is built.
4. **`Collection.Kind` vocabulary** — left as an open string
   (`"franchise"`, `"book_series"`) rather than an enum, consistent with
   the `Role` decision, but not exercised against a third use case yet
   (only Movies and Books currently need it).
5. **Profile-repository port shape** — Part 3 asserts each module gets its
   own narrow `XProfileRepository`, but the exact method set (Get by
   PersonID, bulk-load for a list view, etc.) isn't specified here — left
   for implementation, since it's a straightforward, low-risk detail once
   the shape above is agreed.
