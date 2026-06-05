package engine

import (
	"context"
	"log/slog"
	"time"

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
	retryCfg        RetryConfig
	logger          *slog.Logger
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
		retryCfg:        DefaultRetryConfig(),
		logger:          slog.Default(),
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

	if len(original.Entries) == 0 {
		return nil, domain.ErrInvalidReversal
	}

	if len(original.Entries) == 1 {
		entry := original.Entries[0]
		req := DepositRequest{
			AccountID:      entry.AccountID,
			Amount:         entry.Amount,
			Currency:       entry.Currency,
			IdempotencyKey: idempotencyKey,
		}
		switch entry.Direction {
		case domain.DirectionCredit:
			return e.execute(ctx, NewWithdrawOperation(req))
		case domain.DirectionDebit:
			return e.execute(ctx, NewDepositOperation(req))
		default:
			return nil, domain.ErrInvalidReversal
		}
	}

	fromEntry, toEntry := original.Entries[0], original.Entries[1]

	fromID := fromEntry.AccountID
	toID := toEntry.AccountID
	if fromEntry.Direction == domain.DirectionDebit {
		fromID, toID = toEntry.AccountID, fromEntry.AccountID
	}

	req := TransferRequest{
		FromAccountID:  fromID,
		ToAccountID:    toID,
		Amount:         fromEntry.Amount,
		Currency:       fromEntry.Currency,
		IdempotencyKey: idempotencyKey,
	}
	return e.execute(ctx, NewTransferOperation(req))
}

func (e *Engine) execute(ctx context.Context, op Operation) (txResult *domain.Transaction, err error) {
	if existing, err := e.idempotency.Check(ctx, op.IdempotencyKey()); err != nil {
		return nil, err
	} else if existing != nil {
		e.logger.Debug("idempotent request", "key", op.IdempotencyKey())
		return existing, nil
	}

	var lastErr error
	for attempt := 0; attempt <= e.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := retryBackoff(e.retryCfg, attempt-1)
			e.logger.Info("retrying transaction",
				"attempt", attempt,
				"reason", lastErr.Error(),
				"delay", delay,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		txResult, lastErr = e.executeOnce(ctx, op)
		if lastErr == nil {
			return txResult, nil
		}
		if !isRetryableError(lastErr) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func (e *Engine) executeOnce(ctx context.Context, op Operation) (txResult *domain.Transaction, err error) {
	walEntry := NewWALEntry(op)
	if err := e.wal.Append(ctx, walEntry); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			e.locker.Rollback(ctx)
			e.wal.MarkRolledBack(ctx, walEntry.ID)
			e.logger.Warn("transaction rolled back",
				"wal_id", walEntry.ID,
				"type", op.Type(),
				"error", err,
			)
		} else {
			e.locker.Commit(ctx)
			e.wal.MarkCommitted(ctx, walEntry.ID)
			e.logger.Info("transaction committed",
				"wal_id", walEntry.ID,
				"type", op.Type(),
				"idempotency_key", op.IdempotencyKey(),
			)
		}
	}()

	var accounts []*domain.Account
	ctx, accounts, err = e.locker.ReadAccounts(ctx, op.AccountIDs())
	if err != nil {
		return nil, err
	}

	if err := op.Validate(ctx, accounts); err != nil {
		return nil, err
	}

	tx, err := op.Apply(ctx, accounts)
	if err != nil {
		return nil, err
	}

	for _, acct := range accounts {
		if err := e.accountRepo.UpdateBalance(ctx, acct); err != nil {
			return nil, err
		}
	}

	if err := e.transactionRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}
