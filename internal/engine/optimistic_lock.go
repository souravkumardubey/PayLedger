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

func (l *OptimisticLock) ReadAccounts(ctx context.Context, ids []string) ([]*domain.Account, error) {
	accounts := make([]*domain.Account, len(ids))
	for i, id := range ids {
		acct, err := l.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		accounts[i] = acct
	}
	return accounts, nil
}
