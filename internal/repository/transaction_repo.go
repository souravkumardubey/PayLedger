package repository

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx *domain.Transaction) error
	GetByID(ctx context.Context, id string) (*domain.Transaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)
	UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error
	ListByAccountID(ctx context.Context, accountID string, page, limit int) ([]domain.Transaction, int, error)
}
