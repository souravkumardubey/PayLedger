package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type mockAccountRepo struct {
	createFunc         func(ctx context.Context, account *domain.Account) error
	getByIDFunc        func(ctx context.Context, id string) (*domain.Account, error)
	updateBalanceFunc  func(ctx context.Context, account *domain.Account) error
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, account)
	}
	return nil
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *mockAccountRepo) UpdateBalance(ctx context.Context, account *domain.Account) error {
	if m.updateBalanceFunc != nil {
		return m.updateBalanceFunc(ctx, account)
	}
	return nil
}

type mockTxnRepo struct {
	getByIdempotencyKeyFunc func(ctx context.Context, key string) (*domain.Transaction, error)
	createFunc              func(ctx context.Context, tx *domain.Transaction) error
	getByIDFunc             func(ctx context.Context, id string) (*domain.Transaction, error)
	updateStatusFunc        func(ctx context.Context, id string, status domain.TransactionStatus) error
}

func (m *mockTxnRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, domain.ErrTransactionNotFound
}

func (m *mockTxnRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	return m.getByIdempotencyKeyFunc(ctx, key)
}

func (m *mockTxnRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx)
	}
	return nil
}

func (m *mockTxnRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(ctx, id, status)
	}
	return nil
}

func (m *mockTxnRepo) ListByAccountID(ctx context.Context, accountID string, page, limit int) ([]domain.Transaction, int, error) {
	return nil, 0, nil
}

type mockLocker struct {
	readAccountsFunc func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error)
	commitFunc       func(ctx context.Context) error
	rollbackFunc     func(ctx context.Context) error
}

func (m *mockLocker) ReadAccounts(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
	return m.readAccountsFunc(ctx, ids)
}

func (m *mockLocker) Commit(ctx context.Context) error {
	if m.commitFunc != nil {
		return m.commitFunc(ctx)
	}
	return nil
}

func (m *mockLocker) Rollback(ctx context.Context) error {
	if m.rollbackFunc != nil {
		return m.rollbackFunc(ctx)
	}
	return nil
}

type mockWAL struct {
	appendFunc         func(ctx context.Context, entry *WALEntry) error
	markCommittedFunc  func(ctx context.Context, id string) error
	markRolledBackFunc func(ctx context.Context, id string) error
}

func (m *mockWAL) Append(ctx context.Context, entry *WALEntry) error {
	if m.appendFunc != nil {
		return m.appendFunc(ctx, entry)
	}
	return nil
}

func (m *mockWAL) MarkCommitted(ctx context.Context, id string) error {
	if m.markCommittedFunc != nil {
		return m.markCommittedFunc(ctx, id)
	}
	return nil
}

func (m *mockWAL) MarkRolledBack(ctx context.Context, id string) error {
	if m.markRolledBackFunc != nil {
		return m.markRolledBackFunc(ctx, id)
	}
	return nil
}

func (m *mockWAL) GetPending(ctx context.Context) ([]WALEntry, error) {
	return nil, nil
}

func newTestAccount(id, userID string, balance int64, currency domain.Currency) *domain.Account {
	return &domain.Account{
		ID:        id,
		UserID:    userID,
		Type:      domain.AccountTypeChecking,
		Currency:  currency,
		Balance:   balance,
		Version:   1,
		Status:    domain.AccountStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestEngine_Deposit_Success(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 0, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			return acc, nil
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
		createFunc: func(ctx context.Context, tx *domain.Transaction) error {
			if tx.Type != domain.TransactionTypeCredit {
				t.Errorf("expected CREDIT, got %s", tx.Type)
			}
			if len(tx.Entries) != 1 {
				t.Errorf("expected 1 entry, got %d", len(tx.Entries))
			}
			return nil
		},
	}

	var walCommitted bool
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error {
			if entry.Status != WALPending {
				t.Errorf("expected PENDING, got %s", entry.Status)
			}
			return nil
		},
		markCommittedFunc: func(ctx context.Context, id string) error {
			walCommitted = true
			return nil
		},
	}

	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil transaction")
	}
	if result.Status != domain.TransactionStatusCompleted {
		t.Errorf("expected COMPLETED, got %s", result.Status)
	}
	if !walCommitted {
		t.Error("expected WAL to be committed")
	}
	if acc.Balance != 100000 {
		t.Errorf("expected balance 100000, got %d", acc.Balance)
	}
}

