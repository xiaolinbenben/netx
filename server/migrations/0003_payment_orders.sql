CREATE TABLE IF NOT EXISTS payment_orders (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    out_trade_no  TEXT    NOT NULL UNIQUE,
    trade_no      TEXT    NOT NULL DEFAULT '',
    plan          TEXT    NOT NULL,
    amount_fen    INTEGER NOT NULL,
    code_id       INTEGER NOT NULL,
    status        TEXT    NOT NULL DEFAULT 'pending',
    created_at    INTEGER NOT NULL,
    paid_at       INTEGER,
    FOREIGN KEY (code_id) REFERENCES codes(id)
);

CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders (status);
