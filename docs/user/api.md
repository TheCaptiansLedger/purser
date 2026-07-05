# Purser — API Reference

Base URL: `http://localhost:7474/api/v1`

All requests and responses use `application/json`. No authentication is required (authentication is deferred to a future phase).

## Error format

Every error response uses the same shape:

```json
{ "error": "human-readable message", "code": "SNAKE_CASE_CODE" }
```

## Async operations

Some import operations (fetching a full artist discography, downloading images, creating many tracks) run as background jobs. When a response includes `"job_id"`, poll until done:

```bash
# Poll until status is "done" or "error"
GET /api/v1/commands/{job_id}
```

```json
{
  "id": "abc123",
  "name": "ImportEntry",
  "status": "done",
  "started_at": "2026-07-04T10:00:00Z",
  "completed_at": "2026-07-04T10:00:03Z"
}
```

Status values: `queued` | `running` | `done` | `error`

---

## Config

### Get app config
```bash
GET /api/v1/config
```
```json
{
  "modules": {
    "music": { "enabled": true, "roots": ["/media/content/music"] },
    "afterdark": { "enabled": true, "roots": ["/media/content/adult"] },
    "movies": { "enabled": false, "roots": [] }
  },
  "scan": {
    "auto_import_threshold": 0.85
  }
}
```

### Get content type configs (used by the UI to build module tabs)
```bash
GET /api/v1/config/content-types
```
```json
[
  {
    "content_type": "music",
    "label": "Music",
    "module_key": "music",
    "parent_kind": "artist",
    "group_label": "Albums",
    "item_label": "Tracks",
    "supports_file_grouping": true
  },
  {
    "content_type": "adult",
    "label": "AfterDark",
    "module_key": "afterdark",
    "parent_kind": "studio",
    "group_label": "Series",
    "item_label": "Scenes",
    "supports_file_grouping": false
  }
]
```

---

## Library Entries

Library entries are the root nodes of the content tree: artists, studios, series, authors.

### List entries
```bash
GET /api/v1/library-entries?contentType=music&kind=artist&page=1&pageSize=25
```

### Get one entry
```bash
GET /api/v1/library-entries/{id}
```
```json
{
  "id": "le-001",
  "content_type": "music",
  "kind": "artist",
  "name": "Stevie Nicks",
  "sort_name": "Nicks, Stevie",
  "overview": "American singer-songwriter...",
  "monitored": true,
  "monitor_mode": "all",
  "status": "active",
  "path": "/media/content/music/Stevie Nicks",
  "image_path": "/media/images/entries/le-001.jpg",
  "external_ids": [
    { "source": "musicbrainz", "value": "8b4dba1b-..." }
  ],
  "added_at": "2026-07-04T10:00:00Z"
}
```

### Create an entry manually (no external source)
```bash
POST /api/v1/library-entries
```
```json
{
  "content_type": "music",
  "kind": "artist",
  "name": "The Boozy Fig Digglers",
  "monitored": true,
  "monitor_mode": "all"
}
```
Returns the created entry with its generated `id`.

### Update an entry
```bash
PATCH /api/v1/library-entries/{id}
```
```json
{
  "overview": "Updated bio text",
  "monitored": false
}
```

### Delete an entry
```bash
DELETE /api/v1/library-entries/{id}
```

---

## Groups

Groups are mid-level containers: albums (music), seasons (TV), series (adult).

### List groups for an entry
```bash
GET /api/v1/groups?libraryEntryId={entry_id}
```

### Get one group
```bash
GET /api/v1/groups/{id}
```
```json
{
  "id": "grp-001",
  "library_entry_id": "le-001",
  "title": "Bella Donna",
  "sort_name": "Bella Donna",
  "year": 1981,
  "monitored": true,
  "monitor_mode": "all",
  "image_path": "/media/images/groups/grp-001.jpg",
  "external_ids": [
    { "source": "musicbrainz", "value": "rg-mbid-..." }
  ]
}
```

### Create a group manually
```bash
POST /api/v1/groups
```
```json
{
  "library_entry_id": "le-001",
  "title": "Pickled Frequencies",
  "year": 2026,
  "monitored": true,
  "monitor_mode": "all"
}
```

---

## Items

Items are leaf content: tracks, episodes, scenes, movies.

### List items
```bash
GET /api/v1/items?contentType=music&groupId={group_id}&page=1&pageSize=50
```

