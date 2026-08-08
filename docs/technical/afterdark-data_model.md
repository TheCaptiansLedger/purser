# AfterDark Module — Data Model Proposal

> Status: decided (Section 5) during AfterDark scan-pipeline planning — no
> new ADR, since nothing here is a new architectural decision, just 0024/
> 0026/0027 applied to a new content type. Sections 1-4 remain the original
> research pass; Section 5 records the resolutions.

Scope decision from the planning conversation: **AfterDark is one module**,
covering both western adult content and JAV, via StashDB and ThePornDB
(theporndb.net) — not split into separate `afterdark`/`jav` docs, even
though prior art (see Section 4) treated them as distinct.

Unlike Music, there is **no dedicated pre-reset data model doc** to compare
against — `docs/technical/v1/` only ever covered music. The closest prior
art is the generic schema in [`v1/data-model.md`](v1/data-model.md) (which
already has `adult`/`jav` as `content_type` values and `Performer`/`Actress`
as roles) and a deleted doc, `docs/project/content-types.md`, recovered via
`git show 8ae1ac2^:docs/project/content-types.md` for this research pass —
see Section 4.

---

## 1. Providers covered

| Source | Auth | `.env` key(s) | Role |
|---|---|---|---|
| StashDB | API key (`ApiKey` header) | `PURSER_SOURCES_STASHDB_ENABLED`, `PURSER_SOURCES_STASHDB_API_KEY` | Community-curated GraphQL database: performers, studios, scenes, tags, sites. Crowd-edited (has a full `Edit` review workflow) and crowd-fingerprinted for scene identification. |
| ThePornDB (theporndb.net) | Bearer token | live in `.env` as unprefixed `THEPORNDB_API_KEY` only — **no app-level `PURSER_SOURCES_TPDB_*` key exists yet**, though `.env.example` already documents the convention for it | REST database covering both western scenes and JAV (`type: "JAV"` on the same scene-shaped resource). Aggregates performer/scene data across many sites, includes JAV via r18.dev/r18.com sourcing. |

**Gap found, not fixed here (out of scope for this doc, same as the
TVDB/OMDb gap noted in the Music pass):** `.env.example` already has a
`PURSER_SOURCES_TPDB_ENABLED`/`PURSER_SOURCES_TPDB_API_KEY` template
(lines 13-16), but the live `.env` only has the legacy unprefixed
`THEPORNDB_API_KEY`. Worth promoting when config is next touched.

Note also: `.env` has a `STASH_URL` (self-hosted Stash media-organizer
instance, distinct from stashdb.org) used for something else in this
environment — not one of the two providers requested for this doc, and not
treated as a metadata provider here. Flagged under Open Questions since it
could plausibly be a third data source later.

All schema claims below are live-verified against both APIs during this
research pass (StashDB via GraphQL introspection + live queries against
public data for Riley Reid / X-Art; ThePornDB via its documented
`api.theporndb.net` host — note this differs from `theporndb.net/api`, which
returns 401 regardless of key validity).

---

## 2. Provider data shapes

### StashDB

**Base URL:** `https://stashdb.org/graphql` (GraphQL, POST). **Auth:**
`ApiKey` header. Live schema introspection (`__schema.queryType.fields`)
confirms the full query surface: `Performer`, `Studio`, `Tag`
(+ `TagCategory`), `Scene`, `Site` (+ `SiteCategory`), `Edit`, `User`. There
is **no `Movie`/DVD-release entity** — StashDB models everything at the
Scene level; a DVD/compilation-style grouping would have to come from
`Studio`/`Site` hierarchy, not a dedicated type.

**Performer** (`queryPerformers`, live-verified against Riley Reid):
```
id, name, disambiguation, aliases[], gender, birth_date, country,
ethnicity, eye_color, hair_color, height, cup_size, band_size,
waist_size, hip_size, breast_type, career_start_year, career_end_year,
tattoos[]{location, description}, piercings[]{location, description},
urls[]{url, site{name}}, images[]{url, width, height}, is_favorite
```
`urls[].site.name` cross-references external identity — a Riley Reid lookup
returned links to Wikipedia, Fancentro, OnlyFans, **ThePornDB**,
**IAFD**, AFDB, Babepedia, DATA18, FreeOnes among others. This is a
different linking mechanism than MusicBrainz's typed `relations[]` — here
every external link is undifferentiated except by `site.name` string
matching, so a "give me the ThePornDB ID for this performer" query means
string-matching `site.name == "ThePornDB"` and parsing the URL, not a typed
external-ID field.

