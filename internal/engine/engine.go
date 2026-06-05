package engine

import (
	"context"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/repository"
)

type TransferRequest struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64
	Currency       domain.Currency
	IdempotencyKey string
	Metadata       map[string]string
}

type DepositRequest struct {
	AccountID      string
	Amount         int64
	Currency       domain.Currency
	IdempotencyKey string
	Metadata       map[string]string
}

type Engine struct {
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	locker          LockingStrategy
	idempotency     *Idempotency
	wal             WALStore
}

func New(
	accountRepo repository.AccountRepository,
	transactionRepo repository.TransactionRepository,
	locker LockingStrategy,
	idempotency *Idempotency,
	wal WALStore,
) *Engine {
	return &Engine{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		locker:          locker,
		idempotency:     idempotency,
		wal:             wal,
	}
}

func (e *Engine) Transfer(ctx context.Context, req TransferRequest) (*domain.Transaction, error) {
	op := NewTransferOperation(req)
	return e.execute(ctx, op)
}

func (e *Engine) Deposit(ctx context.Context, req DepositRequest) (*domain.Transaction, error) {
	op := NewDepositOperation(req)
	return e.execute(ctx, op)
}

func (e *Engine) Withdraw(ctx context.Context, req DepositRequest) (*domain.Transaction, error) {
	op := NewWithdrawOperation(req)
	return e.execute(ctx, op)
}

func (e *Engine) ReverseTransaction(ctx context.Context, originalTxID string, idempotencyKey string) (*domain.Transaction, error) {
	original, err := e.transactionRepo.GetByID(ctx, originalTxID)
	if err != nil {
		return nil, err
	}

	if len(original.Entries) != 2 {
		return nil, nil
	}

	fromEntry, toEntry := original.Entries[0], original.Entries[1]

	fromID := ""
	toID := ""
	var amount int64
	var currency domain.Currency

	if fromEntry.Direction == domain.DirectionDebit {
		fromID = fromEntry.AccountID
		toID = toEntry.AccountID
	} else {
		fromID = toEntry.AccountID
		toID = fromEntry.AccountID
	}
	amount = fromEntry.Amount
	currency = fromEntry.Currency

	req := TransferRequest{
		FromAccountID:  toID,
		ToAccountID:    fromID,
		Amount:         amount,
		Currency:       currency,
		IdempotencyKey: idempotencyKey,
	}
	op := NewTransferOperation(req)
	return e.execute(ctx, op)
}

func (e *Engine) execute(ctx context.Context, op Operation) (*domain.Transaction, error) {
	if existing, err := e.idempotency.Check(ctx, op.IdempotencyKey()); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	walEntry := NewWALEntry(op)
	if err := e.wal.Append(ctx, walEntry); err != nil {
		return nil, err
	}

	accounts, err := e.locker.ReadAccounts(ctx, op.AccountIDs())
	if err != nil {
		e.wal.MarkRolledBack(ctx, walEntry.ID)
		return nil, err
	}

	if err := op.Validate(ctx, accounts); err != nil {
		e.wal.MarkRolledBack(ctx, walEntry.ID)
		return nil, err
	}

	tx, err := op.Apply(ctx, accounts)
	if err != nil {
		e.wal.MarkRolledBack(ctx, walEntry.ID)
		return nil, err
	}

	for _, acct := range accounts {
		if err := e.accountRepo.UpdateBalance(ctx, acct); err != nil {
			e.wal.MarkRolledBack(ctx, walEntry.ID)
			return nil, err
		}
	}

	if err := e.transactionRepo.Create(ctx, tx); err != nil {
		e.wal.MarkRolledBack(ctx, walEntry.ID)
		return nil, err
	}

	if err := e.wal.MarkCommitted(ctx, walEntry.ID); err != nil {
		return nil, err
	}

	return tx, nil
}