### Get one item
```bash
GET /api/v1/items/{id}
```
```json
{
  "id": "itm-001",
  "content_type": "music",
  "library_entry_id": "le-001",
  "group_id": "grp-001",
  "title": "Edge of Seventeen",
  "sequence": "6",
  "runtime_seconds": 295,
  "monitored": true,
  "status": "imported",
  "media_file": {
    "id": "mf-001",
    "path": "/media/content/music/Stevie Nicks/Bella Donna/06 Edge of Seventeen.flac",
    "size": 38291042,
    "oshash": "a1b2c3d4",
    "match_confidence": "verified"
  },
  "people": [
    { "person_id": "p-001", "name": "Stevie Nicks", "role": "artist" }
  ],
  "external_ids": [
    { "source": "musicbrainz", "value": "rec-mbid-..." }
  ]
}
```

### Create an item manually
```bash
POST /api/v1/items
```
```json
{
  "content_type": "music",
  "library_entry_id": "le-001",
  "group_id": "grp-001",
  "title": "Landslide",
  "sequence": "3",
  "runtime_seconds": 199,
  "monitored": true
}
```

---

## People

Person records are cross-cutting — the same person can appear across multiple studios, series, or albums.

### Search people
```bash
GET /api/v1/people?q=Stevie&role=artist&page=1&pageSize=25
```

### Get one person
```bash
GET /api/v1/people/{id}
```
```json
{
  "id": "p-001",
  "name": "Stevie Nicks",
  "sort_name": "Nicks, Stevie",
  "overview": "...",
  "monitored": false,
  "image_path": "/media/images/people/p-001.jpg",
  "external_ids": [
    { "source": "musicbrainz", "value": "person-mbid-..." }
  ]
}
```

### Import a person from a provider
```bash
POST /api/v1/metadata/people/import
```
```json
{
  "source": "musicbrainz",
  "external_id": "person-mbid-...",
  "name": "Stevie Nicks",
  "monitored": false,
  "monitor_mode": "none"
}
```

---

## Metadata — Search

Search external providers without importing anything. Use this to find external IDs before importing.

### Search for an artist or studio
```bash
GET /api/v1/metadata/search?kind=studio&q=Stevie+Nicks&contentType=music&limit=10
```
```json
{
  "results": [
    {
      "source": "musicbrainz",
      "external_id": "8b4dba1b-...",
      "name": "Stevie Nicks",
      "image_url": "https://...",
      "overview": "..."
    }
  ]
}
```

### Search for tracks within an album
```bash
GET /api/v1/metadata/search?kind=track&source=musicbrainz&contentType=music&groupExternalId={release_group_mbid}&limit=50
```
```json
{
  "results": [
    {
      "external_id": "rec-mbid-001",
      "title": "Bella Donna",
      "sequence": "1",
      "runtime_seconds": 222,
      "source": "musicbrainz"
    }
  ]
}
```

---

## Metadata — Import

Import creates the full content tree in your library. All operations are **idempotent** — calling the same import twice returns the existing record without modification.

### Import an artist (music) or studio (adult/JAV)

This creates the correct library entry kind for the content type — `artist` for music, `studio` for adult/JAV, `series` for TV — without any caller configuration required. The kind is derived from the content type.

```bash
POST /api/v1/metadata/entries/import
```
```json
{
  "source": "musicbrainz",
  "external_id": "8b4dba1b-...",
  "name": "Stevie Nicks",
  "content_type": "music",
  "monitored": true,
  "monitor_mode": "all"
}
```
```json
{
  "entry": {
    "id": "le-001",
    "kind": "artist",
    "name": "Stevie Nicks",
    "content_type": "music",
    "image_path": "/media/images/entries/le-001.jpg"
  }
}
```

For adult content with a parent network:
```bash
POST /api/v1/metadata/entries/import
```
```json
{
  "source": "stashdb",
  "external_id": "studio-stashdb-id",
  "name": "Naughty Salamander Productions",
  "content_type": "adult",
  "monitored": true,
  "monitor_mode": "latest",
  "parent_external_id": "network-stashdb-id",
  "parent_name": "Salamander Network"
}
```
```json
{
  "entry": {
    "id": "le-002",
    "kind": "studio",
    "name": "Naughty Salamander Productions"
  },
  "network": {
    "id": "le-003",
    "kind": "network",
    "name": "Salamander Network"
  }
}
```

### Import an album

Creates the album group under an existing artist entry and imports all tracks.

```bash
POST /api/v1/metadata/albums/import
```
```json
{
  "source": "musicbrainz",
  "external_id": "rg-mbid-bella-donna",
  "library_entry_id": "le-001",
  "title": "Bella Donna",
  "year": 1981,
  "monitored": true,
  "monitor_mode": "all"
}
```
```json
{
  "id": "grp-001",
  "library_entry_id": "le-001",
  "title": "Bella Donna",
  "year": 1981
}
```

