# Music Module — Data Model Proposal

> Status: proposal, not yet an ADR-backed decision. This is step 1 of the plan
> described in the conversation that produced it: document the module in
> isolation first, compare against prior art, and defer the actual shared
> domain design to a later cross-module synthesis pass. Nothing here commits
> to a Go type — see "Proposed modern entity model" for why.

Cross-reference: [`docs/technical/v1/music-data-model.md`](v1/music-data-model.md)
and [`docs/technical/v1/metadata-providers.md`](v1/metadata-providers.md) are
the frozen pre-reset snapshot, explicitly marked descriptive-not-prescriptive.
That snapshot was built directly from the old adapter source, not
recollection, so its field-level claims are trustworthy as *historical fact*.
This document treats it as a strong starting reference, re-verifies the
provider schemas live against the current APIs, and proposes a model
independently rather than assuming the old shape is still correct.

---

## 1. Providers covered

| Source | Auth | `.env` key(s) | Role | New vs. v1? |
|---|---|---|---|---|
| MusicBrainz | None (User-Agent required) | `PURSER_SOURCES_MUSICBRAINZ_ENABLED`, `PURSER_SOURCES_MUSICBRAINZ_USER_AGENT` | Identity graph: artist/release-group/release/recording IDs and relationships. The join key for every other source. | Same role as v1. |
| TheAudioDB | API key in URL path | `PURSER_SOURCES_THEAUDIODB_ENABLED`, `PURSER_SOURCES_THEAUDIODB_API_KEY` | Primary image source (artist art, album covers), plus biography/genre/social-link text. | Same role as v1, richer fields available now (verified live). |
| fanart.tv | `api_key` query param | `PURSER_SOURCES_FANART_ENABLED`, `PURSER_SOURCES_FANART_API_KEY` | Secondary/fallback image source. | Same role as v1. |
| Last.fm | API key + shared secret | `PURSER_SOURCES_LASTFM_ENABLED`, `PURSER_SOURCES_LASTFM_API_KEY`, `PURSER_SOURCES_LASTFM_SHARED_SECRET` | **New real candidate.** Tags/genre consensus, similar-artist graph, play-count popularity signal, long-form bio. | v1 explicitly had **no** Last.fm adapter — a Last.fm URL was only ever captured as a string from an MBZ relation. `.env` now provisions a real key+secret, which is a signal (not yet a decision) that this rebuild intends to integrate it for real. Flagged as an open question below. |
| AcoustID | Client API key | `PURSER_SOURCES_ACOUSTID_ENABLED`, `PURSER_SOURCES_ACOUSTID_API_KEY` | Acoustic fingerprint → MusicBrainz recording/release-group resolution. | v1 used AcoustID only as a *file-tag value* (`MediaFile.Metadata["acoustid"]`, a confidence signal), never called the lookup API as a real identification source. `.env` now provisions a real key, same signal as Last.fm. |

Note: `PURSER_SOURCES_ACOUSTID_*` exists in the live `.env` but is not yet
documented in `.env.example` — a pre-existing gap, out of scope for this doc
per the plan (no `.env`/`.env.example` edits here), but worth fixing whenever
config is next touched.

All five keys above are live and enabled in this repo's `.env`. The schema
claims below were re-verified with live requests against MusicBrainz,
TheAudioDB, fanart.tv, and Last.fm during this research pass (noted inline as
"live-verified"); AcoustID's contract is documented from its public API spec
only, since exercising it meaningfully requires a real audio fingerprint, not
just a key.

---

## 2. Provider data shapes

### MusicBrainz

