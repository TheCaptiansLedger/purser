# Music Metadata Providers (v1 snapshot)

> Snapshot built directly from the adapter source at commit `5fcd657`
> (`internal/adapters/mbz/`, `internal/adapters/theaudiodb/`,
> `internal/adapters/fanart/`), not from recollection. See
> [README.md](README.md) for how to use this document.

Three real adapters existed for music. There was **no dedicated Cover Art
Archive or Last.fm adapter** — a prior project memory claiming otherwise was
wrong; see the notes under MusicBrainz and Artist metadata below for what
actually touched those two names.

| Source | Adapter package | Auth | Rate limit | Role |
|---|---|---|---|---|
| MusicBrainz | `internal/adapters/mbz/` | None (User-Agent required) | 1 req/sec, hard-enforced | IDs, artist/release/track metadata, discography |
| TheAudioDB | `internal/adapters/theaudiodb/` | API key in URL path | None enforced client-side | Artist images (priority 100), album cover fallback |
| fanart.tv | `internal/adapters/fanart/` | API key as `api_key` query param | None enforced for personal keys | Artist images (priority 50, secondary), album covers |

---

## MusicBrainz

**Base URL:** `https://musicbrainz.org/ws/2/` (overridable via config, e.g. for a self-hosted mirror).
**Auth:** none. Required `User-Agent` header: `purser/1.0 (https://github.com/thecaptiansledger/purser)` (overridable via config).
**Rate limiting:** a client-side limiter serialized requests to one per second minimum gap (`internal/adapters/mbz/adapter.go`, `rateLimiter`). On `429 Too Many Requests`, the adapter read the `Retry-After` header and retried once the wait elapsed. On `503` with body containing `"rate limit"`, it waited a flat 5 seconds and retried. On `400` with body containing `"Invalid mbid"`, it mapped to a typed not-found error rather than a generic error.
**Caching:** all GET responses cached 24h keyed by full URL, when a cache is configured.

### Endpoints used

| Endpoint | Purpose | Key query params |
|---|---|---|
| `GET /artist?query=<name>[ AND type:Person]&fmt=json&limit=N` | Search artists (studio search = any type; people search = `type:Person` appended) | `query`, `limit` |
| `GET /artist/{mbid}?inc=url-rels&fmt=json` | Fetch one artist by MBID (`FindByExternalID`) | `inc` |
| `GET /artist/{mbid}?inc=artist-rels&fmt=json` | Fetch band members / solo self-link (`FetchEntryPeople`) | `inc` |
| `GET /artist/{mbid}?inc=artist-rels+url-rels+aliases&fmt=json` | Full artist enrichment (`FetchEntryMetadata`) | `inc` |
| `GET /recording?query=recording:<title>&fmt=json&limit=N` | Search tracks by title | `query`, `limit` |
| `GET /recording/{mbid}?inc=artist-credits+releases+release-groups&fmt=json` | Fetch one recording, with all its releases and release-groups embedded | `inc` |
| `GET /release?release-group=<rgMBID>&limit=1&fmt=json` | Resolve a release-group to one canonical release MBID | `release-group`, `limit` |
| `GET /release/{mbid}?inc=recordings&fmt=json` | Fetch all tracks (media/tracks) for a specific release | `inc` |
| `GET /release?artist=<mbid>&status=official&inc=release-groups&fmt=json&limit=100&offset=N` | Paginate an artist's official releases, deduplicated to unique release-groups (discography) | `artist`, `status`, `inc`, `limit`, `offset` |
| `GET /release?release-group=<rgMBID>&inc=labels+mediums&limit=100&fmt=json` | Fetch all known pressings/editions of a release-group | `release-group`, `inc`, `limit` |
| `GET /release?query=barcode:<barcode>&fmt=json` | Resolve a barcode to a release | `query` |
| `GET /isrc/{isrc}?inc=releases+release-groups&fmt=json` | Resolve a recording ISRC to a release-group MBID | `inc` |