func TestEngine_Withdraw_Success(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			return acc, nil
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error {
			return nil
		},
		markCommittedFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.Withdraw(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         40000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "wd-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil transaction")
	}
	if acc.Balance != 60000 {
		t.Errorf("expected balance 60000, got %d", acc.Balance)
	}
}

func TestEngine_Transfer_Success(t *testing.T) {
	from := newTestAccount("from-1", "user-1", 100000, domain.CurrencyINR)
	to := newTestAccount("to-1", "user-2", 0, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{
		updateBalanceFunc: func(ctx context.Context, acct *domain.Account) error {
			return nil
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc:         func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc:  func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{from, to}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.Transfer(context.Background(), TransferRequest{
		FromAccountID:  "from-1",
		ToAccountID:    "to-1",
		Amount:         50000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "tx-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil transaction")
	}
	if from.Balance != 50000 {
		t.Errorf("from balance: expected 50000, got %d", from.Balance)
	}
	if to.Balance != 50000 {
		t.Errorf("to balance: expected 50000, got %d", to.Balance)
	}
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Entries))
	}
}

func TestEngine_Idempotency_ReturnsExisting(t *testing.T) {
	existing := &domain.Transaction{
		ID:             "existing-tx",
		IdempotencyKey: "dup-key",
		Type:           domain.TransactionTypeCredit,
		Status:         domain.TransactionStatusCompleted,
		CreatedAt:      time.Now(),
	}

	var walAppended bool
	accountRepo := &mockAccountRepo{}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return existing, nil
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error {
			walAppended = true
			return nil
		},
	}
	locker := &mockLocker{}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dup-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != existing {
		t.Error("expected existing transaction to be returned")
	}
	if walAppended {
		t.Error("WAL should not be appended for idempotent requests")
	}
}

func TestEngine_InvalidAmount(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         0,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-invalid",
	})
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestEngine_InsufficientFunds(t *testing.T) {
	from := newTestAccount("from-1", "user-1", 100, domain.CurrencyINR)
	to := newTestAccount("to-1", "user-2", 0, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{from, to}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Transfer(context.Background(), TransferRequest{
		FromAccountID:  "from-1",
		ToAccountID:    "to-1",
		Amount:         500,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "tx-insuf",
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestEngine_CurrencyMismatch(t *testing.T) {
	from := newTestAccount("from-1", "user-1", 100000, domain.CurrencyINR)
	to := newTestAccount("to-1", "user-2", 0, domain.CurrencyUSD)

	accountRepo := &mockAccountRepo{}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{from, to}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Transfer(context.Background(), TransferRequest{
		FromAccountID:  "from-1",
		ToAccountID:    "to-1",
		Amount:         50000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "tx-cur",
	})
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Errorf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestEngine_SelfTransfer(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc, acc}, nil
		},
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Transfer(context.Background(), TransferRequest{
		FromAccountID:  "acc-1",
		ToAccountID:    "acc-1",
		Amount:         50000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "tx-self",
	})
	if !errors.Is(err, domain.ErrSelfTransfer) {
		t.Errorf("expected ErrSelfTransfer, got %v", err)
	}
}

func TestEngine_AccountNotFound(t *testing.T) {
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, nil, domain.ErrAccountNotFound
		},
		rollbackFunc: func(ctx context.Context) error { return nil },
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "nonexistent",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-nf",
	})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestEngine_WALRolledBackOnError(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	accountRepo := &mockAccountRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			return acc, nil
		},
		updateBalanceFunc: func(ctx context.Context, acct *domain.Account) error {
			return errors.New("db error")
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}

	var walRolledBack bool
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error {
			walRolledBack = true
			return nil
		},
	}

	var lockerRolledBack bool
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
		rollbackFunc: func(ctx context.Context) error {
			lockerRolledBack = true
			return nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-fail",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !walRolledBack {
		t.Error("expected WAL to be rolled back on error")
	}
	if !lockerRolledBack {
		t.Error("expected locker to be rolled back on error")
	}
}

