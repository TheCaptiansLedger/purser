CREATE TABLE IF NOT EXISTS documents (
    collection VARCHAR(64)  NOT NULL,
    id         VARCHAR(255) NOT NULL,
    data       TEXT NOT NULL,
    index_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (collection, id)
)
