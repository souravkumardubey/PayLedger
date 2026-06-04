package engine

import (
	"context"
	"errors"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/repository"
)

type Idempotency struct {
	repo repository.TransactionRepository
}

func NewIdempotency(repo repository.TransactionRepository) *Idempotency {
	return &Idempotency{repo: repo}
}

func (i *Idempotency) Check(ctx context.Context, key string) (*domain.Transaction, error) {
	if key == "" {
		return nil, nil
	}
	tx, err := i.repo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return tx, nil
}
