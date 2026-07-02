-- Add pre-computed sort keys so adapters can paginate without re-implementing sort logic.
-- The domain layer owns the computation; adapters store the result and ORDER BY it.

ALTER TABLE items           ADD COLUMN sort_key TEXT NOT NULL DEFAULT '';
ALTER TABLE groups          ADD COLUMN sort_key TEXT NOT NULL DEFAULT '';
ALTER TABLE library_entries ADD COLUMN sort_key TEXT NOT NULL DEFAULT '';
ALTER TABLE people          ADD COLUMN sort_key TEXT NOT NULL DEFAULT '';
ALTER TABLE tags            ADD COLUMN sort_key TEXT NOT NULL DEFAULT '';

-- Backfill items: date|sequence|title (matches domain.ItemSortKey)
UPDATE items SET sort_key = COALESCE(date, '1970-04-10') || '|' || sequence || '|' || title;

-- Backfill groups: zero-padded number|title (matches domain.GroupSortKey)
UPDATE groups SET sort_key = printf('%010d', number) || '|' || title;

-- Backfill library_entries: sort_name with name fallback (matches domain.NameSortKey)
UPDATE library_entries SET sort_key = CASE WHEN sort_name != '' THEN sort_name ELSE name END;

-- Backfill people: sort_name with name fallback (matches domain.NameSortKey)
UPDATE people SET sort_key = CASE WHEN sort_name != '' THEN sort_name ELSE name END;

-- Backfill tags: key|value (matches domain.TagSortKey)
UPDATE tags SET sort_key = key || '|' || value;

CREATE INDEX idx_items_sort_key            ON items(sort_key);
CREATE INDEX idx_groups_sort_key           ON groups(library_entry_id, sort_key);
CREATE INDEX idx_library_entries_sort_key  ON library_entries(sort_key);
CREATE INDEX idx_people_sort_key           ON people(sort_key);
CREATE INDEX idx_tags_sort_key             ON tags(sort_key);