**Studio** (`queryStudios`, live-verified against X-Art):
```
id, name, urls[]{url, site{name}}, parent{id, name},
child_studios[]{id, name}, images[]{url}
```
Self-referential parent/child hierarchy, one level shown in the query but
presumably arbitrary depth (X-Art → parent Malibu Media).

**Scene** (`queryScenes`, live-verified, filtered by performer):
```
id, title, details, date, duration, director, code,
studio{id, name, parent{id, name}},
performers[]{performer{id, name}, as},
tags[]{id, name}, urls[]{url, site{name}}, images[]{url, width, height},
fingerprints[]{hash, algorithm, duration, submissions}
```
`performers[].as` is the credited/on-screen name for that specific scene,
distinct from the performer's canonical `name` — same pattern as
MusicBrainz's `artist-credit` name-vs-canonical-artist distinction in Music.
A single scene returned ~20 tags in the live check (granular action/attribute
tags: "Creampie", "Doggy Style", "Trimmed Pussy", etc.), confirming StashDB's
tag taxonomy is deep and scene-level, not just genre-level.

**Tag** (`queryTags`, live-verified against "Creampie" → resolved to
"Anal Creampie"):
```
id, name, description, aliases[], category{name, group}
```
Tags have their own alias list (11 aliases found, including CJK
translations) and a two-level taxonomy: `category.group` (e.g. `"ACTION"`)
→ `category.name` (e.g. `"Finishers"`) → the tag itself. This is
structurally richer than anything in the Music module — MusicBrainz genres
have no category grouping, TheAudioDB genre/mood/style are three flat
single-value fields. If AfterDark's tags feed the shared `Tag(key, value,
scope)` model from Section 3, the category/group information doesn't fit
that flat shape without a decision (see Open Questions).

**Fingerprint-based scene identification** — the AfterDark analog of
AcoustID, and structurally *more* mature than anything in the Music module.
Every scene carries a `fingerprints[]` array: `{hash, algorithm ("PHASH" |
"OSHASH"), duration, submissions}` — `submissions` is a crowd-sourced
confidence count (the live-checked scene had one hash submitted 4 times, a
strong duplicate-confirmation signal). There's a dedicated top-level query,
`findScenesBySceneFingerprints`, and write-back mutations
(`submitFingerprint`, `submitFingerprints`) for contributing new hashes back
to the community database. This is the identification mechanism a
`FileIdentifier`-style adapter (per v1's music `FileIdentifier` pattern)
would use to match a file on disk to a Scene without relying on filename
parsing at all.

**Community edit workflow** — `Edit`/`queryEdits`, plus `submitSceneDraft`/
`submitPerformerDraft`/`destroyDraft` mutations, mean StashDB is a
read-and-write collaborative database, unlike MusicBrainz (also
collaborative, but Purser only ever read from it) or TheAudioDB/fanart.tv
(pure read-only image/text sources). Whether Purser should ever write back
is an open question, not assumed here.

### ThePornDB (theporndb.net)

**Base URL:** `https://api.theporndb.net/` — **not** `theporndb.net/api`,
which returns `401 unauthorized` regardless of key validity; this cost real
debugging time during this research pass and is worth documenting
explicitly for whoever writes the adapter. **Auth:** `Authorization: Bearer
{token}` header.

**Performer** (`GET /performers?q=`, live-verified against Riley Reid):
```
id, _id (legacy numeric ID), slug, name, full_name, disambiguation, bio,
rating, is_parent,
extras{gender, birthday, birthday_timestamp, birthplace, birthplace_code,
       astrology, ethnicity, nationality, hair_colour, eye_colour, weight,
       height, measurements, cupsize, tattoos, piercings, waist, hips,
       fake_boobs, same_sex_only, career_start_year, career_end_year,
       links{<SiteName>: <url>, ... 40+ site keys}}
```
`extras.links` is a flat map keyed by site display name (`"IAFD"`,
`"StashDB"`, `"Pornhub"`, `"IMDb"`, etc.) — same undifferentiated-by-type
problem as StashDB's `urls[]`, but as a map instead of a list. **Notably,
ThePornDB's own performer response links back to StashDB by URL** (and
vice versa) — the two providers are already cross-referenced by their
respective communities, which is a strong argument for using one as the
primary identity source and the other as enrichment, rather than treating
them as fully independent (see Section 3).

