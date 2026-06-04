package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type DepositOperation struct {
	Request DepositRequest
}

func NewDepositOperation(req DepositRequest) *DepositOperation {
	return &DepositOperation{Request: req}
}

func (op *DepositOperation) Type() domain.TransactionType {
	return domain.TransactionTypeCredit
}

func (op *DepositOperation) IdempotencyKey() string {
	return op.Request.IdempotencyKey
}

func (op *DepositOperation) AccountIDs() []string {
	return []string{op.Request.AccountID}
}

func (op *DepositOperation) Validate(ctx context.Context, accounts []*domain.Account) error {
	if op.Request.Amount <= 0 {
		return domain.ErrInvalidAmount
	}

	acct := accounts[0]
	if acct.Status != domain.AccountStatusActive {
		return fmt.Errorf("account is %s", acct.Status)
	}
	if acct.Currency != op.Request.Currency {
		return domain.ErrCurrencyMismatch
	}
	return nil
}

func (op *DepositOperation) Apply(ctx context.Context, accounts []*domain.Account) (*domain.Transaction, error) {
	acct := accounts[0]
	acct.Balance += op.Request.Amount

	now := time.Now()
	txID := newID()
	tx := &domain.Transaction{
		ID:             txID,
		IdempotencyKey: op.Request.IdempotencyKey,
		Type:           domain.TransactionTypeCredit,
		Status:         domain.TransactionStatusCompleted,
		Metadata:       op.Request.Metadata,
		CreatedAt:      now,
		CompletedAt:    &now,
		Entries: []domain.LedgerEntry{
			{
				ID:              newID(),
				TransactionID:   txID,
				AccountID:       acct.ID,
				Direction:       domain.DirectionCredit,
				Amount:          op.Request.Amount,
				Currency:        op.Request.Currency,
				BalanceSnapshot: acct.Balance,
			},
		},
	}
	return tx, nil
}