**Base URL:** `https://musicbrainz.org/ws/2/`. **Auth:** none, but a
descriptive `User-Agent` is required by MusicBrainz policy. **Rate limit:** 1
req/sec, MusicBrainz-enforced (v1's client-side limiter matched this).

Endpoints and response shapes below are unchanged in role from v1; only new
fields found in live verification are called out.

- `GET /artist/{mbid}?inc=url-rels+aliases&fmt=json` (live-verified against
  The Beatles, `b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d`):
  ```
  id, name, sort-name, disambiguation, type ("Group"|"Person"|"Orchestra"|"Choir"|"Character"|"Other"),
  country, area{name, iso-3166-1-codes}, begin-area{name}, life-span{begin,end,ended},
  isnis[], ipis[], gender, gender-id,
  aliases[]{name, sort-name, locale, type, primary, begin, end, ended},
  relations[]{type, direction, url{resource}, artist{id,name}}
  ```
  Matches v1's documented shape field-for-field; `gender`/`gender-id`/`ipis`
  were present but not called out in v1 (they were presumably unused, not
  absent).

- `GET /release-group?artist={mbid}&limit=N&fmt=json` — discography by
  release-group (live-verified). Returns `id, title, primary-type,
  first-release-date` per entry — matches v1.

- `GET /release/{mbid}?inc=recordings+labels&fmt=json` — release detail
  (live-verified against *Please Please Me*,
  `ade577f6-6087-4a4f-8e87-38b0f8169814`):
  ```
  id, title, status, country, date, barcode, asin, disambiguation,
  packaging, quality, text-representation,
  label-info[]{label{name}, catalog-number},
  release-events[]{date, area},
  cover-art-archive{artwork(bool), front(bool), back(bool), darkened(bool), count(int)},
  media[]{id, position, format, track-count, track-offset,
           tracks[]{id, position, number(string), title, length,
                     recording{id, title, length, first-release-date, disambiguation, video}}}
  ```
  **Finding not present in v1's documentation:** the `cover-art-archive`
  object is returned inline on every release response and tells you whether
  Cover Art Archive actually has artwork (`front: true/false`, `count`)
  *before* constructing the derived image URL. v1's adapter derived
  `https://coverartarchive.org/release/{mbid}/front-250` blindly with no
  existence check. This is a concrete, low-risk improvement candidate for the
  new adapter — see Open Questions.

### TheAudioDB

**Base URL:** `https://www.theaudiodb.com/api/v1/json/{key}/`. **Auth:** key
as a URL path segment. **Live-verified** against artist-mb.php for The
Beatles using both the historic free-tier key `123` and key `2` — **both
still work** as of this research pass, returning identical data. This repo's
`.env` has a distinct personal key configured.

`GET {key}/artist-mb.php?i={mbid}` response fields, live-verified — richer
than v1 documented:
```
idArtist, strArtist, strArtistAlternate, strMusicBrainzID, strISNIcode,
strGender, strCountry, strCountryCode, strGenre, strMood, strStyle,
strLabel, intFormedYear, strDisbanded, intBornYear, intDiedYear,
intMembers, intCharted, intFollowers, intPopularity,
strBiography[EN|DE|FR|...] (13 locales),
strArtistThumb, strArtistLogo, strArtistClearart, strArtistCutout,
strArtistWideThumb, strArtistBanner,
strArtistFanart, strArtistFanart2, strArtistFanart3, strArtistFanart4,
strWebsite, strFacebook, strTwitter, strLastFMChart, strLocked
```
v1 only consumed `strArtist, strMusicBrainzID, strBiographyEN,
strArtistThumb, strArtistFanart[1-3], strArtistBanner, strWebsite,
strGenre`. Fields available but unused by v1 that are directly relevant to a
"modern" model: `strArtistLogo`/`strArtistClearart`/`strArtistCutout` (extra
image slots), `intFormedYear`/`intBornYear`/`intDiedYear`/`strDisbanded`
(v1 stored equivalent dates as free-text `Metadata` strings sourced from MBZ
life-span instead — TheAudioDB gives year-only ints, a coarser but redundant
second source), `intMembers`/`intPopularity`/`intFollowers` (new signals,
not in v1 at all), `strFacebook`/`strTwitter` (v1 only captured Last.fm/
Wikipedia/homepage URLs from MBZ relations, not social links from
TheAudioDB).

`GET {key}/album-mb.php?i={releaseGroupMBID}` — unchanged from v1:
`album[]{strMusicBrainzID, strAlbumThumb}`.

### fanart.tv

**Base URL:** `https://webservice.fanart.tv/v3/`. **Auth:** `api_key` query
param. Live-verified against `music/{mbid}` for The Beatles:
```
{
  name, mbid_id,
  artistthumb[]{id, url, likes}, artistthumb_count,
  artistbackground[]{...}, artistbackground_count,
  hdmusiclogo[]{...}, hdmusiclogo_count,
  musiclogo[]{...}, musiclogo_count,        -- not documented in v1
  musicbanner[]{id, url, likes}, musicbanner_count,
  albums: {
    "<release-group-mbid>": {
      albumcover[]{id, url, likes}, albumcover_count
      -- cdart[] present on some albums, absent on others (not on any of
         The Beatles' albums checked in this pass)
    }
  }
}
```
`musiclogo` is a field not mentioned in the v1 snapshot at all — fanart.tv
appears to have added it (or v1's adapter simply never mapped it, same as
`hdmusiclogo`/`cdart`). Otherwise matches v1: one endpoint serves artist art
and every album's art in one call, keyed by release-group MBID.

### Last.fm — new for this pass

**Base URL:** `https://ws.audioscrobbler.com/2.0/`. **Auth:** `api_key`
query param; a shared secret exists for write/session methods this module
would not need (read-only metadata use). Live-verified,
`artist.getinfo?mbid={mbid}&format=json`:
```
{
  name, mbid, url, streamable, ontour,
  image[]{#text, size},   -- Last.fm has deprecated/blanked most image URLs; treat as unreliable
  stats{listeners, playcount},
  similar{ artist[]{name, url, image[]} },
  tags{ tag[]{name, url} },
  bio{ links, published, summary, content }
}
```
Two things worth noting for the entity model: `stats.listeners`/`playcount`
are a genuine popularity signal nothing else in this stack provides, and
`tags.tag[]` is free-form crowd-sourced folksonomy (e.g. "classic rock",
"60s", "british") — a plausible additional `Tag` source distinct from
MusicBrainz genre relationships, but noisier and non-authoritative.

### AcoustID — new for this pass, documented not live-tested

**Base URL:** `https://api.acoustid.org/v2/lookup`. **Auth:** client API key
as a query param. Contract per public API spec (not live-verified — requires
a real Chromaprint fingerprint + duration, not exercisable from a doc-research
pass):
```
GET/POST ?client={key}&fingerprint={chromaprint}&duration={seconds}
         &meta=recordings+releasegroups+compress
→ { status, results[]{ id (AcoustID UUID), score (0-1),
                        recordings[]{ id (MBZ recording MBID), title,
                                      releasegroups[]{id, title, type} } } }
```
This is a fundamentally different kind of provider from the other four: it
takes *audio content* (a fingerprint computed from decoded PCM) as input, not
a name or MBID, and returns MusicBrainz IDs — i.e. it's an identification
source usable during import matching, not an enrichment source for an
already-identified artist/release. v1's `MusicConfidenceSignals.AcoustID`
treated it exactly this way (a matching signal, weight 0.35), just without
ever calling the real lookup API — the fingerprint was computed and stored,
but matching apparently relied on other signals. Whether to build a real
AcoustID lookup adapter or keep the fingerprint-as-signal-only approach is an
open question below.

---

## 3. Proposed modern entity model

This section is deliberately conceptual — tables and prose, not Go structs —
per the plan's constraint that no shared domain shape has been decided yet.
Each conceptual entity below explicitly defers to **People** and **Tags** as
external shared concepts (not owned by this module) and to a generic
**External ID** concept for cross-provider linking, consistent with what v1
already did successfully (see Section 4).

### Artist

The MusicBrainz artist is the anchor identity. Conceptually:

| Attribute | Notes |
|---|---|
| Name, sort name, disambiguation | From MBZ |
| Artist kind | `person` \| `group` \| `orchestra` \| `choir` \| `character` \| `other` — drives which date-pair applies (born/died vs. formed/disbanded) |
| Aliases | From MBZ `aliases[]`, locale-filtered |
| Lifecycle dates | Born/died (person) or formed/disbanded (group) — MBZ `life-span` is the authoritative source; TheAudioDB's year-only ints are a fallback only, not a competing source of truth |
| External identity | ISNI, MBZ MBID (primary join key), TheAudioDB ID (== MBID), fanart.tv ID (== MBID), Last.fm URL/mbid |
| Popularity/social signals | Last.fm listeners/playcount, TheAudioDB followers/popularity/charted — explicitly informational, never used for identity matching |
| Members / solo self-link | Deferred to the shared **People** concept exactly as v1 did — no reason to change this, it worked and required zero shared-struct changes |
| Genre/mood/style | Deferred to the shared **Tags** concept, `scope=metadata`. Candidate sources: MBZ genre relations (authoritative, sparse), TheAudioDB `strGenre`/`strMood`/`strStyle` (single-value, less structured), Last.fm folksonomy tags (crowd-sourced, noisiest but richest) |

### Release Group (Album)

The conceptual work, independent of pressing/edition. Matches v1's role for
`Group` — nothing here argues for a different shape:

| Attribute | Notes |
|---|---|
| Title | |
| Primary/secondary type | `album`\|`single`\|`ep`\|`compilation`\|`live`\|`soundtrack`\|etc. from MBZ `primary-type`/`secondary-types` |
| First release date | Earliest known release in the group |
| External identity | MBZ release-group MBID (primary), fanart.tv keys albums by this same MBID |
| Cover art | TheAudioDB `album-mb.php` (priority candidate 1), fanart.tv `albums[rgMBID].albumcover` (candidate 2) — same two-source priority pattern as v1 |

### Release (specific pressing/edition)

Still a first-class new concept, same as v1's `MusicRelease` — a
release-group has one-to-many releases, and "what's actually on disk" only
makes sense at this granularity:

| Attribute | Notes |
|---|---|
| Title (edition-qualified) | e.g. "Please Please Me (2009 Remaster)" |
| Country, date, label, catalog number, barcode | From MBZ `release` |
| Format, medium count, track count | From MBZ `media[]` |
| Status | `Official` required to be a monitoring-default candidate |
| Cover art availability | **New, not in v1:** MBZ's `cover-art-archive{front, back, count}` inline flag lets the model know *before* fetching whether Cover Art Archive actually has art — v1's derive-a-URL-and-hope approach is avoidable now |
| External identity | MBZ release MBID |

### Recording (Track)

Same role as v1's `Item`-with-music-metadata — the leaf. No new entity
needed here either:

| Attribute | Notes |
|---|---|
| Title, length, position/number | From MBZ `media[].tracks[]` and `recording` |
| Disc number | From medium `position` |
| ISRC, composer, lyricist | From embedded file tags, not any provider — same as v1 |
| Featured/credited artists | Deferred to shared **People**/credit mechanism, same as v1's `ItemPerson` |
| External identity | MBZ recording MBID (distinct external-ID source from artist/release-level MBIDs, same reasoning v1 used) |
| Acoustic fingerprint | AcoustID fingerprint value, stored as file/media metadata regardless of whether a live AcoustID lookup adapter is built |

### Deferred to shared domain (not designed here)

- **People** — artist members, solo self-links, track credits. v1's approach
  (generic `Person` + role-tagged association, no music-specific struct
  fields) is the strongest candidate and should be validated against the
  other modules' docs before being finalized, not decided unilaterally here.
- **Tags** — genre/mood/style from any of the three tag-capable sources
  above (MBZ relations, TheAudioDB fields, Last.fm folksonomy), `scope`
  distinguishing user-applied vs. provider-sourced, matching v1's
  `(key, value, scope)` shape.
- **External IDs** — a generic `(entity_type, entity_id, source, external_id)`
  join, exactly as v1 had it, extended with `lastfm` and `acoustid` as new
  `source` values if those adapters are built for real this time.

---

## 4. Comparison to prior snapshot

Where this proposal agrees with v1 (documented above, not repeated): the
overall MBID-is-the-join-key philosophy, the release-group/release/recording
three-tier granularity, deferring people/tags/external-ids to shared
concepts with zero new fields on shared structs, and the two-tier image
source priority (TheAudioDB over fanart.tv).

Where it differs or adds:

1. **Last.fm and AcoustID as real candidate adapters**, not just a captured
   URL string and a stored-but-unused fingerprint. This is new because
   `.env` now provisions real credentials for both — v1 explicitly had no
   real Last.fm integration and never called the AcoustID lookup API. This
   is flagged as an open question, not a decision, below.
2. **Cover-art-existence checking via MBZ's inline `cover-art-archive`
   object**, instead of v1's blind URL derivation. Concrete, low-risk
   improvement, independent of any other open question.
3. **TheAudioDB has materially more usable fields today** than v1 consumed
   (extra image slots, formed/born/died years, popularity/follower counts,
   social links). Whether to consume them is a modeling decision for the
   entity table above (mostly: yes for image slots and social links, no for
   redundant date fields where MBZ already wins).
4. No disagreement found with v1's `MusicScanGroup`/`MusicTagSummary`/
   confidence-signal design for the import-matching pipeline — that
   machinery lives in the pipeline/acquisition core (per your point 9, "the
   core domain should be the pipeline"), not in this module's data model, so
   it's out of scope for this doc and deferred to whenever the pipeline core
   is documented.

---

## 5. Open questions

Left open for the cross-module synthesis phase or for explicit user decision
— not resolved unilaterally here:

1. **Build real Last.fm and AcoustID adapters, or keep them as v1 did
   (metadata-bag string / stored-but-unused fingerprint)?** The credentials
   existing in `.env` is a signal of intent, not a decision. If real
   adapters are wanted: Last.fm's tags/stats fit the existing `ImageSource`-
   style narrow-port pattern reasonably well (a new `TagSource` or
   `PopularitySource` capability port, ISP-style); AcoustID is structurally
   different (identification input is audio, not a name/MBID) and would
   need its own port shape, closer to `FileIdentifier` than `MetadataSource`.
2. **Tag provenance across three sources** (MBZ genre relations, TheAudioDB
   genre/mood/style, Last.fm folksonomy) — do all three feed the same
   `Tag(key, value, scope=metadata)` bucket with no source distinction, or
   does `Tag` need a `source` concept to let a user trust MBZ genres over
   Last.fm folksonomy? This is a shared-domain question, not music-specific,
   and should be raised again once other modules' tag sources are known.
3. **`ItemFilter` metadata-query gap** — v1 flagged that `ListTracksByRelease`
   worked around the lack of a generic metadata filter on the shared item
   port. Still unresolved; worth deciding early if/when ports are actually
   designed, per v1's own note and [0002 (ISP)](../adr/0002-solid-design-principles.md).
4. **How much of "modern" TheAudioDB data is worth the added surface area?**
   Popularity/follower counts and social links are new capability, not
   correctness-critical — could be deferred to a later iteration rather than
   the first cut of the module.
