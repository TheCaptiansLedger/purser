CREATE TABLE music_releases (
    id               TEXT PRIMARY KEY,
    group_id         TEXT NOT NULL REFERENCES groups(id),
    library_entry_id TEXT NOT NULL REFERENCES library_entries(id),
    title            TEXT NOT NULL DEFAULT '',
    country          TEXT NOT NULL DEFAULT '',
    date             TEXT NOT NULL DEFAULT '',
    label            TEXT NOT NULL DEFAULT '',
    catalog_number   TEXT NOT NULL DEFAULT '',
    barcode          TEXT NOT NULL DEFAULT '',
    format           TEXT NOT NULL DEFAULT '',
    medium_count     INTEGER NOT NULL DEFAULT 0,
    track_count      INTEGER NOT NULL DEFAULT 0,
    is_default       INTEGER NOT NULL DEFAULT 0,
    monitored        INTEGER NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'stub',
    cover_path       TEXT NOT NULL DEFAULT '',
    added_at         TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX idx_music_releases_group_id         ON music_releases(group_id);
CREATE INDEX idx_music_releases_library_entry_id ON music_releases(library_entry_id);
CREATE INDEX idx_music_releases_barcode          ON music_releases(barcode);
CREATE INDEX idx_music_releases_status           ON music_releases(status);
