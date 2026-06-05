package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type WithdrawOperation struct {
	Request DepositRequest
}

func NewWithdrawOperation(req DepositRequest) *WithdrawOperation {
	return &WithdrawOperation{Request: req}
}

func (op *WithdrawOperation) Type() domain.TransactionType {
	return domain.TransactionTypeDebit
}

func (op *WithdrawOperation) IdempotencyKey() string {
	return op.Request.IdempotencyKey
}

func (op *WithdrawOperation) AccountIDs() []string {
	return []string{op.Request.AccountID}
}

func (op *WithdrawOperation) RequestData() json.RawMessage {
	data, _ := json.Marshal(op.Request)
	return data
}

func (op *WithdrawOperation) Validate(ctx context.Context, accounts []*domain.Account) error {
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
	if acct.Balance < op.Request.Amount {
		return domain.ErrInsufficientFunds
	}
	return nil
}

func (op *WithdrawOperation) Apply(ctx context.Context, accounts []*domain.Account) (*domain.Transaction, error) {
	acct := accounts[0]
	acct.Balance -= op.Request.Amount

	now := time.Now()
	txID := newID()
	tx := &domain.Transaction{
		ID:             txID,
		IdempotencyKey: op.Request.IdempotencyKey,
		Type:           domain.TransactionTypeDebit,
		Status:         domain.TransactionStatusCompleted,
		Metadata:       op.Request.Metadata,
		CreatedAt:      now,
		CompletedAt:    &now,
		Entries: []domain.LedgerEntry{
			{
				ID:              newID(),
				TransactionID:   txID,
				AccountID:       acct.ID,
				Direction:       domain.DirectionDebit,
				Amount:          op.Request.Amount,
				Currency:        op.Request.Currency,
				BalanceSnapshot: acct.Balance,
			},
		},
	}
	return tx, nil
}
