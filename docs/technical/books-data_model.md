# Books Module — Data Model Proposal

> Status: proposal, not yet an ADR-backed decision. Fifth and final module
> in the per-module documentation pass (after [Music](music-data_model.md),
> [AfterDark](afterdark-data_model.md), [Movies](movies-data_model.md), and
> [TV](tv-data_model.md)), same template and ground rules: nothing here
> commits to a Go type, People/Tags/External IDs are deferred as shared
> concepts, and this doc is independently derived rather than assuming any
> prior shape is correct.

Provider set for this pass, per your instruction: **Open Library** (no key
needed) plus whatever the current *arr-ecosystem community actually uses —
**not** Google Books (no key available, explicitly out of scope this time).
Before researching providers directly, this pass checked what the community
uses today, since the book-metadata landscape has shifted significantly and
trusting stale assumptions here would be a mistake — see below.

---

## 0. What the community actually uses (checked live, not assumed)

**Readarr — the historical reference project for this kind of tool — is
retired.** Its metadata backend (Goodreads-derived) broke after Goodreads
shut down its public API, and the project stopped active maintenance. This
matters directly: it means "do what Readarr does" is no longer a safe
default to reach for without checking, which is why this section exists
before any provider research.

What's replaced it, confirmed via live search this session:

- **Open Library** — the consensus fallback/primary across every current
  tool checked (Shelfarr, rreading-glasses, Livrarr). Free, no key, and its
  Works/Editions data model is explicitly cited by more than one project as
  the right shape to build on.
- **Hardcover** (hardcover.app) — the modern, actively-curated
  Goodreads-replacement. Free GraphQL API, but requires a **personal**
  account API token (not a service-level key) — see Section 1 for why that
  matters operationally.
- **Shelfarr** (`github.com/Pedro-Revez-Silva/shelfarr`) — an actively
  maintained (commits as recent as June 2026) self-hosted ebook/audiobook
  request-and-download system, structurally the closest current analog to
  this project's AfterDark/Movies/TV modules. Its own planning doc states
  it uses **Open Library as the core source** (search, work, author, cover
  data) with **Hardcover** as an alternate, and explicitly falls back to
  Google Books / Audible scraping only for audiobook-specific gaps
  (narrator, runtime) that Open Library doesn't cover — confirming Google
  Books' role was audiobook-specific, not something this module loses much
  by skipping per your instruction.
- **rreading-glasses** (`github.com/blampe/rreading-glasses`) — a caching
  proxy built specifically to keep legacy Readarr-compatible clients
  working, with pluggable backends (Goodreads legacy dumps, Hardcover,
  OpenLibrary). Not a provider itself, but useful prior art: it models
  Works/Editions as a read-through cache keyed by ID, serving up to 20
  deduplicated top editions per book — the same Work→Edition shape Open
  Library and Hardcover both natively use.
- **Livrarr** — another 2026-era alternative, pulls from Hardcover +
  OpenLibrary + Audnexus with AI-assisted disambiguation when matches are
  weak. Mentioned for completeness; not investigated further, since it's a
  consumer of the same two providers this doc covers, not a third source.

Net effect on scope: **Open Library (primary) + Hardcover (secondary)** is
not just "what's available without Google Books" — it's independently
confirmed as what the current community has converged on regardless of key
availability.

---

## 1. Providers covered

| Source | Auth | `.env` key(s) | Role |
|---|---|---|---|
| Open Library | None | none needed | Primary: search, canonical Work metadata, Author detail (with extensive cross-provider ID linking), Edition detail, ISBN lookup, cover images |
| Hardcover | Personal account API token (Bearer) | **none currently in `.env`** — new gap, same pattern as the TPDB gap noted in the AfterDark doc | Secondary: community-curated Book/Edition/Author/Series data with an explicit default-edition-per-format concept; richer contributor-role modeling than Open Library |

**Operational difference worth flagging up front:** every other provider
in this project (StashDB, ThePornDB, TMDB, TVDB, OMDb, MusicBrainz,
TheAudioDB, fanart.tv, Last.fm) uses a service-level API key — one
credential per Purser instance. Hardcover's tokens are **personal**, tied
to an individual account, expire annually (reset every January 1st per
their docs), and are rate-limited to 60 requests/minute with a 30-second
query timeout. If Hardcover is adopted as a real adapter, this is a
meaningfully different operational shape than every other source in this
project — not just a missing `.env` line.

### Live verification vs. schema-only

Open Library was live-queried end-to-end this pass (search, work, author,
edition, ISBN, covers — all against *The Hobbit* / J.R.R. Tolkien). Hardcover
was **not** live-queried, since no personal token exists in this
environment — its shapes below are taken directly from the public GraphQL
schema (`github.com/hardcoverapp/hardcover-docs/schema.graphql`, fetched
live this pass), the same "documented, not live-tested" treatment given to
AcoustID in the Music doc.

