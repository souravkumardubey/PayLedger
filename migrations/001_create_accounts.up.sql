CREATE TABLE IF NOT EXISTS accounts (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    type       TEXT        NOT NULL,
    currency   TEXT        NOT NULL,
    balance    BIGINT      NOT NULL DEFAULT 0,
    version    BIGINT      NOT NULL DEFAULT 1,
    status     TEXT        NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_user_id ON accounts(user_id);
