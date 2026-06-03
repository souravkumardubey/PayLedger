package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

const migrationAccounts = `
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

CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
`

const migrationTransactions = `
CREATE TABLE IF NOT EXISTS transactions (
    id              TEXT         PRIMARY KEY,
    idempotency_key TEXT         NOT NULL UNIQUE,
    type            TEXT         NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'PENDING',
    metadata        JSONB        DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id               TEXT        PRIMARY KEY,
    transaction_id   TEXT        NOT NULL REFERENCES transactions(id),
    account_id       TEXT        NOT NULL REFERENCES accounts(id),
    direction        TEXT        NOT NULL,
    amount           BIGINT      NOT NULL,
    currency         TEXT        NOT NULL,
    balance_snapshot BIGINT      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_transaction ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account ON ledger_entries(account_id);
`

func (db *DB) RunMigrations(ctx context.Context) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{"001_create_accounts", migrationAccounts},
		{"002_create_transactions", migrationTransactions},
	}

	for _, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return nil
}
