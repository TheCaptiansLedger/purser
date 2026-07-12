# TV Module — Data Model Proposal

> Status: proposal, not yet an ADR-backed decision. Fourth module in the
> per-module documentation pass (after [Music](music-data_model.md),
> [AfterDark](afterdark-data_model.md), and [Movies](movies-data_model.md)),
> same template and ground rules: nothing here commits to a Go type,
> People/Tags/External IDs are deferred as shared concepts, and this doc is
> independently derived rather than assuming any prior shape is correct.

Same provider set as Movies — **TMDB + TVDB + OMDb** — but TVDB is TV's home
turf (TheTVDB predates and was purpose-built for TV episode tracking, unlike
its bolted-on movie coverage seen in the Movies doc), so its role here is
closer to primary than secondary. Live research below is against *Breaking
Bad* (TMDB id `1396`, TVDB id `81189`, IMDb id `tt0903747`).

---

## 1. Providers covered

| Source | Auth | `.env` key(s) | Role |
|---|---|---|---|
| TMDB | Bearer token | `PURSER_SOURCES_TMDB_ENABLED`, `PURSER_SOURCES_TMDB_API_KEY` | Series/season/episode detail, cast (per-episode and aggregate), content ratings, keywords, images |
| TVDB | API key → JWT via `/login` | `PURSER_SOURCES_TVDB_ENABLED`, `PURSER_SOURCES_TVDB_API_KEY` | TV-native provider: multiple parallel episode-numbering schemes (aired/DVD/absolute order), richer company hierarchy, cross-references IMDb + TMDB directly |
| OMDb | `apikey` query param | `PURSER_SOURCES_OMDB_ENABLED`, `PURSER_SOURCES_OMDB_API_KEY` | Ratings aggregation, at **both** series and individual-episode granularity — each episode has its own IMDb ID, distinct from the series' |

All three verified live this pass.

---

## 2. Provider data shapes

### TMDB

`GET /search/tv?query=` — same shape as movie search (`adult`, `softcore`,
`popularity`, etc.), confirming the `softcore` field flagged in the Movies
doc is not movie-specific — it's a general TMDB content-classification
field present on TV too.

`GET /tv/{id}?append_to_response=external_ids,content_ratings,aggregate_credits,keywords`
(live-verified, full series detail):
```
id, name, original_name, overview, tagline, status, type,
first_air_date, last_air_date, in_production,
number_of_seasons, number_of_episodes, episode_run_time[],
languages[], origin_country[], spoken_languages[],
genres[]{id, name}, networks[]{id, name, logo_path, origin_country},
production_companies[]{...}, production_countries[],
created_by[]{id, name, credit_id, profile_path, gender},
seasons[]{ id, season_number, name, overview, air_date, episode_count,
           poster_path, vote_average },
last_episode_to_air{...}, next_episode_to_air{...},
external_ids{ imdb_id, tvdb_id, tvrage_id, freebase_mid, freebase_id,
              wikidata_id, facebook_id, instagram_id, twitter_id },
content_ratings{ results[]{ iso_3166_1, rating, descriptors[] } },
aggregate_credits{
  cast[]{ id, name, roles[]{credit_id, character, episode_count},
          total_episode_count, order, known_for_department, ... }
},
keywords{ results[]{id, name} }
```
Two findings worth calling out:

1. **`external_ids.tvdb_id` is returned directly by TMDB** — series-level
   identity convergence between TMDB and TVDB is even stronger here than in
   Movies (where TVDB's `remoteIds` had to supply the TMDB link one-way;
   here TMDB supplies the TVDB link too, both directions confirmed live).