---

## 2. Provider data shapes

### Open Library

**Base URL:** `https://openlibrary.org/`. **Auth:** none. **Covers:**
`https://covers.openlibrary.org/b/id/{cover_id}-{S|M|L}.jpg` (live-verified,
returns a real JPEG).

`GET /search.json?q=&fields=` (live-verified, trimmed to relevant fields):
```
key ("/works/{id}"), title, author_name[], author_key[],
first_publish_year, edition_key[], isbn[], cover_i, language[],
subject[], ratings_average, ratings_count, want_to_read_count,
already_read_count, number_of_pages_median
```
Search results are **Work-level**, not Edition-level — `edition_key[]` is a
list of every known edition's key, `isbn[]` aggregates ISBNs across all of
them. Community rating/engagement counts (`ratings_average`,
`want_to_read_count`, `already_read_count`) are returned directly on
search results with no separate call — a convenience none of the other
four modules' primary providers offer at search time.

`GET /works/{id}.json` (live-verified against *The Hobbit*,
`/works/OL27482W`):
```
key, title, type, authors[]{author{key}, type},
description{value} | description (string, inconsistent across records),
first_publish_date, covers[], links[]{title, url},
subjects[], subject_places[], subject_people[], subject_times[],
first_sentence{value}, excerpts[]{excerpt, comment, author},
created{value}, last_modified{value}, revision
```
`subjects[]` for this one work returned **~90 free-text values**, ranging
from real genre-like terms ("Fantasy fiction") to hyper-specific ones
("Baggins, bilbo (fictitious character), fiction") — closer in spirit to
AfterDark's deep, granular tag taxonomy than Music's coarse genre fields,
but *unstructured* (no category/group grouping the way StashDB tags have).

`GET /authors/{id}.json` (live-verified against Tolkien,
`/authors/OL26320A`):
```
key, name, personal_name, fuller_name, alternate_names[],
birth_date, death_date, bio, links[], photos[],
remote_ids{ viaf, wikidata, isni, goodreads, storygraph, amazon,
            librarything, imdb, musicbrainz, bookbrainz,
            project_gutenberg, librivox, lc_naf, opac_sbn },
source_records[]
```
**`remote_ids` is a real, populated cross-provider hub** — this single
live-verified author record links to 13 external identifiers, including
`goodreads`, `storygraph`, `amazon`, and notably `musicbrainz`/`bookbrainz`
(useful if an author is also a recording artist or has MusicBrainz-tracked
work). This is the same convergence pattern found in every other module
(MBID-as-join-key in Music, StashDB↔ThePornDB URLs in AfterDark,
TMDB↔TVDB `remoteIds`/`external_ids` in Movies/TV) — Open Library's author
records are already a strong identity anchor, not just a search index.

`GET /books/{id}.json` (Edition detail, live-verified) /
`GET /isbn/{isbn}.json` (redirects to the same Edition shape, live-verified,
`-L` flag needed to follow the redirect):
```
key ("/books/{id}"), title, subtitle, works[]{key},
publish_date, publish_places[], publishers[],
physical_format, pagination, number_of_pages, edition_name,
isbn_10[], isbn_13[], lccn[], oclc_numbers[],
identifiers{ goodreads[], ... },   -- present when available, sparse
languages[]{key}, covers[], classifications{},
source_records[], created{value}, last_modified{value}
```
Editions carry their own sparse `identifiers{}` bag (only `goodreads` was
populated on the ISBN-looked-up edition checked live) — a weaker,
less-structured version of the Author-level `remote_ids`. Edition-level
cross-provider linking is real but inconsistent, unlike the Author level
where it was comprehensive.

### Hardcover

**Base URL:** `https://api.hardcover.app/v1/graphql` (GraphQL, POST).
**Auth:** `Authorization` header with a personal token. Schema fetched
live from the public docs repo, not queried against real data (no token
available in this environment).

`books` type (schema-verified, relationship fields elided):
```
id, title, subtitle, description, slug, headline,
release_date, release_year, pages, audio_seconds,
rating, ratings_count, reviews_count, editions_count,
compilation (bool), is_partial_book,
canonical: books, canonical_id,   -- self-reference for duplicate/merge resolution
default_cover_edition_id, default_audio_edition_id,
default_ebook_edition_id, default_physical_edition_id,
featured_book_series_id, image_id
```
**`canonical`/`canonical_id` is a self-referential merge mechanism** — a
duplicate or variant Book record points at its canonical counterpart,
conceptually similar to MusicBrainz's artist/release merge history, but
exposed directly as a queryable field rather than requiring redirect
handling.

