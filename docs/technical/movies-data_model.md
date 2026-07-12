# Movies Module — Data Model Proposal

> Status: proposal, not yet an ADR-backed decision. Third module in the
> per-module documentation pass (after [Music](music-data_model.md) and
> [AfterDark](afterdark-data_model.md)), same template and ground rules:
> nothing here commits to a Go type, People/Tags/External IDs are deferred
> as shared concepts, and this doc is independently derived rather than
> assuming any prior shape is correct.

Provider set is fixed by your earlier answer: **TMDB + TVDB + OMDb**, since
there is no real public IMDb API — OMDb is the practical way to carry IMDb
IDs/ratings. As with AfterDark, there is no dedicated pre-reset data model
doc to compare against; the closest prior art is the generic schema in
[`v1/data-model.md`](v1/data-model.md) and the recovered
`docs/project/content-types.md` (see Section 4), which already documents
movies as a **collapsed hierarchy** (no group level) — a claim this pass
tests directly against live provider data in Section 4.

---

## 1. Providers covered

| Source | Auth | `.env` key(s) | Role |
|---|---|---|---|
| TMDB | Bearer token (v4 read-access token) | `PURSER_SOURCES_TMDB_ENABLED`, `PURSER_SOURCES_TMDB_API_KEY` | Primary: search, full detail, cast/crew, external IDs, certifications, franchise/collection grouping, images |
| OMDb | `apikey` query param | `PURSER_SOURCES_OMDB_ENABLED`, `PURSER_SOURCES_OMDB_API_KEY` | Secondary: aggregates IMDb/Rotten Tomatoes/Metacritic ratings by IMDb ID — the practical substitute for a real IMDb API |
| TVDB | API key exchanged for a JWT via `/login` | `PURSER_SOURCES_TVDB_ENABLED`, `PURSER_SOURCES_TVDB_API_KEY` | Secondary/cross-reference: TV-primary provider that also covers movies; useful mainly for its `remoteIds` cross-reference and alternate cast/company data, not as the primary movie source |

All three keys are live in `.env` (added earlier this session) and were
exercised live during this research pass against *Fight Club* (TMDB id
`550`, IMDb id `tt0137523`, TVDB id `247`) to get real response shapes
rather than relying on documentation alone.

---

## 2. Provider data shapes

### TMDB

**Base URL:** `https://api.themoviedb.org/3/`. **Auth:** `Authorization:
Bearer {v4 read-access token}` — confirmed working live; the response is
served through TMDB's own CloudFront/openresty gateway (verified via
response headers), not a proxy or mock.

`GET /search/movie?query=` (live-verified):
```
page, total_results, total_pages,
results[]{ id, title, original_title, original_language, overview,
           adult, softcore, video, popularity, vote_average, vote_count,
           genre_ids[], release_date, poster_path, backdrop_path }
```
**`softcore` is a genuinely new field**, not previously documented anywhere
in this project's history — a boolean sibling to `adult`, presumably a
finer-grained content classification TMDB added since v1 was last touched.
Worth flagging for the cross-module synthesis phase: if TMDB now
distinguishes `adult`/`softcore` at the movie level, that's a potential
signal for routing content between the Movies and AfterDark modules, not
just a metadata curiosity.

