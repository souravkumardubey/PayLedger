package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type LockingStrategy interface {
	ReadAccounts(ctx context.Context, ids []string) ([]*domain.Account, error)
}
