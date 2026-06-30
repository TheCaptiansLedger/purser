CREATE TABLE IF NOT EXISTS person_roles (
    person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT NOT NULL,
    PRIMARY KEY (person_id, role)
);
