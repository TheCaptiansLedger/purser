-- Add category column to tags for distinguishing genres from content warnings
-- and general metadata labels.
ALTER TABLE tags ADD COLUMN category TEXT NOT NULL DEFAULT '';
