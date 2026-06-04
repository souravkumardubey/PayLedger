package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type PessimisticLock struct{}

func NewPessimisticLock() *PessimisticLock {
	return &PessimisticLock{}
}

func (l *PessimisticLock) ReadAccounts(ctx context.Context, ids []string) ([]*domain.Account, error) {
	return nil, nil
}
