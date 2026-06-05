package engine

import (
	"context"
	"errors"
	"log"

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
			log.Printf("WAL recovery: entry %s (%s) → COMMITTED", entry.ID[:8], entry.OperationType)
		} else if errors.Is(err, domain.ErrTransactionNotFound) {
			if err := wal.MarkRolledBack(ctx, entry.ID); err != nil {
				return err
			}
			log.Printf("WAL recovery: entry %s (%s) → ROLLED_BACK", entry.ID[:8], entry.OperationType)
		} else {
			return err
		}
	}

	if len(entries) == 0 {
		log.Println("WAL recovery: no pending entries found")
	}

	return nil
}
