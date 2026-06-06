package engine

import (
	"context"
	"encoding/json"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

// Operation defines a unit of work in the transaction engine.
// Each operation (Transfer, Deposit, Withdraw) implements this interface.
// The Engine.execute() method orchestrates the lifecycle via the Template Method pattern,
// calling Validate → Apply → persist in a fixed order, while individual operations
// supply their own validation and application logic.
type Operation interface {
	Type() domain.TransactionType
	IdempotencyKey() string
	AccountIDs() []string
	Validate(ctx context.Context, accounts []*domain.Account) error
	Apply(ctx context.Context, accounts []*domain.Account) (*domain.Transaction, error)
	RequestData() json.RawMessage
}
