CREATE TABLE music_scan_groups (
    id            TEXT PRIMARY KEY,
    folder_path   TEXT NOT NULL DEFAULT '',
    total_tracks  INTEGER NOT NULL DEFAULT 0,
    total_discs   INTEGER NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'pending',
    candidates    TEXT NOT NULL DEFAULT '[]',
    discovered_at TEXT NOT NULL
);

CREATE INDEX idx_music_scan_groups_status ON music_scan_groups(status);