### Response shapes actually consumed

Artist search/detail (`mbzArtist`, `mbzArtistDetail`, `mbzArtistWithRels`, `mbzArtistEnrichment`):
```
id, name, disambiguation, type ("Group"|"Person"|"Orchestra"|"Choir"|"Character"|"Other"),
country, life-span{begin,end}, begin-area{name}, isnis[],
aliases[]{name, type, locale},   // filtered to locale=="" or "en", type in {"Artist name","Search hint",""}
relations[]{type, direction, url{resource}, artist{id,name}}
```
Notable relation types mapped to metadata: `"member of band"` + `direction:"backward"` → band member; `"official homepage"` → `official_url`; `"last.fm"` → `lastfm_url`; `"wikipedia"` → `wikipedia_url`. **These are the only "Last.fm" and homepage touches in the whole codebase** — captured as a URL string from an MBZ relation, never fetched or parsed as Last.fm data.

Recording (`mbzRecording`, `mbzRecordingRelease`):
```
id, title, length (ms), artist-credit[]{name, artist{id,name}},
releases[]{id, title, artist-credit[], barcode, country, date,
           label-info[]{label{name}, catalog-number},
           release-group{id, title, first-release-date, primary-type, secondary-types}}
```
When multiple releases were embedded on a recording, the adapter picked the one whose title matched an "album hint" (the file's tagged album name) case-insensitively, falling back to the first. `GroupExternalID` on the resulting `ExternalItem` is always the **release-group** MBID (the conceptual album), not the release MBID — this is what lets discography-imported albums and scan-matched files converge on the same group.

Release/medium/track (`mbzReleaseDetail`, `mbzMedium`, `mbzTrack`):
```
media[]{track-count, tracks[]{number (string, e.g. "1" or "A1" for vinyl),
                                title, recording{id, title, length}}}
```

Release-group browse for discography (`mbzReleaseBrowseResponse`):
```
releases[]{release-group{id, title, first-release-date, primary-type, secondary-types}},
release-count
```
Paginated locally at 100/page, deduplicated by release-group ID across pages (MusicBrainz can return the same release-group multiple times across different releases).

Full release list for pressings (`mbzFullRelease`, used by `FetchReleaseGroupReleases` and barcode search):
```
id, title, status ("Official" required to be eligible as default), country, date, barcode,
label-info[]{label{name}, catalog-number}, media[]{format, track-count}
```
`IsDefault` was computed client-side: earliest `status=="Official"` release by date string comparison, empty dates sorted last, ties broken by API result order.

### Cover art

MusicBrainz responses do not include cover art. The adapter derived a Cover Art Archive URL directly from the release MBID with no separate API call or adapter:
```
https://coverartarchive.org/release/{release_mbid}/front-250
```
`ImagePriority()` for the MusicBrainz adapter itself was `0` — it never claimed to be an image source.

---

## TheAudioDB

**Base URL:** `https://www.theaudiodb.com/api/v1/json/` (overridable).
**Auth:** API key embedded as a URL path segment: `{baseURL}{apiKey}/{endpoint}`. Free tier key was `"123"`; a personal key came via config for Patreon subscribers.
**Rate limiting:** none enforced client-side.
**Caching:** 24h, keyed by the full request URL (including the key segment).
**Role:** `ImagePriority() = 100` — the preferred image source for music (artist thumbnails/backgrounds/banner; album cover fallback).

### Endpoints used

| Endpoint | Purpose |
|---|---|
| `GET {key}/search.php?s=<name>` | Search artists by name (`SearchStudios`) |
| `GET {key}/artist-mb.php?i=<mbid>` | Fetch one artist by MusicBrainz ID — the primary lookup, since TheAudioDB is always entered via an MBID already resolved from MusicBrainz |
| `GET {key}/album-mb.php?i=<releaseGroupMBID>` | Fetch one album's cover art by release-group MBID |

### Response shapes actually consumed

```
artists[]{strArtist, strMusicBrainzID, strBiographyEN, strArtistThumb,
          strArtistFanart, strArtistFanart2, strArtistFanart3,
          strArtistBanner, strWebsite, strGenre}
album[]{strMusicBrainzID, strAlbumThumb}
```
Search results without `strMusicBrainzID` were dropped — TheAudioDB entries were only used when they could be tied back to a MusicBrainz artist, since MBID is the join key across every source.

### Image slot mapping

- `strArtistThumb` → `ImageTypeHero`
- `strArtistFanart` / `strArtistFanart2` / `strArtistFanart3` → `ImageTypeBackground` (all three, if present)
- `strArtistBanner` → `ImageTypeBanner`
- `strAlbumThumb` → `ImageTypePoster` (album cover)

`Verify()` (health check) called `SearchStudios(ctx, "Radiohead", 1)` and required it to succeed.

---

## fanart.tv

**Base URL:** `https://webservice.fanart.tv/v3/` (overridable).
**Auth:** `api_key` query parameter appended to every request.
**Rate limiting:** none enforced — comment in source notes personal keys aren't rate-limited.
**Caching:** 24h, keyed by the request path *without* the query string (the API key is fixed per instance, so it was deliberately excluded from the cache key).
**Role:** `ImagePriority() = 50` — secondary/fallback image source, behind TheAudioDB.

### Endpoints used

There is exactly **one** endpoint for music, reused for three different capabilities:

| Endpoint | Used for |
|---|---|
| `GET music/{mbid}?api_key=...` | Artist images (`FindByExternalID`, `FetchPersonImage`) **and** every album's cover art for that artist (`FetchEntryContent`, `FindGroupImages`) — the same response serves all three |

Because one call returns both artist art and a map of every album's art, `FetchEntryContent` (list albums with covers) and `FindGroupImages` (get one specific album's cover) both re-fetch the same `music/{mbid}` response rather than a per-album endpoint — there is no per-album fanart.tv endpoint.