func TestEngine_FrozenAccount(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)
	acc.Status = domain.AccountStatusFrozen

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc: func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Withdraw(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         10000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "wd-frozen",
	})
	if err == nil {
		t.Fatal("expected error for frozen account")
	}
}

func TestEngine_RetryOnDeadlock(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	callCount := 0
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			callCount++
			if callCount == 1 {
				return ctx, nil, &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}
			}
			return ctx, []*domain.Account{acc}, nil
		},
		commitFunc:   func(ctx context.Context) error { return nil },
		rollbackFunc: func(ctx context.Context) error { return nil },
	}

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc:         func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc:  func(ctx context.Context, id string) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-retry",
	})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
}

func TestEngine_RetryOnSerializationFailure(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	callCount := 0
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			callCount++
			if callCount <= 2 {
				return ctx, nil, &pgconn.PgError{Code: "40001", Message: "serialization failure"}
			}
			return ctx, []*domain.Account{acc}, nil
		},
		commitFunc:   func(ctx context.Context) error { return nil },
		rollbackFunc: func(ctx context.Context) error { return nil },
	}

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc:         func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc:  func(ctx context.Context, id string) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-retry-ser",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 retries), got %d", callCount)
	}
}

func TestEngine_NonRetryableErrorNotRetried(t *testing.T) {
	callCount := 0
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			callCount++
			return ctx, nil, &pgconn.PgError{Code: "23505", Message: "unique violation"}
		},
		rollbackFunc: func(ctx context.Context) error { return nil },
	}

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc:         func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-no-retry",
	})
	if err == nil {
		t.Fatal("expected error for non-retryable PG error")
	}
	if callCount != 1 {
		t.Errorf("expected only 1 call (no retry), got %d", callCount)
	}
}

func TestEngine_MaxRetriesExhausted(t *testing.T) {
	callCount := 0
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			callCount++
			return ctx, nil, &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}
		},
		rollbackFunc: func(ctx context.Context) error { return nil },
	}

	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	wal := &mockWAL{
		appendFunc:         func(ctx context.Context, entry *WALEntry) error { return nil },
		markRolledBackFunc: func(ctx context.Context, id string) error { return nil },
	}

	eng := New(&mockAccountRepo{}, txnRepo, locker, NewIdempotency(txnRepo), wal)

	_, err := eng.Deposit(context.Background(), DepositRequest{
		AccountID:      "acc-1",
		Amount:         100000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "dep-exhausted",
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	expected := eng.retryCfg.MaxRetries + 1
	if callCount != expected {
		t.Errorf("expected %d calls (max retries), got %d", expected, callCount)
	}
}

func TestEngine_ReverseTransfer(t *testing.T) {
	from := newTestAccount("from-1", "user-1", 50000, domain.CurrencyINR)
	to := newTestAccount("to-1", "user-2", 50000, domain.CurrencyINR)

	original := &domain.Transaction{
		ID:             "orig-tx",
		Type:           domain.TransactionTypeTransfer,
		Status:         domain.TransactionStatusCompleted,
		IdempotencyKey: "orig-key",
		Entries: []domain.LedgerEntry{
			{AccountID: "from-1", Direction: domain.DirectionDebit, Amount: 50000, Currency: domain.CurrencyINR},
			{AccountID: "to-1", Direction: domain.DirectionCredit, Amount: 50000, Currency: domain.CurrencyINR},
		},
	}

	txnRepo := &mockTxnRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			return original, nil
		},
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}

	wal := &mockWAL{
		appendFunc:        func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc: func(ctx context.Context, id string) error { return nil },
	}

	accounts := map[string]*domain.Account{"from-1": from, "to-1": to}
	accountRepo := &mockAccountRepo{
		updateBalanceFunc: func(ctx context.Context, acct *domain.Account) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			result := make([]*domain.Account, len(ids))
			for i, id := range ids {
				result[i] = accounts[id]
			}
			return ctx, result, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.ReverseTransaction(context.Background(), "orig-tx", "rev-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if from.Balance != 100000 {
		t.Errorf("from balance: expected 100000, got %d", from.Balance)
	}
	if to.Balance != 0 {
		t.Errorf("to balance: expected 0, got %d", to.Balance)
	}
}

