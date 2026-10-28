CREATE TABLE IF NOT EXISTS fetch_proxy (
    id       INTEGER PRIMARY KEY CHECK (id = 1),
    enabled  INTEGER NOT NULL,
    host     TEXT    NOT NULL,
    port     INTEGER NOT NULL,
    username TEXT    NOT NULL DEFAULT '',
    password TEXT    NOT NULL DEFAULT ''
);