### Response shape actually consumed

```
{
  name, mbid_id,
  artistthumb[]{id, url, likes},
  artistbackground[]{...},      // fetched but not mapped to any image slot — see below
  hdmusiclogo[]{...},           // fetched but not mapped — no corresponding slot
  musicbanner[]{id, url, likes},
  cdart[]{...},                 // fetched but not mapped — no corresponding slot
  albums: {
    "<release-group-mbid>": {
      albumcover[]{id, url, likes},
      cdart[]{...}              // not mapped
    }
  }
}
```

### Image slot mapping

- `artistthumb` → `ImageTypeHero`
- `musicbanner` → `ImageTypeBanner`
- `artistbackground`, `hdmusiclogo`, `cdart` → deliberately **not** mapped; comment in source: "not a valid music library_entry image slot"
- `albums[rgMBID].albumcover` → `ImageTypePoster`, keyed by release-group MBID (album pagination sorted the map keys for determinism, since Go map iteration order is random)

`Verify()` (health check) probed The Beatles' well-known MBID (`b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d`) and treated `ErrNotFound` as a successful verification (key accepted, just no data for that probe) — only a transport/auth error failed verification.

---

## Cross-provider notes

- **MBID is the join key.** Every provider call after the initial MusicBrainz search/import is keyed by MusicBrainz ID (artist MBID, release-group MBID, or recording MBID) — TheAudioDB and fanart.tv were never searched by name directly in the pipeline, only looked up by an MBID already obtained from MusicBrainz.
- **No provider returned track-level audio data** (duration used for confidence scoring came from decoding the audio bitstream locally, tag data came from embedded file tags — see [music-data-model.md](music-data-model.md#musictagsummary)).
- **Image priority order** across the two image-capable sources: TheAudioDB (100) beats fanart.tv (50). MusicBrainz never supplied images (priority 0); Cover Art Archive was a derived URL, not a competing prioritized source.
