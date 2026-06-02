package domain

import "time"

type TransactionType string

const (
	TransactionTypeCredit    TransactionType = "CREDIT"
	TransactionTypeDebit     TransactionType = "DEBIT"
	TransactionTypeTransfer  TransactionType = "TRANSFER"
	TransactionTypeRefund    TransactionType = "REFUND"
	TransactionTypeReversal  TransactionType = "REVERSAL"
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "PENDING"
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	TransactionStatusFailed    TransactionStatus = "FAILED"
	TransactionStatusReversed  TransactionStatus = "REVERSED"
)

type Transaction struct {
	ID              string            `json:"id"`
	IdempotencyKey  string            `json:"idempotency_key"`
	Type            TransactionType   `json:"type"`
	Status          TransactionStatus `json:"status"`
	Entries         []LedgerEntry     `json:"entries,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
}
