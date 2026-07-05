-- Add match_detail column to media_files for per-file match evidence.
ALTER TABLE media_files ADD COLUMN match_detail TEXT;
