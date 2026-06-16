-- Add image path columns for artwork storage.
-- Images are stored on disk; these columns record the relative path
-- within the configured images.path directory.

ALTER TABLE people          ADD COLUMN image_path TEXT NOT NULL DEFAULT '';
ALTER TABLE library_entries ADD COLUMN image_path TEXT NOT NULL DEFAULT '';
ALTER TABLE items           ADD COLUMN cover_path  TEXT NOT NULL DEFAULT '';
