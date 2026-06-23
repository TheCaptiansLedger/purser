-- Migration 010: convert tags from (name UNIQUE, category) to (key, value) with UNIQUE(key, value).
-- Existing rows: category='' → key='tag'; non-empty category → key=category; name → value.
-- Adds group_tags join table for albums/seasons to carry key-value tags.

CREATE TABLE tags_new (
    id    TEXT PRIMARY KEY,
    key   TEXT NOT NULL,
    value TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'user',
    UNIQUE(key, value)
);

INSERT INTO tags_new (id, key, value, scope)
SELECT
    id,
    CASE WHEN category = '' THEN 'tag' ELSE category END,
    name,
    scope
FROM tags;

CREATE INDEX idx_tags_key   ON tags_new(key);
CREATE INDEX idx_tags_value ON tags_new(value COLLATE NOCASE);

DROP TABLE tags;
ALTER TABLE tags_new RENAME TO tags;

CREATE TABLE group_tags (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    tag_id   TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, tag_id)
);

CREATE INDEX idx_group_tags_tag_id ON group_tags(tag_id);
