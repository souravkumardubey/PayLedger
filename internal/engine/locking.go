package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type LockingStrategy interface {
	ReadAccounts(ctx context.Context, ids []string) (context.Context, []*domain.Account, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
