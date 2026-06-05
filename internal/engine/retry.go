package engine

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  50 * time.Millisecond,
		MaxDelay:   500 * time.Millisecond,
	}
}

func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}

func retryBackoff(cfg RetryConfig, attempt int) time.Duration {
	d := cfg.BaseDelay * (1 << attempt)
	if d > cfg.MaxDelay {
		d = cfg.MaxDelay
	}
	return d
}
