package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/souravkumardubey/PayLedger/internal/engine"
)

type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (db *DB) q(ctx context.Context) Querier {
	if tx, ok := engine.TxFromContext(ctx); ok {
		return tx
	}
	return db.Pool
}