func TestEngine_ReverseDeposit(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 100000, domain.CurrencyINR)

	original := &domain.Transaction{
		ID:             "dep-tx",
		Type:           domain.TransactionTypeCredit,
		Status:         domain.TransactionStatusCompleted,
		IdempotencyKey: "dep-key",
		Entries: []domain.LedgerEntry{
			{AccountID: "acc-1", Direction: domain.DirectionCredit, Amount: 50000, Currency: domain.CurrencyINR},
		},
	}

	txnRepo := &mockTxnRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			return original, nil
		},
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}

	wal := &mockWAL{
		appendFunc:        func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc: func(ctx context.Context, id string) error { return nil },
	}

	accountRepo := &mockAccountRepo{
		updateBalanceFunc: func(ctx context.Context, acct *domain.Account) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.ReverseTransaction(context.Background(), "dep-tx", "rev-dep")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if acc.Balance != 50000 {
		t.Errorf("balance: expected 50000 (deposit reversed), got %d", acc.Balance)
	}
}

func TestEngine_ReverseWithdrawal(t *testing.T) {
	acc := newTestAccount("acc-1", "user-1", 10000, domain.CurrencyINR)

	original := &domain.Transaction{
		ID:             "wd-tx",
		Type:           domain.TransactionTypeDebit,
		Status:         domain.TransactionStatusCompleted,
		IdempotencyKey: "wd-key",
		Entries: []domain.LedgerEntry{
			{AccountID: "acc-1", Direction: domain.DirectionDebit, Amount: 5000, Currency: domain.CurrencyINR},
		},
	}

	txnRepo := &mockTxnRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			return original, nil
		},
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}

	wal := &mockWAL{
		appendFunc:        func(ctx context.Context, entry *WALEntry) error { return nil },
		markCommittedFunc: func(ctx context.Context, id string) error { return nil },
	}

	accountRepo := &mockAccountRepo{
		updateBalanceFunc: func(ctx context.Context, acct *domain.Account) error { return nil },
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			return ctx, []*domain.Account{acc}, nil
		},
	}

	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), wal)

	result, err := eng.ReverseTransaction(context.Background(), "wd-tx", "rev-wd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if acc.Balance != 15000 {
		t.Errorf("balance: expected 15000 (withdrawal reversed), got %d", acc.Balance)
	}
}

func TestEngine_ReverseNoEntries(t *testing.T) {
	original := &domain.Transaction{
		ID:     "empty-tx",
		Type:   domain.TransactionTypeCredit,
		Status: domain.TransactionStatusCompleted,
	}

	txnRepo := &mockTxnRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			return original, nil
		},
	}

	eng := New(&mockAccountRepo{}, txnRepo, &mockLocker{}, NewIdempotency(txnRepo), &mockWAL{})

	_, err := eng.ReverseTransaction(context.Background(), "empty-tx", "rev-empty")
	if !errors.Is(err, domain.ErrInvalidReversal) {
		t.Errorf("expected ErrInvalidReversal, got %v", err)
	}
}