**Performer alias/parent linking — structurally different from both Music
and StashDB.** ThePornDB models a performer's site-specific persona as its
*own* performer record with `is_parent: false` and a `parent` field pointing
to the canonical `is_parent: true` record — e.g. a scene credited to
"Anikka" returned a performer record for "Anikka" (`is_parent: false`)
whose `parent` field is the full "Anikka Albrite" record. This is a
different mechanism than StashDB's `performers[].as` (a credit-name string
on the scene-performer join) or MusicBrainz's `aliases[]` (alternate names
on one canonical record) — ThePornDB creates a **separate queryable entity**
per alias, each with its own `id`/`slug`/image set, linked via `parent`.

**Scene** (`GET /scenes?q=`, live-verified):
```
id, _id, title, type ("Scene" | "JAV"), slug, external_id, description,
rating, site_id, date, url, duration, format, sku,
poster/posters{full,large,medium,small}, background{...}, back_image,
created, last_updated,
performers[] (full nested Performer objects, not just IDs),
tags[], directors[], links[], hashes[]{hash, type, duration, submissions,
                                        users[], created_at, updated_at}
site{uuid, id, name, short_name, url, logo, network{...}, parent}
```
`hashes[]` mirrors StashDB's `fingerprints[]` almost field-for-field
(`type` instead of `algorithm`, `submissions` count present on both) — the
same live scene checked against both providers returned the *same* PHASH
value (`e18951f0e3a49edc`) from both StashDB and ThePornDB independently,
confirming both communities converge on the same hash for the same file,
which matters if both are used as identification sources (no need to
reconcile conflicting hashes for the same content, just merge submission
counts).

`tags[]` was **empty on every scene checked** in this pass (western and
JAV alike), in contrast to StashDB's ~20 populated tags on the same scene.
This is a real, observed asymmetry, not a documentation gap — if tag
richness matters, StashDB is the stronger source for it, at least for
scenes that exist in both.

**JAV coverage** — confirmed live via `GET /jav?parse={code}` (e.g.
`?parse=SSIS-001`), which returns the *same Scene shape* as above with
`type: "JAV"`, `site` pointing to a JAV-specific network (`r18.dev`/
`r18.com` sourcing), `sku` holding the raw JAV product code
(`"ssis00001"`) distinct from `external_id` (`"ssis-001"`, hyphenated/
lowercased). The `/jav` collection endpoint (no query) lists newest JAV
scenes; `?parse=` is the code-lookup path — this is the closest thing
ThePornDB has to a "resolve a filename/code to a title" endpoint, directly
relevant to the pipeline's file-identification step (point 11 in the
planning conversation). Plain `/scenes?q=SSIS-001` did **not** find the same
JAV title — JAV content is only reliably reachable through `/jav`, not the
general scene search, which matters for adapter routing (a `FileIdentifier`
would need to try both paths, or detect JAV-shaped codes and route
directly).

**Site/Studio** (`site` object nested in scene responses):
```
uuid, id, parent_id, network_id, name, short_name, url, description,
rating, logo, favicon, poster, network{...}, parent{...}
```
Same parent/network hierarchy concept as StashDB's Studio, independently
maintained (different IDs, no cross-reference between the two providers'
studio graphs found in this pass — only performers/scenes were seen to
cross-link).

---

## 3. Proposed modern entity model

Conceptual, not Go structs — same constraint as the Music doc. People, Tags,
and External IDs are deferred to the shared-domain synthesis phase.

### Performer

