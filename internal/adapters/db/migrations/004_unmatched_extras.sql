-- Add duplicate tracking and thumbnail cache fields to unmatched_files.
ALTER TABLE unmatched_files ADD COLUMN duplicate_of    TEXT NOT NULL DEFAULT '';
ALTER TABLE unmatched_files ADD COLUMN thumbnail_path  TEXT NOT NULL DEFAULT '';
