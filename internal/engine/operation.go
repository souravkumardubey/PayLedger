package engine

import (
	"context"
	"encoding/json"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

type Operation interface {
	Type() domain.TransactionType
	IdempotencyKey() string
	AccountIDs() []string
	Validate(ctx context.Context, accounts []*domain.Account) error
	Apply(ctx context.Context, accounts []*domain.Account) (*domain.Transaction, error)
	RequestData() json.RawMessage
}
