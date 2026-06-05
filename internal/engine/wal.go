package engine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WALStatus string

const (
	WALPending    WALStatus = "PENDING"
	WALCommitted  WALStatus = "COMMITTED"
	WALRolledBack WALStatus = "ROLLED_BACK"
)

type WALEntry struct {
	ID             string          `json:"id"`
	IdempotencyKey string          `json:"idempotency_key"`
	OperationType  string          `json:"operation_type"`
	RequestData    json.RawMessage `json:"request_data"`
	Status         WALStatus       `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	CommittedAt    *time.Time      `json:"committed_at,omitempty"`
}

type WALStore interface {
	Append(ctx context.Context, entry *WALEntry) error
	MarkCommitted(ctx context.Context, id string) error
	MarkRolledBack(ctx context.Context, id string) error
	GetPending(ctx context.Context) ([]WALEntry, error)
}

func NewWALEntry(op Operation) *WALEntry {
	return &WALEntry{
		ID:             uuid.New().String(),
		IdempotencyKey: op.IdempotencyKey(),
		OperationType:  string(op.Type()),
		RequestData:    op.RequestData(),
		Status:         WALPending,
		CreatedAt:      time.Now(),
	}
}