| Attribute | Notes |
|---|---|
| Name, aliases | Both providers return alias data, but shaped differently — StashDB as a flat `aliases[]` list on one record (like MBZ), ThePornDB as separate linked performer records via `parent`. Reconciling these two alias models is an open question, not resolved here. |
| Physical/biographical attributes | Gender, ethnicity, hair/eye color, measurements, tattoos, piercings, career start/end years — both providers cover this ground almost identically; StashDB's is more structured (typed enums for gender/ethnicity/hair/eye color), ThePornDB's is closer to free text |
| External identity | StashDB performer MBID-equivalent is its own UUID; ThePornDB likewise. Both cross-reference each other by URL already (see Section 2) — a real opportunity to treat StashDB ID as canonical and resolve ThePornDB ID via the existing cross-link rather than independent matching |
| Deferred to shared **People** | Scene/title credits (who appears in what) — same `ItemPerson`-style mechanism v1 used for Music track credits, roles `performer`/`actress` per the recovered `content-types.md` distinction |

### Studio (Network → Studio → Site hierarchy)

| Attribute | Notes |
|---|---|
| Name, parent/child hierarchy | Both providers model this identically in spirit (self-referential parent link); matches the existing shared `library_entries` self-referential tree from v1's general schema, no new mechanism needed |
| External identity | Independent ID graphs per provider, no observed cross-linking between StashDB Studio and ThePornDB Site in this pass (only performers/scenes cross-link) |

### Scene / JAV Title

Proposed as **one conceptual leaf entity**, not two, consistent with the
"one AfterDark module" decision — with a `content_kind` (or equivalent)
distinguishing `scene` vs `jav_title` only where terminology actually
differs (v1's recovered `content-types.md` used "Scene"/"Performer" for
`adult` and "Title"/"Actress" for `jav`), not as a structural fork. Both
providers already return JAV through the *same* Scene-shaped response
(`type: "JAV"` on ThePornDB), which supports treating this as one entity
with a discriminator rather than two entities.

| Attribute | Notes |
|---|---|
| Title, description/details, date, duration | Present on both providers, same shape |
| Code/SKU | ThePornDB's JAV `sku`/`external_id` split (raw product code vs. normalized slug) matters for matching — a JAV file on disk is most reliably identified by this code, not by title fuzzy-matching |
| Studio/Site | Reference to Studio entity above |
| Performers, credited-as name | StashDB's `performers[].as` and ThePornDB's parent/alias-performer model both need to resolve to "who actually appears, under what displayed name" |
| Tags | Deferred to shared **Tags** — see the tag-provenance open question below, sharper here than in Music because the two sources disagree in richness, not just in structure |
| **Fingerprints** | First-class, not an afterthought — both providers already return crowd-sourced content hashes (PHASH/OSHASH) with submission counts as confidence. This is a stronger, more mature identification mechanism than Music had with AcoustID (which v1 only stored, never looked up). A `FileIdentifier`-style adapter for this module should treat fingerprint lookup as the primary matching path, with title/code parsing as fallback — the reverse priority from v1's music pipeline, which led with tag/filename parsing and used AcoustID as one signal among several |

### Deferred to shared domain (not designed here)

- **People** — performer/actress credits, same generic mechanism as Music.
- **Tags** — genre/action/attribute tags from StashDB's rich
  category/group taxonomy vs. ThePornDB's sparse (often empty) tag field.
  Whether `Tag` needs to carry StashDB's `category`/`group` structure, or
  flattens it into `key="category:group"`-style values, is unresolved.
- **External IDs** — `(entity_type, entity_id, source, external_id)`, same
  shape as Music, extended with `stashdb` and `tpdb` as `source` values
  (already anticipated in v1's `external_ids.source` enum:
  `stashdb|tpdb|tmdb|tvdb|mbid|javlibrary|r18|discogs|mal|anilist`).
- **Fingerprints as a shared concept** — this is the one place this module
  argues for something Music didn't need: fingerprints are richer here
  (multi-algorithm, submission-counted, queryable in bulk via
  `findScenesBySceneFingerprints`) than a single AcoustID string in a
  metadata bag. Whether this warrants a first-class shared `Fingerprint`
  concept usable by any content-identification pipeline (not just
  AfterDark) is a strong candidate question for the cross-module synthesis
  phase — the pipeline is supposed to be the core (per point 9 in the
  planning conversation), and content-hash identification is exactly the
  kind of thing that shouldn't be reinvented per module.

---

## 4. Comparison to prior snapshot

No dedicated AfterDark data model existed pre-reset — only the generic
schema (`v1/data-model.md`) and a recovered UI-facing doc
(`docs/project/content-types.md`, deleted in the reset, not part of the
frozen `v1/` snapshot). That recovered doc is the actual prior art here:

