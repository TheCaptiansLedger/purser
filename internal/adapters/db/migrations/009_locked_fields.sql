ALTER TABLE library_entries ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE groups         ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE items          ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE people         ADD COLUMN locked_fields TEXT NOT NULL DEFAULT '[]';
