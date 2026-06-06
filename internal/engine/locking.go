package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

// LockingStrategy implements the Strategy pattern for concurrency control.
// The Engine delegates account locking to the selected strategy, allowing
// callers to swap between OptimisticLock (version-based retry) and
// PessimisticLock (SELECT FOR UPDATE) without changing engine logic.
type LockingStrategy interface {
	ReadAccounts(ctx context.Context, ids []string) (context.Context, []*domain.Account, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
