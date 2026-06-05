package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

func TestTransferOperation_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     TransferRequest
		from    *domain.Account
		to      *domain.Account
		wantErr error
		wantAny bool
	}{
		{
			name: "success",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 1000, Currency: domain.CurrencyINR,
			},
			from: &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			to:   &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
		},
		{
			name: "invalid amount",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 0, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name: "negative amount",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: -100, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name: "self transfer",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a1",
				Amount: 1000, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			to:      &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			wantErr: domain.ErrSelfTransfer,
		},
		{
			name: "insufficient funds",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 10000, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 1000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
			wantErr: domain.ErrInsufficientFunds,
		},
		{
			name: "source frozen",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 1000, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusFrozen, Currency: domain.CurrencyINR, Balance: 5000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
			wantAny: true,
		},
		{
			name: "destination frozen",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 1000, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusFrozen, Currency: domain.CurrencyINR, Balance: 0},
			wantAny: true,
		},
		{
			name: "currency mismatch source",
			req: TransferRequest{
				FromAccountID: "a1", ToAccountID: "a2",
				Amount: 1000, Currency: domain.CurrencyINR,
			},
			from:    &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyUSD, Balance: 5000},
			to:      &domain.Account{ID: "a2", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 0},
			wantErr: domain.ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := NewTransferOperation(tt.req)
			err := op.Validate(context.Background(), []*domain.Account{tt.from, tt.to})
			if tt.wantAny {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransferOperation_Apply(t *testing.T) {
	from := &domain.Account{ID: "a1", Balance: 5000, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	to := &domain.Account{ID: "a2", Balance: 1000, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	op := NewTransferOperation(TransferRequest{
		FromAccountID:  "a1",
		ToAccountID:    "a2",
		Amount:         2000,
		Currency:       domain.CurrencyINR,
		IdempotencyKey: "tx-apply",
	})

	tx, err := op.Apply(context.Background(), []*domain.Account{from, to})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if from.Balance != 3000 {
		t.Errorf("from balance = %d, want 3000", from.Balance)
	}
	if to.Balance != 3000 {
		t.Errorf("to balance = %d, want 3000", to.Balance)
	}
	if tx.Type != domain.TransactionTypeTransfer {
		t.Errorf("type = %s, want TRANSFER", tx.Type)
	}
	if len(tx.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(tx.Entries))
	}
	if tx.Entries[0].Direction != domain.DirectionDebit {
		t.Errorf("entry[0] direction = %s, want DEBIT", tx.Entries[0].Direction)
	}
	if tx.Entries[1].Direction != domain.DirectionCredit {
		t.Errorf("entry[1] direction = %s, want CREDIT", tx.Entries[1].Direction)
	}
}

func TestDepositOperation_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     DepositRequest
		acc     *domain.Account
		wantErr error
		wantAny bool
	}{
		{
			name: "success",
			req:  DepositRequest{AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR},
			acc:  &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR},
		},
		{
			name:    "invalid amount",
			req:     DepositRequest{AccountID: "a1", Amount: 0, Currency: domain.CurrencyINR},
			acc:     &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR},
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name:    "frozen account",
			req:     DepositRequest{AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR},
			acc:     &domain.Account{ID: "a1", Status: domain.AccountStatusFrozen, Currency: domain.CurrencyINR},
			wantAny: true,
		},
		{
			name:    "currency mismatch",
			req:     DepositRequest{AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR},
			acc:     &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyUSD},
			wantErr: domain.ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := NewDepositOperation(tt.req)
			err := op.Validate(context.Background(), []*domain.Account{tt.acc})
			if tt.wantAny {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDepositOperation_Apply(t *testing.T) {
	acc := &domain.Account{ID: "a1", Balance: 5000}

	op := NewDepositOperation(DepositRequest{
		AccountID: "a1", Amount: 1500, Currency: domain.CurrencyINR, IdempotencyKey: "dep-apply",
	})

	tx, err := op.Apply(context.Background(), []*domain.Account{acc})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if acc.Balance != 6500 {
		t.Errorf("balance = %d, want 6500", acc.Balance)
	}
	if tx.Type != domain.TransactionTypeCredit {
		t.Errorf("type = %s, want CREDIT", tx.Type)
	}
	if len(tx.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(tx.Entries))
	}
	if tx.Entries[0].Direction != domain.DirectionCredit {
		t.Errorf("direction = %s, want CREDIT", tx.Entries[0].Direction)
	}
	if tx.Entries[0].BalanceSnapshot != 6500 {
		t.Errorf("balance_snapshot = %d, want 6500", tx.Entries[0].BalanceSnapshot)
	}
}

