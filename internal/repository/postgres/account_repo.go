package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type AccountRepo struct {
	db *DB
}

func NewAccountRepo(db *DB) *AccountRepo {
	return &AccountRepo{db: db}
}

func (r *AccountRepo) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (id, user_id, type, currency, balance, version, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.Pool.Exec(ctx, query,
		account.ID, account.UserID, account.Type, account.Currency,
		account.Balance, account.Version, account.Status,
		account.CreatedAt, account.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAccountNotFound
		}
		return err
	}
	return nil
}

func (r *AccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	query := `
		SELECT id, user_id, type, currency, balance, version, status, created_at, updated_at
		FROM accounts WHERE id = $1`

	account := &domain.Account{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&account.ID, &account.UserID, &account.Type, &account.Currency,
		&account.Balance, &account.Version, &account.Status,
		&account.CreatedAt, &account.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}
	return account, nil
}

func (r *AccountRepo) UpdateBalance(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts
		SET balance = $1, version = version + 1, updated_at = $2
		WHERE id = $3 AND version = $4`

	now := time.Now()
	tag, err := r.db.Pool.Exec(ctx, query, account.Balance, now, account.ID, account.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}

	account.Version++
	account.UpdatedAt = now
	return nil
}
