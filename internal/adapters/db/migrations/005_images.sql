CREATE TABLE images (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('library_entry','group','item','person')),
    entity_id   TEXT NOT NULL,
    image_type  TEXT NOT NULL CHECK (image_type IN ('poster','thumbnail','banner','hero','background')),
    url         TEXT NOT NULL,
    source      TEXT NOT NULL,
    width       INTEGER,
    height      INTEGER,
    added_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (entity_type, entity_id, image_type, url)
);

CREATE INDEX idx_images_entity ON images (entity_type, entity_id, image_type);

CREATE TABLE image_selections (
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    image_type  TEXT NOT NULL,
    image_id    TEXT REFERENCES images(id) ON DELETE SET NULL,
    PRIMARY KEY (entity_type, entity_id, image_type)
);
