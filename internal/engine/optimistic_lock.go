package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/repository"
)

type OptimisticLock struct {
	repo repository.AccountRepository
}

func NewOptimisticLock(repo repository.AccountRepository) *OptimisticLock {
	return &OptimisticLock{repo: repo}
}

func (l *OptimisticLock) ReadAccounts(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
	accounts := make([]*domain.Account, len(ids))
	for i, id := range ids {
		acct, err := l.repo.GetByID(ctx, id)
		if err != nil {
			return ctx, nil, err
		}
		accounts[i] = acct
	}
	return ctx, accounts, nil
}

func (l *OptimisticLock) Commit(ctx context.Context) error {
	return nil
}

func (l *OptimisticLock) Rollback(ctx context.Context) error {
	return nil
}
