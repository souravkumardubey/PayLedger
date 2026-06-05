package engine

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type PessimisticLock struct {
	pool *pgxpool.Pool
}

func NewPessimisticLock(pool *pgxpool.Pool) *PessimisticLock {
	return &PessimisticLock{pool: pool}
}

func (l *PessimisticLock) ReadAccounts(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return ctx, nil, err
	}

	ctx = ContextWithTx(ctx, tx)

	accounts := make([]*domain.Account, len(ids))
	for i, id := range ids {
		query := `
			SELECT id, user_id, type, currency, balance, version, status, created_at, updated_at
			FROM accounts WHERE id = $1 FOR UPDATE`

		acct := &domain.Account{}
		err := tx.QueryRow(ctx, query, id).Scan(
			&acct.ID, &acct.UserID, &acct.Type, &acct.Currency,
			&acct.Balance, &acct.Version, &acct.Status,
			&acct.CreatedAt, &acct.UpdatedAt,
		)
		if err != nil {
			tx.Rollback(ctx)
			return ctx, nil, err
		}
		accounts[i] = acct
	}

	return ctx, accounts, nil
}

func (l *PessimisticLock) Commit(ctx context.Context) error {
	tx, ok := TxFromContext(ctx)
	if !ok {
		return nil
	}
	return tx.Commit(ctx)
}

func (l *PessimisticLock) Rollback(ctx context.Context) error {
	tx, ok := TxFromContext(ctx)
	if !ok {
		return nil
	}
	return tx.Rollback(ctx)
}
