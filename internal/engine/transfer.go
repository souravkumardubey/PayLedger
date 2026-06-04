package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type TransferOperation struct {
	Request TransferRequest
}

func NewTransferOperation(req TransferRequest) *TransferOperation {
	return &TransferOperation{Request: req}
}

func (op *TransferOperation) Type() domain.TransactionType {
	return domain.TransactionTypeTransfer
}

func (op *TransferOperation) IdempotencyKey() string {
	return op.Request.IdempotencyKey
}

func (op *TransferOperation) AccountIDs() []string {
	return []string{op.Request.FromAccountID, op.Request.ToAccountID}
}

func (op *TransferOperation) Validate(ctx context.Context, accounts []*domain.Account) error {
	if op.Request.Amount <= 0 {
		return domain.ErrInvalidAmount
	}
	if op.Request.FromAccountID == op.Request.ToAccountID {
		return domain.ErrSelfTransfer
	}

	from, to := accounts[0], accounts[1]
	if from.Status != domain.AccountStatusActive {
		return fmt.Errorf("source account is %s", from.Status)
	}
	if to.Status != domain.AccountStatusActive {
		return fmt.Errorf("destination account is %s", to.Status)
	}
	if from.Currency != op.Request.Currency || to.Currency != op.Request.Currency {
		return domain.ErrCurrencyMismatch
	}
	if from.Balance < op.Request.Amount {
		return domain.ErrInsufficientFunds
	}
	return nil
}

func (op *TransferOperation) Apply(ctx context.Context, accounts []*domain.Account) (*domain.Transaction, error) {
	from, to := accounts[0], accounts[1]
	from.Balance -= op.Request.Amount
	to.Balance += op.Request.Amount

	now := time.Now()
	txID := newID()
	tx := &domain.Transaction{
		ID:             txID,
		IdempotencyKey: op.Request.IdempotencyKey,
		Type:           domain.TransactionTypeTransfer,
		Status:         domain.TransactionStatusCompleted,
		Metadata:       op.Request.Metadata,
		CreatedAt:      now,
		CompletedAt:    &now,
		Entries: []domain.LedgerEntry{
			{
				ID:              newID(),
				TransactionID:   txID,
				AccountID:       from.ID,
				Direction:       domain.DirectionDebit,
				Amount:          op.Request.Amount,
				Currency:        op.Request.Currency,
				BalanceSnapshot: from.Balance,
			},
			{
				ID:              newID(),
				TransactionID:   txID,
				AccountID:       to.ID,
				Direction:       domain.DirectionCredit,
				Amount:          op.Request.Amount,
				Currency:        op.Request.Currency,
				BalanceSnapshot: to.Balance,
			},
		},
	}
	return tx, nil
}
