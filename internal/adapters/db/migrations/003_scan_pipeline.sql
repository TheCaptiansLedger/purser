-- Add match_confidence to media_files; create unmatched_files queue table.

ALTER TABLE media_files ADD COLUMN match_confidence TEXT NOT NULL DEFAULT 'name_matched';

CREATE TABLE unmatched_files (
    id            TEXT PRIMARY KEY,
    path          TEXT NOT NULL,
    size          INTEGER NOT NULL,
    content_type  TEXT NOT NULL,
    fingerprint   TEXT NOT NULL DEFAULT '{}',
    candidates    TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'pending',
    discovered_at TEXT NOT NULL
);

CREATE INDEX idx_unmatched_files_status       ON unmatched_files(status);
CREATE INDEX idx_unmatched_files_content_type ON unmatched_files(content_type);
