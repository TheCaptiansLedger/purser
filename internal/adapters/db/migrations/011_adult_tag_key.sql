-- Migration 011: reclassify imported scene tags from key='tag' to key='adult'.
-- All metadata-scoped key='tag' tags are adult act/position tags from the
-- StashDB importer. The 'tag' key was the 010 fallback for tags with no
-- category; adult scene tags were never assigned a category.
UPDATE tags SET key = 'adult' WHERE key = 'tag' AND scope = 'metadata';
