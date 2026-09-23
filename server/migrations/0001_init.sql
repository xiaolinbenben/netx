CREATE TABLE IF NOT EXISTS codes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    code       TEXT    NOT NULL UNIQUE,
    note       TEXT    NOT NULL DEFAULT '',
    status     TEXT    NOT NULL DEFAULT 'unused',
    created_at INTEGER NOT NULL,
    used_at    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_codes_status ON codes (status);

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL
);
