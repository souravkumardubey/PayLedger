package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/engine"
	"github.com/souravkumardubey/PayLedger/internal/repository/postgres"
)

func TestEngine_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("set TEST_DB_DSN to run integration tests")
	}

	ctx := context.Background()

	db, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()
	defer db.Pool.Exec(ctx, `DROP TABLE IF EXISTS wal_entries, ledger_entries, transactions, accounts`)

	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	accountRepo := postgres.NewAccountRepo(db)
	txnRepo := postgres.NewTransactionRepo(db)
	walStore := postgres.NewWALStore(db)
	locker := engine.NewPessimisticLock(db.Pool)
	idempotency := engine.NewIdempotency(txnRepo)
	eng := engine.New(accountRepo, txnRepo, locker, idempotency, walStore)

	acc1 := &domain.Account{
		ID:        "int-acc-1",
		UserID:    "int-user",
		Type:      domain.AccountTypeChecking,
		Currency:  domain.CurrencyINR,
		Balance:   0,
		Version:   1,
		Status:    domain.AccountStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := accountRepo.Create(ctx, acc1); err != nil {
		t.Fatalf("create account: %v", err)
	}

	acc2 := &domain.Account{
		ID:        "int-acc-2",
		UserID:    "int-user",
		Type:      domain.AccountTypeChecking,
		Currency:  domain.CurrencyINR,
		Balance:   0,
		Version:   1,
		Status:    domain.AccountStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := accountRepo.Create(ctx, acc2); err != nil {
		t.Fatalf("create account 2: %v", err)
	}

	deposit, err := eng.Deposit(ctx, engine.DepositRequest{
		AccountID:      "int-acc-1",
		Amount:         50000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "int-dep-1",
	})
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if deposit.Status != domain.TransactionStatusCompleted {
		t.Errorf("deposit status: expected COMPLETED, got %s", deposit.Status)
	}

	bal1, _ := accountRepo.GetByID(ctx, "int-acc-1")
	if bal1.Balance != 50000 {
		t.Errorf("balance after deposit: expected 50000, got %d", bal1.Balance)
	}

	transfer, err := eng.Transfer(ctx, engine.TransferRequest{
		FromAccountID:  "int-acc-1",
		ToAccountID:    "int-acc-2",
		Amount:         20000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "int-tx-1",
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if transfer.Status != domain.TransactionStatusCompleted {
		t.Errorf("transfer status: expected COMPLETED, got %s", transfer.Status)
	}

	bal1, _ = accountRepo.GetByID(ctx, "int-acc-1")
	bal2, _ := accountRepo.GetByID(ctx, "int-acc-2")
	if bal1.Balance != 30000 {
		t.Errorf("from balance: expected 30000, got %d", bal1.Balance)
	}
	if bal2.Balance != 20000 {
		t.Errorf("to balance: expected 20000, got %d", bal2.Balance)
	}

	rev, err := eng.ReverseTransaction(ctx, transfer.ID, "int-rev-1")
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if rev.Status != domain.TransactionStatusCompleted {
		t.Errorf("reversal status: expected COMPLETED, got %s", rev.Status)
	}

	bal1, _ = accountRepo.GetByID(ctx, "int-acc-1")
	bal2, _ = accountRepo.GetByID(ctx, "int-acc-2")
	if bal1.Balance != 50000 {
		t.Errorf("from balance after reversal: expected 50000, got %d", bal1.Balance)
	}
	if bal2.Balance != 0 {
		t.Errorf("to balance after reversal: expected 0, got %d", bal2.Balance)
	}

	idempResult, err := eng.Deposit(ctx, engine.DepositRequest{
		AccountID:      "int-acc-1",
		Amount:         99999,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "int-dep-1",
	})
	if err != nil {
		t.Fatalf("idempotent deposit: %v", err)
	}
	if idempResult.ID != deposit.ID {
		t.Error("idempotent request should return original transaction")
	}
}
