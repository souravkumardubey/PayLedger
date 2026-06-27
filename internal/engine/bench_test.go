package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

// silenceLogs replaces the default slog logger with a discard handler for the
// duration of a benchmark so engine lifecycle logs don't pollute the output.
func silenceLogs() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// newBenchEngine returns an Engine wired with no-op mocks.
// The locker returns fresh struct copies per call so parallel benchmarks
// never race on shared account state.
func newBenchEngine(from, to *domain.Account) *Engine {
	accountRepo := &mockAccountRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			if id == from.ID {
				cp := *from
				return &cp, nil
			}
			cp := *to
			return &cp, nil
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			f := *from
			t := *to
			return ctx, []*domain.Account{&f, &t}, nil
		},
	}
	return New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), &mockWAL{})
}

// BenchmarkEngine_Transfer measures single-goroutine transfer throughput.
func BenchmarkEngine_Transfer(b *testing.B) {
	silenceLogs()
	from := newTestAccount("bench-from", "u1", 1_000_000_000, domain.CurrencyINR)
	to := newTestAccount("bench-to", "u2", 0, domain.CurrencyINR)
	eng := newBenchEngine(from, to)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := eng.Transfer(context.Background(), TransferRequest{
			FromAccountID:  "bench-from",
			ToAccountID:    "bench-to",
			Amount:         100,
			Currency:       domain.CurrencyINR,
			IdempotencyKey: fmt.Sprintf("bench-%d", i),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngine_Transfer_Parallel measures transfer throughput under
// GOMAXPROCS-level concurrency. Each iteration uses a unique idempotency
// key so no request is ever de-duped by the idempotency check.
func BenchmarkEngine_Transfer_Parallel(b *testing.B) {
	silenceLogs()
	from := newTestAccount("bench-from", "u1", 1_000_000_000, domain.CurrencyINR)
	to := newTestAccount("bench-to", "u2", 0, domain.CurrencyINR)
	eng := newBenchEngine(from, to)

	var counter int64
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := atomic.AddInt64(&counter, 1)
			if _, err := eng.Transfer(context.Background(), TransferRequest{
				FromAccountID:  "bench-from",
				ToAccountID:    "bench-to",
				Amount:         100,
				Currency:       domain.CurrencyINR,
				IdempotencyKey: fmt.Sprintf("bench-p-%d", n),
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkEngine_Deposit measures deposit throughput.
func BenchmarkEngine_Deposit(b *testing.B) {
	silenceLogs()
	acc := newTestAccount("bench-acc", "u1", 0, domain.CurrencyINR)
	accountRepo := &mockAccountRepo{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Account, error) {
			cp := *acc
			return &cp, nil
		},
	}
	txnRepo := &mockTxnRepo{
		getByIdempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Transaction, error) {
			return nil, domain.ErrTransactionNotFound
		},
	}
	locker := &mockLocker{
		readAccountsFunc: func(ctx context.Context, ids []string) (context.Context, []*domain.Account, error) {
			cp := *acc
			return ctx, []*domain.Account{&cp}, nil
		},
	}
	eng := New(accountRepo, txnRepo, locker, NewIdempotency(txnRepo), &mockWAL{})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := eng.Deposit(context.Background(), DepositRequest{
			AccountID:      "bench-acc",
			Amount:         1000,
			Currency:       domain.CurrencyINR,
			IdempotencyKey: fmt.Sprintf("dep-%d", i),
		}); err != nil {
			b.Fatal(err)
		}
	}
}
