CREATE TABLE IF NOT EXISTS transactions (
    id              TEXT         PRIMARY KEY,
    idempotency_key TEXT         NOT NULL UNIQUE,
    type            TEXT         NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'PENDING',
    metadata        JSONB        DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id               TEXT        PRIMARY KEY,
    transaction_id   TEXT        NOT NULL REFERENCES transactions(id),
    account_id       TEXT        NOT NULL REFERENCES accounts(id),
    direction        TEXT        NOT NULL,
    amount           BIGINT      NOT NULL,
    currency         TEXT        NOT NULL,
    balance_snapshot BIGINT      NOT NULL
);

CREATE INDEX idx_ledger_entries_transaction ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_id);
