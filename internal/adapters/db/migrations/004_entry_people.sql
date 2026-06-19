CREATE TABLE entry_people (
    library_entry_id  TEXT NOT NULL REFERENCES library_entries(id) ON DELETE CASCADE,
    person_id         TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role              TEXT NOT NULL DEFAULT 'member',
    start_date        TEXT NOT NULL DEFAULT '',
    end_date          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (library_entry_id, person_id, role)
);

CREATE INDEX idx_entry_people_entry  ON entry_people(library_entry_id);
CREATE INDEX idx_entry_people_person ON entry_people(person_id);
