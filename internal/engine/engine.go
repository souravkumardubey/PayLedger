package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type TransferRequest struct {
	FromAccountID string
	ToAccountID   string
	Amount        int64
	Currency      domain.Currency
	IdempotencyKey string
	Metadata      map[string]string
}

type DepositRequest struct {
	AccountID     string
	Amount        int64
	Currency      domain.Currency
	IdempotencyKey string
	Metadata      map[string]string
}

type Engine interface {
	Transfer(ctx context.Context, req TransferRequest) (*domain.Transaction, error)
	Deposit(ctx context.Context, req DepositRequest) (*domain.Transaction, error)
	Withdraw(ctx context.Context, req DepositRequest) (*domain.Transaction, error)
	ReverseTransaction(ctx context.Context, originalTxID string, idempotencyKey string) (*domain.Transaction, error)
}