### Import a single item (track, scene, episode)

Creates the item and its full parent tree (entry + group) if they don't already exist. For music, include the album external ID so the track is placed in the correct album. If the operation takes longer than a few seconds it returns a `job_id` to poll.

```bash
POST /api/v1/metadata/items/import
```
```json
{
  "source": "musicbrainz",
  "external_id": "rec-mbid-edge-of-seventeen",
  "content_type": "music",
  "album_external_id": "rg-mbid-bella-donna",
  "album_title": "Bella Donna",
  "monitored": true
}
```
```json
{
  "item": {
    "id": "itm-001",
    "title": "Edge of Seventeen",
    "status": "wanted"
  },
  "entry": {
    "id": "le-001",
    "kind": "artist",
    "name": "Stevie Nicks"
  },
  "album": {
    "id": "grp-001",
    "title": "Bella Donna"
  }
}
```

Adult scene — the studio is created or resolved automatically:
```bash
POST /api/v1/metadata/items/import
```
```json
{
  "source": "stashdb",
  "external_id": "scene-stashdb-id",
  "content_type": "adult",
  "monitored": true
}
```
```json
{
  "item": {
    "id": "itm-002",
    "title": "The Salamander Affair",
    "status": "wanted",
    "people": [
      { "person_id": "p-002", "name": "Example Performer", "role": "performer" }
    ]
  },
  "entry": {
    "id": "le-002",
    "kind": "studio",
    "name": "Naughty Salamander Productions"
  }
}
```

---

## Discography

Fetch an artist's release groups from a provider without importing them. Use this to let the user select which albums to import.

```bash
GET /api/v1/metadata/discography?source=musicbrainz&contentType=music&externalId={artist_mbid}&page=1&pageSize=50
```
```json
{
  "results": [
    {
      "external_id": "rg-mbid-bella-donna",
      "title": "Bella Donna",
      "year": 1981,
      "primary_type": "Album",
      "image_url": "https://..."
    }
  ],
  "total": 14,
  "page": 1,
  "page_size": 50
}
```

---

## Scan

### Trigger a full scan (all enabled modules)
```bash
POST /api/v1/commands
```
```json
{ "name": "ScanAll" }
```
```json
{ "id": "job-001", "name": "ScanAll", "status": "queued" }
```

### Trigger a scan for a specific library entry
```bash
POST /api/v1/commands
```
```json
{ "name": "ScanLibrary", "entry_id": "le-001" }
```

### Check a job
```bash
GET /api/v1/commands/job-001
```
```json
{
  "id": "job-001",
  "name": "ScanAll",
  "status": "done",
  "started_at": "2026-07-04T10:00:00Z",
  "completed_at": "2026-07-04T10:00:41Z"
}
```

---

## Import Queue (Unmatched Files)

### List pending files — flat view
```bash
GET /api/v1/unmatched-files?contentType=music&status=pending
```
```json
{
  "items": [
    {
      "id": "uf-001",
      "path": "/media/content/music/Stevie Nicks/Bella Donna/01 Bella Donna.flac",
      "size": 28312044,
      "content_type": "music",
      "status": "pending",
      "discovered_at": "2026-07-04T10:00:05Z",
      "fingerprint": {
        "acoust_id": "abc...",
        "embedded_tags": {
          "title": "Bella Donna",
          "artist": "Stevie Nicks",
          "album": "Bella Donna",
          "duration_ms": "222000",
          "musicbrainz_track_id": "rec-mbid-..."
        }
      },
      "candidates": [
        {
          "confidence": 0.97,
          "source": "musicbrainz_track_id",
          "source_label": "MusicBrainz Tag",
          "source_description": "MusicBrainz recording ID found in embedded file tags",
          "item_id": "",
          "external": {
            "source": "musicbrainz",
            "external_id": "rec-mbid-...",
            "title": "Bella Donna",
            "content_type": "music",
            "runtime_seconds": 222,
            "group_external_id": "rg-mbid-bella-donna",
            "parent": {
              "source": "musicbrainz",
              "external_id": "artist-mbid-...",
              "name": "Stevie Nicks"
            }
          }
        }
      ]
    }
  ],
  "total": 10
}
```

### List pending files — grouped by album (music)
```bash
GET /api/v1/unmatched-files?contentType=music&status=pending&groupBy=album
```
```json
{
  "items": [
    {
      "group_id": "rg-mbid-bella-donna",
      "group_title": "Bella Donna",
      "best_candidate_confidence": 0.97,
      "files": [ /* same shape as flat list */ ]
    }
  ],
  "total": 1,
  "grouped_by": "album"
}
```

