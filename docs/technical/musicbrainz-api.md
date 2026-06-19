# MusicBrainz API — Integration Reference

How the MBZ REST API v2 maps to the `ports.MetadataSource` methods in `internal/adapters/mbz/`.

---

## Base URL

```
https://musicbrainz.org/ws/2/
```

Rate limit: **1 request per second** (HTTP 429 returned above that).  
The adapter enforces this via an internal `rateLimiter`.

All requests must include a `User-Agent` header identifying the application.

---

## Endpoints in use

### Artist search

```
GET /ws/2/artist?query=<LUCENE>&fmt=json&limit=N
```

Used by `SearchStudios` and `SearchPeople`.

**Type filtering** is done inside the Lucene `query` parameter, not as a standalone `type=` URL param:

| Method | Query example |
|---|---|
| `SearchStudios` | `query=The Beatles` (no type filter — returns both groups and solo artists) |
| `SearchPeople` | `query=John Lennon AND type:Person` |

Response shape:
```json
{
  "artists": [
    { "id": "b10bbbfc-...", "name": "The Beatles", "disambiguation": "...", "country": "GB" }
  ]
}
```

The `type` field is **not** returned in the search response object; it's only part of the query syntax.

---

### Artist lookup (by MBID)

```
GET /ws/2/artist/<MBID>?inc=url-rels&fmt=json
```

Used by `FindByExternalID`.

Returns 404 if the MBID does not exist → adapter maps this to `ports.ErrNotFound`.

---

### Recording search

```
GET /ws/2/recording?query=recording:<TITLE>&fmt=json&limit=N
```

Used by `SearchItems`. The `recording:` prefix scopes the Lucene field to the recording title.

Response shape:
```json
{
  "recordings": [
    {
      "id": "...",
      "title": "Come Together",
      "length": 259000,
      "releases": [
        { "artist-credit": [{ "name": "The Beatles", "artist": { "id": "...", "name": "The Beatles" } }] }
      ]
    }
  ]
}
```

`length` is in milliseconds. `releases[0].artist-credit[0]` is used to populate `ExternalItem.Studio`.

---

### Release-group browse (artist discography)

```
GET /ws/2/release-group?artist=<ARTIST-MBID>&fmt=json&limit=N&offset=N
```

Used by `FetchEntryContent`.

**`inc=releases` is NOT valid on this endpoint** — the MBZ API returns HTTP 400 if included.

`first-release-date` is always present in the response object (no `inc` required).

Response shape:
```json
{
  "release-group-count": 42,
  "release-groups": [
    {
      "id": "...",
      "title": "Abbey Road",
      "first-release-date": "1969-09-26",
      "primary-type": "Album"
    }
  ]
}
```

Year is parsed from the first 4 characters of `first-release-date`.

---

### Release browse (resolve release-group → release)

```
GET /ws/2/release?release-group=<RG-MBID>&limit=1&fmt=json
```

Step 1 of `FetchGroupContent`. Gets the canonical release MBID for a release-group.

Response shape:
```json
{ "releases": [{ "id": "<RELEASE-MBID>" }] }
```

---

### Release lookup (tracks)

```
GET /ws/2/release/<RELEASE-MBID>?inc=recordings&fmt=json
```

Step 2 of `FetchGroupContent`. Gets all tracks across all media (discs).

Response shape:
```json
{
  "media": [
    {
      "track-count": 17,
      "tracks": [
        {
          "title": "Come Together",
          "recording": { "id": "...", "title": "Come Together", "length": 259000 }
        }
      ]
    }
  ]
}
```

Tracks are flattened across all `media` entries. Pagination is applied in-memory because MBZ does not support track-level pagination.

---

## What is NOT supported

| Operation | Reason |
|---|---|
| `FindByHash` | MBZ has no file-hash lookup → `ports.ErrNotSupported` |
| `inc=releases` on release-group browse | Invalid → HTTP 400 |
| `type=` as a standalone URL parameter on artist search | Silently ignored by MBZ; must be embedded in Lucene query |

---

## Known stable MBIDs (for tests)

| Entity | Name | MBID |
|---|---|---|
| Artist | The Beatles | `b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d` |