func TestWithdrawOperation_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     DepositRequest
		acc     *domain.Account
		wantErr error
	}{
		{
			name: "success",
			req:  DepositRequest{AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR},
			acc:  &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 5000},
		},
		{
			name:    "insufficient funds",
			req:     DepositRequest{AccountID: "a1", Amount: 10000, Currency: domain.CurrencyINR},
			acc:     &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyINR, Balance: 1000},
			wantErr: domain.ErrInsufficientFunds,
		},
		{
			name:    "currency mismatch",
			req:     DepositRequest{AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR},
			acc:     &domain.Account{ID: "a1", Status: domain.AccountStatusActive, Currency: domain.CurrencyUSD, Balance: 5000},
			wantErr: domain.ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := NewWithdrawOperation(tt.req)
			err := op.Validate(context.Background(), []*domain.Account{tt.acc})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithdrawOperation_Apply(t *testing.T) {
	acc := &domain.Account{ID: "a1", Balance: 5000}

	op := NewWithdrawOperation(DepositRequest{
		AccountID: "a1", Amount: 2000, Currency: domain.CurrencyINR, IdempotencyKey: "wd-apply",
	})

	tx, err := op.Apply(context.Background(), []*domain.Account{acc})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if acc.Balance != 3000 {
		t.Errorf("balance = %d, want 3000", acc.Balance)
	}
	if tx.Type != domain.TransactionTypeDebit {
		t.Errorf("type = %s, want DEBIT", tx.Type)
	}
	if len(tx.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(tx.Entries))
	}
	if tx.Entries[0].Direction != domain.DirectionDebit {
		t.Errorf("direction = %s, want DEBIT", tx.Entries[0].Direction)
	}
}

func TestOperation_RequestData(t *testing.T) {
	t.Run("transfer", func(t *testing.T) {
		op := NewTransferOperation(TransferRequest{
			FromAccountID: "a1", ToAccountID: "a2", Amount: 1000, Currency: domain.CurrencyINR, IdempotencyKey: "k1",
		})
		data := op.RequestData()
		if len(data) == 0 {
			t.Error("expected non-empty request data")
		}
	})

	t.Run("deposit", func(t *testing.T) {
		op := NewDepositOperation(DepositRequest{
			AccountID: "a1", Amount: 1000, Currency: domain.CurrencyINR, IdempotencyKey: "k1",
		})
		data := op.RequestData()
		if len(data) == 0 {
			t.Error("expected non-empty request data")
		}
	})
}

func TestTransferOperation_AccountIDs(t *testing.T) {
	op := NewTransferOperation(TransferRequest{
		FromAccountID: "from-id", ToAccountID: "to-id",
	})
	ids := op.AccountIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	if ids[0] != "from-id" {
		t.Errorf("ids[0] = %s, want from-id", ids[0])
	}
	if ids[1] != "to-id" {
		t.Errorf("ids[1] = %s, want to-id", ids[1])
	}
}

func TestDepositOperation_AccountIDs(t *testing.T) {
	op := NewDepositOperation(DepositRequest{
		AccountID: "acc-id",
	})
	ids := op.AccountIDs()
	if len(ids) != 1 {
		t.Fatalf("expected 1 ID, got %d", len(ids))
	}
	if ids[0] != "acc-id" {
		t.Errorf("ids[0] = %s, want acc-id", ids[0])
	}
}