**`default_*_edition_id` (cover/audio/ebook/physical) is a genuinely new
concept** not seen in any other module or in Open Library: Hardcover
explicitly tracks *which specific edition* is canonical *per format type*.
This is a direct, structured answer to a problem every other module in
this pass has had to leave as an open question (Movies' release-date
matrix, TV's content-rating matrix) — "which of several equally-valid
records is THE one to show" is answered per-format here instead of via an
ad hoc policy.

`editions` type (schema-verified):
```
id, book_id, title, subtitle, isbn (via book_mappings, not a direct field),
asin, edition_format, edition_information, physical_format,
physical_information, pages, audio_seconds,
release_date, release_year, publisher_id, language_id, country_id,
reading_format_id, rating, score, source, state, locked,
isbns_match (bool), normalized_at
```
`reading_format_id`/`reading_formats` (referenced, not expanded in this
schema excerpt) is presumably the physical/ebook/audiobook distinction —
confirms Hardcover treats format as a first-class edition attribute, not
an inferred one.

`contributions` type — the author-credit join, schema-verified:
```
id, author_id, book_id, contribution (String, nullable),
contributable_id, contributable_type
```
`contribution` is a **free-text string**, not a fixed enum — Hardcover's
own answer to the "role vocabulary: fixed enum or open string" question
raised independently in the Movies doc (TMDB crew `job` field) and the TV
doc (aggregate cast roles). Real-world precedent here leans toward open
string for book contributor roles (author, illustrator, translator,
editor, narrator — an open-ended, format-dependent set).

`book_series` type — schema-verified:
```
id, book_id, series_id, position (float8), details, compilation, featured
```
**`position` is a float, not an int** — supports non-integer series
positions (e.g. a novella at position `1.5` between books 1 and 2), a real
and common need in book series that neither Music's track `Sequence`
(string, for vinyl side-lettering) nor any other module's ordering field
was designed around. This is a genuine, book-specific schema requirement.

`book_mappings` type — the external-ID mechanism, schema-verified:
```
id, book_id, edition_id, external_id (String), external_data_id,
loaded (bool), loaded_at, attempts, platform (relationship)
```
Same `(entity, platform/source, external_id)` shape as every other
provider's cross-reference mechanism in this project — nothing new to
design here, it fits the existing shared `ExternalID` concept directly.

---

## 3. Proposed modern entity model

Conceptual, not Go structs. People/Tags/External IDs deferred to the
shared-domain synthesis phase, consistent with the other four modules.

### Work (the book, independent of edition)

| Attribute | Notes |
|---|---|
| Title, subtitle, description | Both providers agree on this shape |
| First publish date/year | Open Library's `first_publish_date`; Hardcover's `release_date`/`release_year` at the Book level (edition-level release dates also exist on both — same "work vs. specific printing" distinction Music already solved with Release Group vs. Release) |
| Subjects/genres | Deferred to shared **Tags** — Open Library's are unstructured free text (~90 values seen live), Hardcover's are presumably similar (not directly inspected, `taggings`/`taggable_counts` relationships exist in the schema but weren't expanded this pass) |
| Series membership + position | Deferred to shared **Groups**, but **position needs to support fractional values** (Hardcover's `float8`) — a real schema requirement this module introduces that none of the other four needed |
| Default edition per format | **New concept from Hardcover**, no Open Library equivalent. Worth adopting even if Hardcover itself isn't: "which edition is canonical for display, per format (ebook/audio/physical/cover-image)" is a real problem this module has that others don't (Music's `IsDefault` on `MusicRelease` is the closest precedent, but that's one canonical release, not one per format) |
| Merge/canonical resolution | Hardcover's `canonical`/`canonical_id` self-reference — same shape as any duplicate-resolution mechanism, not something to design fresh |
| External identity | Open Library work key (primary anchor, since it's the free/no-key source), cross-referenced via Author-level `remote_ids` to Goodreads/Hardcover-adjacent identifiers (though Hardcover doesn't appear directly in the one live `remote_ids` sample checked — `storygraph` was present, `hardcover` was not, worth re-checking against a more recently-added author) |

### Edition (a specific printing/format)

| Attribute | Notes |
|---|---|
| Format (physical/ebook/audiobook), publisher, publish date, page count | Both providers cover this; Hardcover's is more structured (`reading_format_id` as a real relationship vs. Open Library's free-text `physical_format`) |
| ISBN/ASIN | Open Library: `isbn_10[]`/`isbn_13[]` direct fields. Hardcover: via `book_mappings`, not a direct field — structurally different access pattern for the same data |
| Audiobook-specific fields (narrator, duration) | **Confirmed gap in both providers checked here** — Shelfarr's own research (Section 0) found Open Library weak here and reached for Audible scraping; Hardcover has `audio_seconds` at both Book and Edition level but no narrator field found in the schema excerpts pulled this pass. If audiobooks matter to this module, a third source may be unavoidable regardless of the Google Books exclusion |
| Cover image | Open Library: `covers[]` → `covers.openlibrary.org` (live-verified, direct and reliable). Hardcover: `image_id` → `images` relationship (not expanded this pass) |

### Deferred to shared domain (not designed here)

- **People** — authors, illustrators, translators, narrators via
  Hardcover's open-string `contribution` field or Open Library's
  simpler `authors[]` (work-level only, no role distinction found in the
  Open Library work schema this pass — Open Library appears to treat
  non-author contributors as a weaker concept than Hardcover does).
- **Tags** — subjects/genres, unstructured on Open Library, not directly
  inspected on Hardcover this pass.
- **External IDs** — same `(entity_type, entity_id, source, external_id)`
  shape as every other module. Two new wrinkles worth carrying into the
  synthesis phase: Open Library's Author-level `remote_ids` is dramatically
  richer than its Edition-level `identifiers{}`, and neither prior art's
  `external_ids.source` enum (`stashdb|tpdb|tmdb|tvdb|mbid|javlibrary|r18|discogs|mal|anilist`)
  nor the content-types table anticipated books at all — see Section 4.

---

## 4. Comparison to prior snapshot

**Books was never anticipated in prior art at all** — a real gap, not a
minor one. The recovered `docs/project/content-types.md` table (also used
for the AfterDark/Movies/TV comparisons) has exactly five rows: `movie`,
`tv`, `music`, `adult`, `jav`. No `book` row exists, despite `ops/purser.yaml`
already listing `books` as a configured module (per the third exploration
agent's findings, early in this session). Similarly, `v1/data-model.md`'s
`external_ids.source` enum (`stashdb|tpdb|tmdb|tvdb|mbid|javlibrary|r18|
discogs|mal|anilist`) has no `openlibrary`/`goodreads`/`hardcover` value,
and its `item_people.role` enum (`performer|actress|director|actor|artist|
producer`) has no `author` value despite the general prose elsewhere in
that same doc explicitly mentioning "authors (book)" as an `entry_people`
use case. This module has more genuinely new ground to cover than any of
the other four — there's no prior structural decision to agree or disagree
with, only a gap to fill.

The one thing this pass can say with confidence against the *general*
shared schema (not book-specific prior art, since none exists): the
Work→Edition split this module needs maps directly onto the same
Group→Item pattern Music already established for Release Group→Release,
and both real providers researched here (Open Library, Hardcover) already
use that exact two-level split natively — this is the strongest
precedent-alignment finding of any module in this pass, even though the
book-specific prior art itself doesn't exist.

---

## 5. Open questions

1. **Fractional series positions** — Hardcover's `book_series.position` is
   a float. Does the shared `Group`-adjacent ordering concept need to
   support this, or is "1.5" handled by convention (e.g. stored as a
   string, like Music's vinyl-side track `Sequence`)? Real, book-specific
   requirement, not hypothetical.
2. **Default-edition-per-format** — worth generalizing beyond Hardcover
   specifically? This is the cleanest solution to a problem that showed up
   independently in Movies (release-date matrix) and TV (content-rating
   matrix): "there are several valid records, pick the right one for
   context." A per-format default is books-specific in form but the
   underlying problem (context-dependent canonical selection) is shared
   across at least three of the five modules now.
3. **Hardcover's personal-token auth model** — every other provider in
   this project uses one service-level key. Does adopting Hardcover mean
   Purser needs a per-user credential story it doesn't have anywhere else,
   or does a single operator token get shared across all users (same
   pattern as every other provider, just philosophically odd given
   Hardcover's account-bound design)? Product decision, not a data-model
   one, but it blocks adapter design either way.
4. **Audiobook metadata gap** — confirmed by both this pass and Shelfarr's
   own research. Is a third source (Audible scraping, Audnexus — used by
   Livrarr per Section 0) worth adding later, or is audiobook-specific
   metadata (narrator, chapter breaks) out of scope for a first cut of this
   module?
5. **Contributor role vocabulary** — same fixed-enum-vs-open-string
   question raised in Movies/TV, with Hardcover's real-world `contribution`
   field as another data point favoring open string for this module
   specifically (author/illustrator/translator/editor/narrator is a wider,
   more format-dependent set than any other module's role vocabulary).
6. **Filling the prior-art gap properly** — should `docs/project/
   content-types.md`'s table (if/when a live equivalent is rebuilt) get a
   `book` row, and should `v1`-derived enums (`external_ids.source`,
   `item_people.role`) be treated as needing `openlibrary`/`hardcover` and
   `author` added, given this pass confirms both are real, needed values?
   Not answered here since it's a decision about editing/extending
   inherited-but-frozen v1 material, not a new-module data-model question.