### Get a single queued file
```bash
GET /api/v1/unmatched-files/{id}
```
Returns the same shape as a single item from the list.

### Match a file to an existing library item
```bash
POST /api/v1/unmatched-files/{id}/match
```
```json
{ "item_id": "itm-001" }
```
Returns the updated `UnmatchedFile` with `status: "matched"`.

### Re-run identification with a custom query
```bash
POST /api/v1/unmatched-files/{id}/scrape
```
```json
{ "query": "Edge of Seventeen Stevie Nicks" }
```
```json
{
  "candidates": [ /* same shape as candidates array */ ]
}
```

### Dismiss a file (remove from pending queue)
```bash
POST /api/v1/unmatched-files/{id}/dismiss
```
Returns `200 OK` with no body.

---

## Full flows

### Flow A — Music: file on disk → library

A user drops Bella Donna FLAC files into `/media/content/music/Stevie Nicks/Bella Donna/`. After the watcher fires (or a manual scan):

1. Files appear in the import queue at `GET /api/v1/unmatched-files?contentType=music&groupBy=album`
2. The group is labelled "Bella Donna" (from candidate `group_external_id`)
3. User clicks **Import Album** in the UI. The UI calls:
   ```bash
   POST /api/v1/metadata/albums/import
   { "source": "musicbrainz", "external_id": "rg-mbid-bella-donna",
     "library_entry_id": "le-001", "title": "Bella Donna",
     "monitored": true, "monitor_mode": "all" }
   ```
   If `le-001` doesn't exist yet, the UI first calls:
   ```bash
   POST /api/v1/metadata/entries/import
   { "source": "musicbrainz", "external_id": "artist-mbid-...",
     "name": "Stevie Nicks", "content_type": "music",
     "monitored": true, "monitor_mode": "all" }
   ```
4. All tracks in the album are now in the library with `status: "wanted"`
5. For each queued file, the UI calls:
   ```bash
   POST /api/v1/unmatched-files/{file_id}/match
   { "item_id": "{matching_track_id}" }
   ```
   On transient failure, the UI retries with exponential backoff.
6. All files have `status: "matched"` and items have `status: "imported"`

### Flow B — Adult: file on disk → library

1. `.mp4` file appears in queue at `GET /api/v1/unmatched-files?contentType=adult`
2. Candidate shows a StashDB scene with performer data
3. User clicks **Import & Create**. UI calls:
   ```bash
   POST /api/v1/metadata/items/import
   { "source": "stashdb", "external_id": "scene-id",
     "content_type": "adult", "monitored": true }
   ```
4. Response contains `item.id`, `entry` (studio), and performers
5. UI calls:
   ```bash
   POST /api/v1/unmatched-files/{id}/match
   { "item_id": "{item_id}" }
   ```

### Flow C — Manual entry: no provider, no file

A user wants to manually create a music artist and album they know they have files for:

```bash
# 1. Create the artist
POST /api/v1/library-entries
{ "content_type": "music", "kind": "artist",
  "name": "The Boozy Fig Digglers", "monitored": true, "monitor_mode": "all" }

# 2. Create the album
POST /api/v1/groups
{ "library_entry_id": "{le_id}", "title": "Pickled Frequencies",
  "year": 2026, "monitored": true, "monitor_mode": "all" }

# 3. Create tracks
POST /api/v1/items
{ "content_type": "music", "library_entry_id": "{le_id}",
  "group_id": "{grp_id}", "title": "Fermented Riff No. 1",
  "sequence": "1", "runtime_seconds": 243, "monitored": true }
```

Then trigger a scan. When the file is found and matched it links to the existing item automatically.

---

## Tags

### List all tags
```bash
GET /api/v1/tags?scope=user&page=1&pageSize=50
```

### Create a tag
```bash
POST /api/v1/tags
```
```json
{ "key": "genre", "value": "Rock", "scope": "user" }
```

---

## Cache

### View cache stats
```bash
GET /api/v1/cache/stats
```
```json
{
  "caches": [
    { "name": "mbz",     "hits": 1423, "misses": 87, "size": 312, "bytes": 2048000 },
    { "name": "audiodb", "hits": 201,  "misses": 14, "size": 98,  "bytes": 512000 },
    { "name": "fanart",  "hits": 88,   "misses": 22, "size": 45,  "bytes": 204800 },
    { "name": "stashdb", "hits": 54,   "misses": 9,  "size": 31,  "bytes": 98304 },
    { "name": "acoustid","hits": 312,  "misses": 41, "size": 210, "bytes": 102400 }
  ]
}
```

### Flush a cache
```bash
DELETE /api/v1/cache/mbz
```
Returns `204 No Content`.
