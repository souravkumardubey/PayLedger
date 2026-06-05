package engine

import (
	"context"
	"errors"
	"log/slog"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/repository"
)

func RecoverWAL(ctx context.Context, wal WALStore, txnRepo repository.TransactionRepository) error {
	entries, err := wal.GetPending(ctx)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		_, err := txnRepo.GetByIdempotencyKey(ctx, entry.IdempotencyKey)
		if err == nil {
			if err := wal.MarkCommitted(ctx, entry.ID); err != nil {
				return err
			}
			slog.Default().Info("WAL recovery entry committed",
				"wal_id", entry.ID[:8],
				"type", entry.OperationType,
			)
		} else if errors.Is(err, domain.ErrTransactionNotFound) {
			if err := wal.MarkRolledBack(ctx, entry.ID); err != nil {
				return err
			}
			slog.Default().Info("WAL recovery entry rolled back",
				"wal_id", entry.ID[:8],
				"type", entry.OperationType,
			)
		} else {
			return err
		}
	}

	if len(entries) == 0 {
		slog.Default().Info("WAL recovery: no pending entries found")
	}

	return nil
}