2. **`aggregate_credits` is a genuinely new shape**, not present in the
   Movies doc: instead of one character per cast member, each cast member
   has a `roles[]` array (a recurring actor can have played more than one
   named character across the show's run) plus `total_episode_count`. This
   doesn't fit the Movies module's flat "one credit, one character" cast
   model — TV credit is inherently aggregate-across-episodes, not a
   per-title fact.

`GET /tv/{id}/season/{n}?append_to_response=credits`
(live-verified, season 1):
```
_id, season_number, name, overview, air_date, poster_path, networks[],
episodes[]{ id, episode_number, season_number, name, overview, air_date,
            runtime, episode_type, production_code, still_path,
            vote_average, vote_count, show_id,
            crew[]{...}, guest_stars[]{... same shape as movie cast credit} }
```
`episode_type` (values seen: `"standard"`; TMDB's public docs also define
`"finale"`, `"mid_season"`, `"season_premiere"`) is new — no equivalent in
Movies or Music. `guest_stars[]` on each episode is distinct from the
series-level `aggregate_credits` — the same actor can appear both as a
recurring aggregate-credit member *and* have specific per-episode
guest-star entries; these aren't the same list at different granularity,
they're two separate TMDB concepts.

### TVDB

`GET /series/{id}/extended` (live-verified):
```
id, name, slug, year, status, score, averageRuntime, overview,
firstAired, lastAired, nextAired, airsDays, airsTime,
originalCountry, originalLanguage, latestNetwork, originalNetwork,
genres[]{id, name}, tags[], aliases[],
companies{ studio[], network[], production[], distributor[],
           special_effects[] },   -- same shape as the Movies doc's companies{}
contentRatings[]{ id, name, country, description, contentType },
characters[]{ id, name, peopleId, personName, peopleType, sort,
              isFeatured, seriesId, episodeId, image },
remoteIds[]{ id, type, sourceName },  -- confirmed: IMDB, TheMovieDB.com,
                                       -- "TMS (Zap2It)", Official Website
seasonTypes[]{ id, name, type, alternateName },
seasons[]{ id, seriesId, number, type{...}, image, companies{...} },
artworks[], trailers[], lists[], nameTranslations[], overviewTranslations[]
```
**`seasonTypes` is the single biggest structural finding in this module.**
Breaking Bad's live response returned three parallel numbering schemes:
`"Aired Order"` (`type: "official"`), `"DVD Order"` (`type: "dvd"`), and
`"Absolute Order"` (`type: "absolute"`). Every season and episode in TVDB
belongs to one of these types, and the *same underlying episode* can have a
different season/episode number depending on which order is requested —
confirmed via `GET /series/{id}/episodes/{seasonType}?season=N`, which
takes the order type as a path segment. TMDB has no equivalent — it exposes
exactly one `seasons[]`/episode-number scheme per series. This matters a
lot for anime and any show that's been re-packaged for DVD/streaming with
different episode boundaries than its original broadcast order — a purely
TMDB-sourced model would silently pick one ordering (whichever TMDB
happens to use) with no way to represent the alternative.

`GET /series/{id}/episodes/official?season=1` (live-verified, aired order):
```
data{ series{...}, episodes[]{ id, seriesId, name, aired, runtime,
      number, seasonNumber, absoluteNumber, isMovie, finaleType, year,
      seasons[] -- cross-reference to this same episode's number under
                   OTHER season types, when it differs
      overview, overviewTranslations[], nameTranslations[], image } }
```
`absoluteNumber` (continuous episode count ignoring season boundaries,
`1, 2, 3...` across the whole series) is a third numbering axis, distinct
from both aired-order season/episode and DVD order — the "Absolute Order"
season type surfaces this formally, but every episode carries an
`absoluteNumber` regardless of which season type was requested.

### OMDb

`GET ?i={imdbID}` for a series returns the same shape as a movie plus
`totalSeasons` and `Type: "series"`. **Episode-level lookup is a distinct
call**, live-verified: `GET ?i={seriesImdbID}&Season=1&Episode=1` returns a
full episode record with its **own** `imdbID` (`tt0959621`, distinct from
the series' `tt0903747`) and a `seriesID` field linking back. This mirrors
a pattern already seen in Music (v1 used a distinct `mbz_recording`
`ExternalIDSource` so recording-level MBIDs didn't collide with
artist/release-level ones) — TV needs the same treatment: episode-level
IMDb IDs are a different `external_ids` entity_type/source pairing than
series-level ones, not an extension of the same ID.

---

## 3. Proposed modern entity model

Conceptual, not Go structs. People/Tags/External IDs deferred to the
shared-domain synthesis phase, consistent with the other three modules.

### Series

| Attribute | Notes |
|---|---|
| Name, original name, overview, tagline, status, type | All three providers cover this; TMDB's `type` (Scripted/Reality/Documentary/etc.) and `status` (Returning Series/Ended/Canceled) have no TVDB equivalent found in this pass — worth keeping if useful for filtering |
| Air dates | `first_air_date`/`last_air_date`/`next_episode_to_air` (TMDB) vs. `firstAired`/`lastAired`/`nextAired` (TVDB) — same concept, agree |
| Networks/studios | TMDB flat `networks[]` list vs. TVDB's typed `companies{studio,network,production,distributor,special_effects}` — same asymmetry found in the Movies doc; TVDB's shape is richer |
| Content rating | Same multi-value problem as Movies (`content_ratings.results[]` per country) — needs the same surfacing policy decided there, not re-decided independently here |
| Genres, keywords/tags | Deferred to shared **Tags** — TMDB `genres`/`keywords`, TVDB `genres`/`tags` |
| Cast (aggregate) | TMDB's `aggregate_credits` — a person can have multiple `roles[]` (multiple named characters) with per-role `episode_count`. Deferred to shared **People**, but the role model needs to support "more than one credited role per person per series," which neither Music nor AfterDark needed |
| External identity | TMDB ID, TVDB ID (confirmed cross-referenced both directions), IMDb ID — all three converge without fuzzy matching, same as Movies |

### Season

Matches prior art directly (TV already has a mandatory-ish group level per
`content-types.md` — see Section 4), but with one addition TVDB surfaces
that prior art didn't anticipate:

| Attribute | Notes |
|---|---|
| Number, name, overview, air date, episode count | Standard, both providers agree |
| Season 0 / "Specials" | Both TMDB and TVDB use `season_number`/`number = 0` for non-canon/special episodes — consistent convention, no conflict |
| **Numbering scheme** | **New, TVDB-only.** A season doesn't just have a number — it has a number *within a `seasonType`* (Aired/DVD/Absolute). If TVDB is a source, "Season 1" is ambiguous without also specifying which order. See Open Questions for whether this needs first-class modeling or can be ignored (aired order only) for a first cut |

### Episode

| Attribute | Notes |
|---|---|
| Number, season number, name, overview, air date, runtime | Standard on both providers |
| Episode type | TMDB-only (`standard`/`finale`/`mid_season`/`season_premiere`) — informational, not identity-relevant |
| Absolute number | TVDB-only — continuous count ignoring season boundaries, relevant for anime-style continuous-numbering libraries |
| Guest stars (per-episode) | Distinct from series-level aggregate cast — deferred to shared **People**, but needs to coexist with the series-level aggregate credit for the same person without being a duplicate concept |
| External identity | TMDB episode ID, TVDB episode ID, **and a separate per-episode IMDb ID from OMDb** — confirmed distinct from the series' IMDb ID, same `entity_type`-scoping pattern Music used for recording-level MBIDs |

### Deferred to shared domain (not designed here)

- **People** — cast/crew/guest-stars, same generic mechanism as the other
  three modules, but TV is the first module where a single title needs
  *both* a series-level aggregate role concept *and* a per-episode credit
  concept for what might be the same person. Whether that's one shared
  mechanism with two granularities or something new is a synthesis-phase
  question.
- **Tags** — genres/keywords, same shape question as Movies (TMDB
  keywords are much more granular than genre).
- **External IDs** — same `(entity_type, entity_id, source, external_id)`
  shape as the other modules; TV is the module that most clearly needs
  `entity_type` to distinguish series-level from episode-level IDs for the
  *same* source (`tmdb`, `tvdb`, and now confirmed `imdb`/OMDb-derived too).

---

## 4. Comparison to prior snapshot

`docs/project/content-types.md` (recovered, same source used for AfterDark
and Movies) already modeled TV as `entry → group (Season) → item
(Episode)`, `Cast` as the person role — unlike Movies, this is **not** in
tension with live provider data. TMDB and TVDB both model Season as a real,
populated, near-universal grouping (unlike Movies' rare/optional
Collection), so prior art's mandatory-group assumption holds for TV. This
directly resolves the open question left in the Movies doc: Movies'
Collection and TV's Season are **not** the same kind of "optional group" —
Season is close to mandatory (virtually every series has at least one),
Collection is rare (most movies have none). That's real evidence they
shouldn't be forced into one identical mechanism, even though both are
conceptually "a group between the top-level entry and the leaf item."

What prior art did **not** anticipate, because it predates any TVDB
research this deep: TVDB's multiple parallel numbering schemes
(`seasonTypes`: aired/DVD/absolute). The v1 general schema's
`groups.number INTEGER` (a single int) has no room for "season 1 in aired
order is a different set of episodes than season 1 in DVD order." This is
new information from this pass, not a revision of something v1 got wrong —
v1 never went deep enough into a TV provider to encounter it (v1's frozen
snapshot only covered Music).

---

## 5. Open questions

1. **Multiple episode-numbering schemes (TVDB `seasonTypes`)** — support
   only aired order for a first cut (simplest, matches what basically every
   other module implicitly assumes: one canonical numbering), or model it
   properly from the start so anime/reordered content isn't a second-class
   citizen later? This is the single most consequential open question this
   doc raises — it affects whether `Group.number` can stay a plain int or
   needs a `(number, scheme)` pair.
2. **Series-level aggregate credit vs. per-episode guest-star credit** —
   are these the same `EntryPerson`/`ItemPerson`-style mechanism at two
   granularities (entry-level = aggregate, item-level = per-episode, which
   actually maps cleanly onto v1's existing `entry_people` vs. `item_people`
   split — "regular cast" is explicitly called out as an `entry_people`
   use case in `v1/data-model.md`), or does aggregate credit need its own
   concept? Initial read: this maps onto the *existing* `entry_people` /
   `item_people` split already in the shared schema with zero new
   mechanism needed — flagged as a question rather than a certainty
   because it wasn't verified against how Music or AfterDark actually used
   that split in practice.
3. **Per-episode IMDb ID handling** — same shape as Music's
   `mbz_recording` distinct-source pattern; does the shared `ExternalID`
   concept need an explicit `entity_type=item` scoping convention
   documented once, rather than rediscovered per module? (v1 already had
   `entity_type` on `external_ids`, so this may already be solved — worth
   confirming in the synthesis phase rather than assuming.)
4. **TMDB `softcore` field, now confirmed on both Movies and TV** — same
   open question as raised in the Movies doc, now with more evidence it's
   a general TMDB classification, not a movie-specific one.
5. **Content rating surfacing policy** — same unresolved question as
   Movies (per-country matrix vs. one displayed value); should be decided
   once, not independently per module, since the shape of the problem is
   identical in both.
