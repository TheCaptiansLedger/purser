-- ── Content hierarchy ─────────────────────────────────────────────────────────

CREATE TABLE library_entries (
    id                  TEXT PRIMARY KEY,
    content_type        TEXT NOT NULL,
    kind                TEXT NOT NULL,
    name                TEXT NOT NULL,
    sort_name           TEXT NOT NULL DEFAULT '',
    overview            TEXT NOT NULL DEFAULT '',
    parent_id           TEXT REFERENCES library_entries(id),
    monitored           INTEGER NOT NULL DEFAULT 0,
    monitor_mode        TEXT NOT NULL DEFAULT 'all',
    status              TEXT NOT NULL DEFAULT 'active',
    quality_profile_id  TEXT NOT NULL DEFAULT '',
    metadata_profile_id TEXT NOT NULL DEFAULT '',
    path                TEXT NOT NULL DEFAULT '',
    image_path          TEXT NOT NULL DEFAULT '',
    banner_url          TEXT,
    metadata            TEXT NOT NULL DEFAULT '{}',
    locked_fields       TEXT NOT NULL DEFAULT '[]',
    added_at            TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE INDEX idx_library_entries_content_type ON library_entries(content_type);
CREATE INDEX idx_library_entries_kind         ON library_entries(kind);
CREATE INDEX idx_library_entries_parent_id    ON library_entries(parent_id);
CREATE INDEX idx_library_entries_monitored    ON library_entries(monitored);
CREATE INDEX idx_library_entries_name         ON library_entries(name COLLATE NOCASE);

CREATE TABLE groups (
    id               TEXT PRIMARY KEY,
    library_entry_id TEXT NOT NULL REFERENCES library_entries(id),
    title            TEXT NOT NULL DEFAULT '',
    sort_name        TEXT NOT NULL DEFAULT '',
    number           INTEGER NOT NULL DEFAULT 0,
    year             INTEGER NOT NULL DEFAULT 0,
    overview         TEXT NOT NULL DEFAULT '',
    monitored        INTEGER NOT NULL DEFAULT 1,
    monitor_mode     TEXT NOT NULL DEFAULT 'all',
    metadata         TEXT NOT NULL DEFAULT '{}',
    locked_fields    TEXT NOT NULL DEFAULT '[]',
    cover_path       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_groups_library_entry_id ON groups(library_entry_id);
CREATE INDEX idx_groups_number           ON groups(library_entry_id, number);

CREATE TABLE items (
    id               TEXT PRIMARY KEY,
    content_type     TEXT NOT NULL,
    library_entry_id TEXT NOT NULL REFERENCES library_entries(id),
    group_id         TEXT REFERENCES groups(id),
    title            TEXT NOT NULL DEFAULT '',
    overview         TEXT NOT NULL DEFAULT '',
    date             TEXT,
    sequence         TEXT NOT NULL DEFAULT '',
    runtime_seconds  INTEGER NOT NULL DEFAULT 0,
    monitored        INTEGER NOT NULL DEFAULT 1,
    status           TEXT NOT NULL DEFAULT 'wanted',
    cover_path       TEXT NOT NULL DEFAULT '',
    metadata         TEXT NOT NULL DEFAULT '{}',
    locked_fields    TEXT NOT NULL DEFAULT '[]',
    added_at         TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX idx_items_library_entry_id ON items(library_entry_id);
CREATE INDEX idx_items_group_id         ON items(group_id);
CREATE INDEX idx_items_content_type     ON items(content_type);
CREATE INDEX idx_items_status           ON items(status);
CREATE INDEX idx_items_monitored        ON items(monitored);
CREATE INDEX idx_items_date             ON items(date);
CREATE INDEX idx_items_title            ON items(title COLLATE NOCASE);

-- ── People ────────────────────────────────────────────────────────────────────

CREATE TABLE people (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    sort_name     TEXT NOT NULL DEFAULT '',
    overview      TEXT NOT NULL DEFAULT '',
    monitored     INTEGER NOT NULL DEFAULT 0,
    monitor_mode  TEXT NOT NULL DEFAULT 'all',
    image_path    TEXT NOT NULL DEFAULT '',
    metadata      TEXT NOT NULL DEFAULT '{}',
    locked_fields TEXT NOT NULL DEFAULT '[]',
    added_at      TEXT NOT NULL
);

CREATE INDEX idx_people_name      ON people(name COLLATE NOCASE);
CREATE INDEX idx_people_sort_name ON people(sort_name COLLATE NOCASE);
CREATE INDEX idx_people_monitored ON people(monitored);

CREATE TABLE people_aliases (
    person_id TEXT NOT NULL REFERENCES people(id),
    alias     TEXT NOT NULL,
    PRIMARY KEY (person_id, alias)
);

CREATE INDEX idx_people_aliases_alias ON people_aliases(alias COLLATE NOCASE);

CREATE TABLE person_roles (
    person_id TEXT NOT NULL REFERENCES people(id),
    role      TEXT NOT NULL,
    PRIMARY KEY (person_id, role)
);

CREATE TABLE item_people (
    item_id   TEXT NOT NULL REFERENCES items(id),
    person_id TEXT NOT NULL REFERENCES people(id),
    role      TEXT NOT NULL,
    PRIMARY KEY (item_id, person_id, role)
);

CREATE INDEX idx_item_people_person_id ON item_people(person_id);

CREATE TABLE entry_people (
    library_entry_id TEXT NOT NULL REFERENCES library_entries(id),
    person_id        TEXT NOT NULL REFERENCES people(id),
    role             TEXT NOT NULL DEFAULT 'member',
    start_date       TEXT NOT NULL DEFAULT '',
    end_date         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (library_entry_id, person_id, role)
);

CREATE INDEX idx_entry_people_entry  ON entry_people(library_entry_id);
CREATE INDEX idx_entry_people_person ON entry_people(person_id);

-- ── External IDs ──────────────────────────────────────────────────────────────

CREATE TABLE external_ids (
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    source      TEXT NOT NULL,
    value       TEXT NOT NULL,
    PRIMARY KEY (entity_type, entity_id, source)
);

CREATE INDEX idx_external_ids_entity       ON external_ids(entity_type, entity_id);
CREATE INDEX idx_external_ids_source_value ON external_ids(source, value);

-- ── Tags ──────────────────────────────────────────────────────────────────────

CREATE TABLE tags (
    id    TEXT PRIMARY KEY,
    key   TEXT NOT NULL,
    value TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'user',
    UNIQUE(key, value)
);

CREATE INDEX idx_tags_key   ON tags(key);
CREATE INDEX idx_tags_value ON tags(value COLLATE NOCASE);

CREATE TABLE item_tags (
    item_id TEXT NOT NULL REFERENCES items(id),
    tag_id  TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (item_id, tag_id)
);

CREATE TABLE entry_tags (
    library_entry_id TEXT NOT NULL REFERENCES library_entries(id),
    tag_id           TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (library_entry_id, tag_id)
);

CREATE TABLE group_tags (
    group_id TEXT NOT NULL REFERENCES groups(id),
    tag_id   TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (group_id, tag_id)
);

CREATE INDEX idx_group_tags_tag_id ON group_tags(tag_id);

-- ── Media files ───────────────────────────────────────────────────────────────

CREATE TABLE media_files (
    id         TEXT PRIMARY KEY,
    item_id    TEXT NOT NULL UNIQUE REFERENCES items(id),
    path       TEXT NOT NULL UNIQUE,
    size       INTEGER NOT NULL DEFAULT 0,
    oshash     TEXT NOT NULL DEFAULT '',
    md5        TEXT NOT NULL DEFAULT '',
    quality    TEXT NOT NULL DEFAULT '',
    resolution TEXT NOT NULL DEFAULT '',
    codec      TEXT NOT NULL DEFAULT '',
    container  TEXT NOT NULL DEFAULT '',
    added_at   TEXT NOT NULL
);

CREATE INDEX idx_media_files_oshash ON media_files(oshash) WHERE oshash != '';
CREATE INDEX idx_media_files_md5    ON media_files(md5)    WHERE md5 != '';

-- ── Settings ──────────────────────────────────────────────────────────────────

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- ── Phase 2: Configuration ────────────────────────────────────────────────────

CREATE TABLE quality_profiles (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    config       TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE metadata_profiles (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    config       TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE indexer_configs (
    id       TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    type     TEXT NOT NULL DEFAULT 'prowlarr',
    enabled  INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    config   TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE download_client_configs (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    type    TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    config  TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE metadata_source_configs (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL,
    content_types TEXT NOT NULL DEFAULT '[]',
    priority      INTEGER NOT NULL DEFAULT 0,
    enabled       INTEGER NOT NULL DEFAULT 1,
    config        TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE parser_configs (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL,
    priority     INTEGER NOT NULL DEFAULT 0,
    enabled      INTEGER NOT NULL DEFAULT 1,
    config       TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE naming_configs (
    id              TEXT PRIMARY KEY,
    content_type    TEXT NOT NULL UNIQUE,
    folder_template TEXT NOT NULL DEFAULT '',
    file_template   TEXT NOT NULL DEFAULT ''
);

-- ── Phase 2: Acquisition pipeline ─────────────────────────────────────────────

CREATE TABLE releases (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    size              INTEGER NOT NULL DEFAULT 0,
    seeders           INTEGER NOT NULL DEFAULT 0,
    leechers          INTEGER NOT NULL DEFAULT 0,
    indexer_config_id TEXT NOT NULL DEFAULT '',
    guid              TEXT NOT NULL DEFAULT '',
    download_url      TEXT NOT NULL DEFAULT '',
    info_url          TEXT NOT NULL DEFAULT '',
    published_at      TEXT,
    item_id           TEXT REFERENCES items(id),
    grabbed           INTEGER NOT NULL DEFAULT 0,
    grabbed_at        TEXT
);

CREATE INDEX idx_releases_item_id ON releases(item_id);
CREATE INDEX idx_releases_grabbed ON releases(grabbed);

CREATE TABLE downloads (
    id               TEXT PRIMARY KEY,
    release_id       TEXT NOT NULL REFERENCES releases(id),
    item_id          TEXT NOT NULL REFERENCES items(id),
    client_config_id TEXT NOT NULL DEFAULT '',
    client_job_id    TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'queued',
    output_path      TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    completed_at     TEXT
);

CREATE INDEX idx_downloads_item_id ON downloads(item_id);
CREATE INDEX idx_downloads_status  ON downloads(status);