1. **`adult` and `jav` were distinct `content_type` values** with distinct
   hierarchy leaf names (`Scene` vs `Title`) and distinct person roles
   (`Performer` vs `Actress`). Live provider research supports *not*
   forking this into two content types structurally, since both providers
   already return JAV through the same Scene/performer shape — but the
   `Scene`/`Title` and `Performer`/`Actress` **display terminology**
   difference is real and provider-confirmed, so a discriminator field
   (not a separate entity or separate module) is the better fit. This
   partially agrees and partially disagrees with prior art: same one-module
   decision as this doc, different mechanism (v1 used two `content_type`
   values under one general schema; this proposal argues for one `content_type`
   value with a display-terminology discriminator, since JAV is
   provider-confirmed to share the Scene shape, not just a schema
   convenience).
2. **Person role vocabulary** (`performer`/`actress`) already anticipated
   in `v1/data-model.md`'s `item_people.role` enum
   (`performer|actress|director|actor|artist|producer`) — no change needed.
3. **`external_ids.source` enum already anticipated** `stashdb`, `tpdb`,
   and `javlibrary`/`r18` as sources — confirms this module was designed
   for from the start, just never documented in depth the way Music was.
4. No prior art exists for the fingerprint-as-primary-identification idea
   proposed in Section 3 — this is new to this pass, not a revision of
   something v1 had (v1's fingerprint/hash matching machinery was
   music-specific, `MusicConfidenceSignals`/`MusicTagSummary`, not a shared
   mechanism).

---

## 5. Resolved (was "Open questions" — decided during scan-pipeline planning)

1. **No new domain types.** Scene/JAV-Title = kernel `domain.Item`
   (`ContentType=adult`), `Metadata["kind"]="scene"|"jav_title"` as the
   display discriminator. Studio/Network = kernel `domain.LibraryEntry`
   (`Kind=studio|network`). Performer = `domain.Person` +
   `afterdark.PerformerProfile` + `ItemPerson` credits. No `music.Release`-
   style edition tier — a scene has no pressing/edition concept.
2. **Fingerprint matching is not a new shared port.** Reuses 0024's
   existing per-content-type `FileFingerprinter`/`Identifier` pattern, same
   as Music's AcoustID. In scope: both OSHash (`pkg/filehash.OSHash`,
   already implemented) and PHash (new — a video perceptual hash matching
   Stash's own algorithm closely enough to be lookup-compatible; doesn't
   exist in the repo yet, real work, not deferred). Fingerprint is the
   *primary* identification tier for this module (reversed from Music,
   where AcoustID was a fallback) since both providers converge on the
   same hash for the same content.
3. **StashDB vs. ThePornDB: neither is canonical, ever — automatic or
   manual.** Each gets its own port/adapter/Connect service
   ([0027](../adr/0027-provider-independence.md)). Inside the automatic
   scan pipeline, each provider's match is scored independently; every one
   that clears the confidence threshold gets merged into the persisted
   scene (external IDs/tags/images unioned). Scalar-field conflicts
   (Title/Overview/Date) resolve via a user-configured provider priority
   list, not a hardcoded pick. Below threshold: existing manual
   `AcceptCandidate` path (one candidate or a raw ID) — any human-side
   merging is client/UI work, not backend.
4. **Tag category/group structure flattens** into the existing flat
   `Tag(scope,key,value)` shape at ingest (`key="category:group"` style).
   No kernel `Tag` schema change — revisit only if a second module needs
   real category structure.
5. **Performer alias mismatch flattens** into kernel `Person.Aliases[]`.
   StashDB's `aliases[]` maps directly. ThePornDB's parent-linked alias
   performer's name becomes a string in the canonical performer's
   `Aliases[]`; its own ThePornDB ID is *also* linked via its own
   `ExternalID` row to the same `Person` (multiple `ExternalID` rows can
   point at one `EntityID`, already schema-legal).
6. **Self-hosted Stash instance (`STASH_URL`)** — out of scope. Not one of
   the two providers this module targets.
7. **Contributing back to StashDB** — out of scope. Read-only, matching
   [0027](../adr/0027-provider-independence.md)'s read-only-passthrough
   rule; no write capability built.
