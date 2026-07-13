CREATE TABLE IF NOT EXISTS document_index (
    collection  VARCHAR(64)  NOT NULL,
    index_key   VARCHAR(64)  NOT NULL,
    index_value VARCHAR(255) NOT NULL,
    id          VARCHAR(255) NOT NULL,
    PRIMARY KEY (collection, index_key, index_value, id)
)