`GET /movie/{id}?append_to_response=credits,external_ids,release_dates,keywords,videos,images`
(live-verified, full detail):
```
id, title, original_title, original_language, overview, tagline, status,
release_date, runtime, budget, revenue, homepage,
genres[]{id, name}, origin_country[], production_countries[],
spoken_languages[], production_companies[]{id, name, logo_path,
                                            origin_country},
belongs_to_collection{id, name, poster_path, backdrop_path},  -- nullable
external_ids{ imdb_id, wikidata_id, facebook_id, instagram_id, twitter_id },
credits{
  cast[]{ id, name, character, order, cast_id, credit_id,
          known_for_department, profile_path, popularity, gender },
  crew[]{ id, name, department, job, credit_id, profile_path,
          known_for_department, popularity, gender }
},
release_dates{ results[]{ iso_3166_1, release_dates[]{ certification,
               release_date, type, note, descriptors[] } } },
keywords{ keywords[]{id, name} }
```
`release_dates.results[].release_dates[].type` is a numeric MPAA-style
release-type code (premiere/theatrical-limited/theatrical/digital/
physical/TV), keyed per-country — meaning "the certification" for a movie
isn't one value, it's a matrix of country × release-type, and a `Movie`
model needs a policy for which cell to surface as *the* rating (this
project's locale, most restrictive, theatrical-only, etc.) rather than
storing one string.

`belongs_to_collection` (live-verified against *The Matrix*, TMDB id `603`
→ "The Matrix Collection"): a real franchise/series grouping concept,
distinct from `genres`. This directly challenges the "movies are a
collapsed hierarchy with no group level" assumption from prior art — see
Section 4.

### OMDb

**Base URL:** `https://www.omdbapi.com/`. **Auth:** `apikey` query param.
Looked up by IMDb ID (`i=`) or title (`t=`); live-verified against
`tt0137523`:
```
Title, Year, Rated, Released, Runtime, Genre, Director, Writer, Actors,
Plot, Language, Country, Awards, Poster,
Ratings[]{ Source, Value },   -- "Internet Movie Database", "Rotten Tomatoes", "Metacritic"
Metascore, imdbRating, imdbVotes, imdbID, Type, DVD, BoxOffice, Production,
Website, Response
```
OMDb's `Ratings[]` array is its whole reason to exist for this module: it's
the only source in this set that aggregates three independent rating
providers by IMDb ID in one call. Everything else in its response (cast as
a flat comma-joined `Actors` string, `Genre` as a comma-joined string) is
strictly worse-structured than TMDB's equivalent fields — OMDb should be
treated as a ratings enrichment source keyed by `imdb_id`, not a primary
detail source, mirroring how TheAudioDB/fanart.tv were secondary to
MusicBrainz in the Music module.

### TVDB

**Base URL:** `https://api4.thetvdb.com/v4/`. **Auth:** two-step — POST
`/login` with the API key to get a short-lived JWT, then `Authorization:
Bearer {jwt}` on subsequent calls. Live-verified end to end.

`GET /search?query=&type=movie`:
```
data[]{ objectID, id, tvdb_id, name, extended_title, slug, type,
        primary_type, year, first_air_time, overview, overviews{<lang>: ...},
        aliases[], genres[], studios[], director, country, status,
        primary_language, image_url }
```

`GET /movies/{id}/extended` (live-verified, full detail):
```
id, name, slug, year, runtime, status, score, budget, boxOffice, boxOfficeUS,
originalCountry, originalLanguage, first_release{...}, releases[]{country,
  date, detail}, genres[]{id, name}, studios[]{id, name},
companies{ studio[], network[], production[]{id, name, companyType{...},
           parentCompany{...}}, distributor[], special_effects[] },
contentRatings[]{ id, name, country, description, contentType, fullname },
characters[]{ id, name, peopleId, personName, peopleType, sort, isFeatured,
              image, personImgURL, movieId },
remoteIds[]{ id, type, sourceName },   -- e.g. sourceName "IMDB", "TheMovieDB.com", "Facebook"
artworks[], trailers[], awards[], inspirations[], tagOptions[]
```
**`remoteIds` cross-references both IMDb and TMDB directly** (live-verified:
Fight Club's TVDB record links to `tt0137523` and TMDB id `550`) — the same
kind of cross-provider convergence found in AfterDark (StashDB ↔ ThePornDB
performer URLs) and implicitly relied on in Music (everything joined by
MBID). This means identity reconciliation across all three movie providers
doesn't require independent fuzzy-matching — TVDB's `remoteIds` and TMDB's
`external_ids.imdb_id` together are enough to link all three records for
the same movie without a matching heuristic.

`characters[]` is TVDB's cast model — flatter than TMDB's `credits.cast`
(no `order`/`cast_id`/`credit_id` distinction, just a `sort` field and an
`isFeatured` boolean) and crew isn't separated from cast the way TMDB does
it (`peopleType` distinguishes "Actor" from other types, but the extended
response shown here only returned actors — crew shape not confirmed live
in this pass).

`contentRatings[]` is TVDB's certification model — one flat list per
country/`contentType`, simpler than TMDB's per-release-type matrix but
covering less nuance (no premiere-vs-theatrical-vs-digital distinction).

`companies{}` splits studio/network/production/distributor/special_effects
into separate buckets with a `parentCompany` hierarchy — richer than TMDB's
flat `production_companies[]` list, structurally similar to AfterDark's
Studio parent/child hierarchy and Music's nothing-comparable (Music had no
company/label hierarchy beyond a flat `Label` string on `MusicRelease`).

---

## 3. Proposed modern entity model

Conceptual, not Go structs — same constraint as the other two module docs.
People, Tags, and External IDs deferred to the shared-domain synthesis
phase.

### Movie

| Attribute | Notes |
|---|---|
| Title, original title, original language | TMDB and TVDB both provide this identically |
| Overview/plot, tagline | TMDB has both; OMDb only has `Plot`; TVDB has multi-language `overviews` |
| Release date(s) | Not one date — TMDB's `release_dates` is a country × release-type matrix; a single "release date" for display purposes needs an explicit selection policy (see Open Questions) |
| Runtime, budget, revenue/box office | All three providers carry these; TMDB and TVDB both had populated values live, OMDb only has `Runtime`/`BoxOffice` as display strings, not structured numbers |
| Certification/content rating | Same matrix problem as release dates — TMDB's per-country/per-type list vs. TVDB's flatter per-country list vs. OMDb's single `Rated` string (US-centric, no locale) |
| Genres | All three providers return genre lists; no per-provider genre-ID overlap confirmed (TMDB genre IDs are TMDB-specific ints, TVDB has its own) — genres should land in shared **Tags**, not a dedicated field, consistent with Music/AfterDark |
| Collection/franchise | **New concept, not in prior art** — TMDB's `belongs_to_collection`. See Section 4 for why this challenges the "movies have no group level" assumption |
| Cast/crew | Deferred to shared **People**, same `ItemPerson`-style credit mechanism as Music tracks and AfterDark scenes — role vocabulary here would include `actor`/`director`/`writer`/`producer` etc., sourced from TMDB's richer `department`/`job` crew breakdown as the primary source (TVDB's flatter `characters[]` as fallback/cross-check) |
| Production companies/studios | TMDB flat list vs. TVDB's typed, hierarchical `companies{}` — if a Studio concept from AfterDark (parent/child hierarchy) generalizes, TVDB's shape fits it better than TMDB's |
| External identity | TMDB ID (primary anchor, since it's the richest source), IMDb ID (via TMDB's own `external_ids.imdb_id` **or** TVDB's `remoteIds` — both agree, confirmed live), TVDB ID |
| Ratings/scores | OMDb's `Ratings[]` (IMDb/RT/Metacritic) is the only cross-aggregator source; TMDB's `vote_average`/`vote_count` and TVDB's `score` are single-provider scores, not aggregated |

### Deferred to shared domain (not designed here)

- **People** — cast/crew credits, same generic mechanism as Music and
  AfterDark. Role vocabulary is richer here (TMDB's `known_for_department`/
  `job` gives fine-grained crew roles beyond a fixed enum) — worth deciding
  whether the shared role vocabulary needs to be open-ended (free string)
  rather than a fixed enum, given how much more varied movie crew roles are
  than music/scene credits.
- **Tags** — genres from all three providers, keywords from TMDB
  (`keywords.keywords[]`, a much larger, more granular vocabulary than
  genre — closer to AfterDark's StashDB scene-tags than to Music's coarse
  genre/mood/style fields).
- **External IDs** — `(entity_type, entity_id, source, external_id)`, same
  shape as the other two modules, extended with `tmdb`, `tvdb` as `source`
  values (both already anticipated in v1's `external_ids.source` enum).
- **Collection/Franchise** — genuinely new. Doesn't map cleanly onto
  either Music's `Group` (release-group, one artist's own body of work) or
  AfterDark's Studio hierarchy (a company, not a body of related works
  across companies). Flagged as its own open question below rather than
  forced into an existing shape.

