# PayLedger

A double-entry transaction engine built in Go. Implements idempotent payment processing with optimistic concurrency control, Write-Ahead Log crash recovery, and a fully auditable ledger trail — designed with zero floating-point money handling.

## Features

- **Double-entry accounting** — every transaction has balanced debit/credit entries
- **Idempotency key deduplication** — safe retries with exactly-once semantics
- **Optimistic concurrency** — version-based conflict detection for concurrent balance updates
- **Write-Ahead Log** — crash recovery guarantees no data loss
- **Balance safety** — int64 amounts (never floats), currency-typed accounts, audit trail on every mutation
- **REST API** — structured request/response types
- **PostgreSQL storage** — migration support

## Tech Stack

Go, PostgreSQL, Docker

## Project Structure

```
cmd/server/main.go              # Entry point
internal/
├── domain/                      # Core domain types
│   ├── account.go              # Account, AccountType, AccountStatus
│   ├── transaction.go          # Transaction, TransactionType, TransactionStatus
│   ├── ledger_entry.go         # LedgerEntry, Direction
│   └── errors.go               # Domain errors
├── repository/                  # Data access interfaces
│   ├── account_repo.go
│   └── transaction_repo.go
├── engine/                      # Business logic
│   ├── engine.go               # Engine interface + request types
│   ├── validator.go
│   ├── idempotency.go
│   └── locking.go
├── api/                         # HTTP layer
│   ├── handler.go              # Routes + handlers
│   ├── middleware.go
│   └── request.go              # Request/response types
└── config/config.go            # Configuration
migrations/                      # SQL migrations
```
