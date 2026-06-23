# Data Model

## `library_entries` — the content hierarchy

Self-referential tree. Every node can be independently monitored.

```sql
id            TEXT PRIMARY KEY
content_type  TEXT NOT NULL   -- movie|tv|music|adult|jav
kind          TEXT NOT NULL   -- network|studio|series|artist|movie
name          TEXT NOT NULL
sort_name     TEXT
overview      TEXT
parent_id     TEXT REFERENCES library_entries(id)  -- network → studio, etc.
monitored     BOOLEAN DEFAULT false
monitor_mode  TEXT DEFAULT 'all'  -- all|future|none|latest
status        TEXT              -- continuing|ended|active
quality_profile_id   TEXT
metadata_profile_id  TEXT
path          TEXT              -- root path on disk
metadata      JSON              -- type-specific fields (rating, label, etc.)
added_at      TIMESTAMP
updated_at    TIMESTAMP
```

## `groups` — intermediate groupings

Season (TV), Album (music), JAV/Adult Series.

```sql
id                TEXT PRIMARY KEY
library_entry_id  TEXT REFERENCES library_entries(id)
title             TEXT
sort_name         TEXT
number            INTEGER    -- season 1, album 2
year              INTEGER
overview          TEXT
monitored         BOOLEAN DEFAULT true
monitor_mode      TEXT DEFAULT 'all'
metadata          JSON
```

## `items` — leaf content

Episode, Scene, Track, Movie (auto-created), JAV Title.

```sql
id                TEXT PRIMARY KEY
content_type      TEXT NOT NULL
library_entry_id  TEXT REFERENCES library_entries(id)
group_id          TEXT REFERENCES groups(id)  -- nullable
title             TEXT
overview          TEXT
date              DATE
sequence          TEXT        -- "S01E05", "3", "SSIS-001", NULL for movies
runtime_seconds   INTEGER
monitored         BOOLEAN DEFAULT true
status            TEXT DEFAULT 'wanted'
                  -- wanted|grabbed|downloading|imported|missing|skipped
metadata          JSON        -- explicit, censored, BPM, content rating, etc.
added_at          TIMESTAMP
updated_at        TIMESTAMP
```

## `people` — cross-cutting persons

Performers, cast, artists, actresses. Independently monitorable across any studio.

```sql
id            TEXT PRIMARY KEY
name          TEXT NOT NULL
sort_name     TEXT
overview      TEXT
monitored     BOOLEAN DEFAULT false
monitor_mode  TEXT DEFAULT 'all'
metadata      JSON
added_at      TIMESTAMP
```

```sql
-- Name aliases (stage names, maiden names, etc.)
people_aliases (person_id, alias)

-- Who appears in what
item_people (item_id, person_id, role)
  role: performer|actress|director|actor|artist|producer
```

## `entry_people` — band/artist member relationships (music only)

```sql
library_entry_id  TEXT NOT NULL REFERENCES library_entries(id) ON DELETE CASCADE
person_id         TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE
role              TEXT NOT NULL DEFAULT 'member'
start_date        TEXT
end_date          TEXT
PRIMARY KEY (library_entry_id, person_id, role)
```

## `external_ids` — links to external databases

```sql
entity_type  TEXT  -- library_entry|group|item|person
entity_id    TEXT
source       TEXT  -- stashdb|tpdb|tmdb|tvdb|mbid|javlibrary|r18|discogs|mal|anilist
external_id  TEXT
PRIMARY KEY (entity_type, entity_id, source)
```

## `tags`

Two scopes: `user` (organizational labels) and `metadata` (genres, content tags from sources).
Key-value pairs with a unique constraint on `(key, value)`. Well-known keys: `tag` (default), `genre`, `content_warning`.

```sql
tags (id, key TEXT NOT NULL, value TEXT NOT NULL, scope, UNIQUE(key, value))
item_tags (item_id, tag_id)
entry_tags (library_entry_id, tag_id)
group_tags (group_id, tag_id)  -- albums, seasons
```

## `media_files` — files on disk

```sql
id          TEXT PRIMARY KEY
item_id     TEXT REFERENCES items(id) UNIQUE
path        TEXT NOT NULL
size        INTEGER
oshash      TEXT
md5         TEXT
quality     TEXT   -- 4K|1080p|720p|480p|SD
resolution  TEXT   -- "1920x1080"
codec       TEXT
container   TEXT
added_at    TIMESTAMP
```

## `releases` — indexer search results

```sql
id                TEXT PRIMARY KEY
title             TEXT NOT NULL   -- raw indexer title
size              INTEGER
seeders           INTEGER
leechers          INTEGER
indexer_config_id TEXT
guid              TEXT
download_url      TEXT
info_url          TEXT
published_at      TIMESTAMP
item_id           TEXT REFERENCES items(id)  -- set when matched
grabbed           BOOLEAN DEFAULT false
grabbed_at        TIMESTAMP
```

## `downloads` — in-flight and completed

```sql
id                TEXT PRIMARY KEY
release_id        TEXT REFERENCES releases(id)
item_id           TEXT REFERENCES items(id)
client_config_id  TEXT
client_job_id     TEXT
status            TEXT  -- queued|downloading|seeding|completed|failed|imported
output_path       TEXT
created_at        TIMESTAMP
updated_at        TIMESTAMP
completed_at      TIMESTAMP
```

## Config Tables

```sql
quality_profiles        (id, name, content_type, config JSON)
metadata_profiles       (id, name, content_type, config JSON)
indexer_configs         (id, name, type, enabled, priority, config JSON)
download_client_configs (id, name, type, enabled, config JSON)
metadata_source_configs (id, name, type, content_types JSON, priority, enabled, config JSON)
parser_configs          (id, name, content_type, type, priority, enabled, config JSON)
naming_configs          (id, content_type UNIQUE, folder_template, file_template)
```

## Monitoring Logic

An item is grabbed when:

```
item.monitored = true
AND item.status = 'wanted'
AND (
  -- hierarchy path: all ancestors monitored
  (library_entry.monitored AND (group.monitored OR group IS NULL))
  OR
  -- person monitoring: any performer/cast has monitored=true
  EXISTS(item_people JOIN people WHERE people.monitored = true)
)
```

`monitor_mode` on any node drives the initial `monitored` state of newly discovered children:

- `all` → new children default to `monitored=true`
- `future` → new children after `added_at` default to `monitored=true`, backfill defaults to `false`
- `none` → new children default to `monitored=false` (manual selection only)
- `latest` → only the single most recently released item is `monitored=true` at initial import; new items on subsequent refreshes arrive as `monitored=false`
