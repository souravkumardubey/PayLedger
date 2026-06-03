package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type TransactionRepo struct {
	db *DB
}

func NewTransactionRepo(db *DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	txQuery := `
		INSERT INTO transactions (id, idempotency_key, type, status, metadata, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.Pool.Exec(ctx, txQuery,
		tx.ID, tx.IdempotencyKey, tx.Type, tx.Status,
		tx.Metadata, tx.CreatedAt, tx.CompletedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateTransaction
		}
		return err
	}

	if err := r.insertEntries(ctx, tx.ID, tx.Entries); err != nil {
		return err
	}
	return nil
}

func (r *TransactionRepo) insertEntries(ctx context.Context, txID string, entries []domain.LedgerEntry) error {
	query := `
		INSERT INTO ledger_entries (id, transaction_id, account_id, direction, amount, currency, balance_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for _, e := range entries {
		_, err := r.db.Pool.Exec(ctx, query,
			e.ID, txID, e.AccountID, e.Direction, e.Amount, e.Currency, e.BalanceSnapshot,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, idempotency_key, type, status, metadata, created_at, completed_at
		FROM transactions WHERE id = $1`

	tx := &domain.Transaction{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&tx.ID, &tx.IdempotencyKey, &tx.Type, &tx.Status,
		&tx.Metadata, &tx.CreatedAt, &tx.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, err
	}

	entries, err := r.getEntriesByTxID(ctx, id)
	if err != nil {
		return nil, err
	}
	tx.Entries = entries

	if tx.Metadata == nil {
		tx.Metadata = make(map[string]string)
	}

	return tx, nil
}

func (r *TransactionRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	query := `
		SELECT id, idempotency_key, type, status, metadata, created_at, completed_at
		FROM transactions WHERE idempotency_key = $1`

	tx := &domain.Transaction{}
	err := r.db.Pool.QueryRow(ctx, query, key).Scan(
		&tx.ID, &tx.IdempotencyKey, &tx.Type, &tx.Status,
		&tx.Metadata, &tx.CreatedAt, &tx.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, err
	}

	entries, err := r.getEntriesByTxID(ctx, tx.ID)
	if err != nil {
		return nil, err
	}
	tx.Entries = entries

	if tx.Metadata == nil {
		tx.Metadata = make(map[string]string)
	}

	return tx, nil
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1, completed_at = $2 WHERE id = $3`

	now := time.Now()
	tag, err := r.db.Pool.Exec(ctx, query, status, now, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTransactionNotFound
	}
	return nil
}

func (r *TransactionRepo) ListByAccountID(ctx context.Context, accountID string, page, limit int) ([]domain.Transaction, int, error) {
	offset := (page - 1) * limit

	var total int
	countQuery := `
		SELECT COUNT(DISTINCT t.id)
		FROM transactions t
		JOIN ledger_entries le ON le.transaction_id = t.id
		WHERE le.account_id = $1`

	err := r.db.Pool.QueryRow(ctx, countQuery, accountID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	txQuery := `
		SELECT DISTINCT t.id, t.idempotency_key, t.type, t.status, t.metadata, t.created_at, t.completed_at
		FROM transactions t
		JOIN ledger_entries le ON le.transaction_id = t.id
		WHERE le.account_id = $1
		ORDER BY t.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, txQuery, accountID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var txs []domain.Transaction
	var txIDs []string
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(
			&tx.ID, &tx.IdempotencyKey, &tx.Type, &tx.Status,
			&tx.Metadata, &tx.CreatedAt, &tx.CompletedAt,
		); err != nil {
			return nil, 0, err
		}
		if tx.Metadata == nil {
			tx.Metadata = make(map[string]string)
		}
		txs = append(txs, tx)
		txIDs = append(txIDs, tx.ID)
	}

	if len(txIDs) == 0 {
		return []domain.Transaction{}, total, nil
	}

	entries, err := r.getEntriesByTxIDs(ctx, txIDs)
	if err != nil {
		return nil, 0, err
	}

	entryMap := make(map[string][]domain.LedgerEntry, len(txIDs))
	for _, e := range entries {
		entryMap[e.TransactionID] = append(entryMap[e.TransactionID], e)
	}

	for i := range txs {
		txs[i].Entries = entryMap[txs[i].ID]
	}

	return txs, total, nil
}

func (r *TransactionRepo) getEntriesByTxID(ctx context.Context, txID string) ([]domain.LedgerEntry, error) {
	query := `
		SELECT id, transaction_id, account_id, direction, amount, currency, balance_snapshot
		FROM ledger_entries
		WHERE transaction_id = $1
		ORDER BY id`

	rows, err := r.db.Pool.Query(ctx, query, txID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.TransactionID, &e.AccountID, &e.Direction,
			&e.Amount, &e.Currency, &e.BalanceSnapshot,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *TransactionRepo) getEntriesByTxIDs(ctx context.Context, txIDs []string) ([]domain.LedgerEntry, error) {
	query := `
		SELECT id, transaction_id, account_id, direction, amount, currency, balance_snapshot
		FROM ledger_entries
		WHERE transaction_id = ANY($1)
		ORDER BY id`

	rows, err := r.db.Pool.Query(ctx, query, txIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.TransactionID, &e.AccountID, &e.Direction,
			&e.Amount, &e.Currency, &e.BalanceSnapshot,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