---

## 4. Comparison to prior snapshot

`docs/project/content-types.md` (recovered via
`git show 8ae1ac2^:docs/project/content-types.md`, same source used for the
AfterDark doc) stated: *"Movies are collapsed: `kind=movie` in
`library_entries` is the movie itself. One `item` record is auto-created as
its leaf... No manual group creation."* — i.e. zero group level, unlike
TV (season) or Music (release-group/album).

This pass's live TMDB research finds a real, populated
`belongs_to_collection` field (Fight Club has none, but *The Matrix* → "The
Matrix Collection" does) that doesn't fit a groupless model. This is a
genuine tension worth surfacing explicitly rather than silently resolving
in either direction:

- **Keep movies collapsed, ignore collections** (matches prior art exactly,
  simplest): a franchise becomes a cross-reference concept at most (e.g. a
  `Tag` or a loose "related titles" list), not a real hierarchy level.
- **Add an optional group level for movies** (breaks prior art, more
  structurally honest to what TMDB actually models): a `Group` becomes
  "Collection" for movies the same way it's "Album" for music and "Season"
  for TV — but most movies have no collection (`belongs_to_collection` is
  nullable and empty for the majority of catalog titles), so this would be
  an *optional* group level, not a mandatory one like TV seasons.

Not resolved here — this is exactly the kind of cross-module structural
question (does "optional group" as a concept generalize, or is Movies
special) that belongs in the synthesis phase once TV's model is also
documented, since TV's Season group is mandatory-ish and Movies' Collection
group would be rare-and-optional, which may argue they're different enough
to warrant different treatment rather than one shared `Group` mechanism
stretched to cover both.

Otherwise no disagreement with prior art: `Cast` as the role terminology
(already in `item_people.role` enum in `v1/data-model.md`), `tmdb`/`tvdb`
already anticipated in the `external_ids.source` enum.

---

## 5. Open questions

1. **Collection/franchise grouping** — see Section 4. Genuinely undecided;
   needs the TV module's Season model alongside it before making a call on
   whether "optional group" is one shared mechanism or two different ones.
2. **Which release date / certification to surface as "the" value** — TMDB
   models both as a country × type matrix, TVDB as a flatter per-country
   list, OMDb as one US-centric string. A policy is needed (e.g. "US
   theatrical release date, fall back to earliest known release" /
   "US certification, fall back to first available") — not a data-model
   question exactly, but the entity model can't have a single
   `ReleaseDate`/`Certification` field without one.
3. **TMDB `adult`/`softcore` fields** — new, undocumented anywhere in prior
   art. Is this a genuine signal for cross-module routing (a title TMDB
   flags `softcore` might belong in AfterDark instead of Movies), or purely
   informational? Needs a product decision, not just a data-model one.
4. **Crew role vocabulary** — fixed enum (matches the pattern used for
   Music/AfterDark roles) vs. open string sourced directly from TMDB's
   `job` field (which has far more distinct values than either other
   module's role set, e.g. "Director of Photography", "Casting", "Sound
   Re-Recording Mixer"). Forcing these into a small fixed enum loses
   information; keeping them open-ended breaks the "role vocabulary lives
   in adapter/config, not domain" pattern's assumption of a bounded set.
5. **Primary-provider choice** — TMDB is clearly the richest single source
   here (unlike Music, where MusicBrainz's role was identity/IDs and images
   came from elsewhere entirely). Should OMDb and TVDB both be pure
   enrichment layers on top of a TMDB-anchored identity, mirroring
   MusicBrainz's role in Music, rather than three co-equal sources? Live
   research supports yes (TVDB's `remoteIds` and TMDB's `external_ids` both
   agree once you have one ID) but this is a design decision, not just an
   observation.
