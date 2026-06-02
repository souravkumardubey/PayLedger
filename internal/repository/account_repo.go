package repository

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account) error
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	UpdateBalance(ctx context.Context, account *domain.Account) error
}
