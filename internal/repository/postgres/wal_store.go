package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/engine"
)

type WALStore struct {
	db *DB
}

func NewWALStore(db *DB) *WALStore {
	return &WALStore{db: db}
}

func (s *WALStore) Append(ctx context.Context, entry *engine.WALEntry) error {
	query := `
		INSERT INTO wal_entries (id, idempotency_key, operation_type, request_data, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.db.Pool.Exec(ctx, query,
		entry.ID, entry.IdempotencyKey, entry.OperationType,
		entry.RequestData, entry.Status, entry.CreatedAt,
	)
	return err
}

func (s *WALStore) MarkCommitted(ctx context.Context, id string) error {
	query := `UPDATE wal_entries SET status = $1, committed_at = $2 WHERE id = $3`
	_, err := s.db.Pool.Exec(ctx, query, engine.WALCommitted, time.Now(), id)
	return err
}

func (s *WALStore) MarkRolledBack(ctx context.Context, id string) error {
	query := `UPDATE wal_entries SET status = $1 WHERE id = $2`
	_, err := s.db.Pool.Exec(ctx, query, engine.WALRolledBack, id)
	return err
}

func (s *WALStore) GetPending(ctx context.Context) ([]engine.WALEntry, error) {
	query := `
		SELECT id, idempotency_key, operation_type, request_data, status, created_at, committed_at
		FROM wal_entries
		WHERE status = $1
		ORDER BY created_at`

	rows, err := s.db.Pool.Query(ctx, query, engine.WALPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []engine.WALEntry
	for rows.Next() {
		var e engine.WALEntry
		var requestData []byte
		if err := rows.Scan(
			&e.ID, &e.IdempotencyKey, &e.OperationType,
			&requestData, &e.Status, &e.CreatedAt, &e.CommittedAt,
		); err != nil {
			return nil, err
		}
		e.RequestData = json.RawMessage(requestData)
		entries = append(entries, e)
	}

	if entries == nil {
		return []engine.WALEntry{}, nil
	}
	return entries, nil
}

var _ engine.WALStore = (*WALStore)(nil)
