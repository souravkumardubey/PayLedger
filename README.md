# PayLedger

A double-entry payment transaction engine built in Go. Demonstrates production-grade software engineering patterns for payment systems: **Template Method** orchestration, **Strategy**-based concurrency control, Write-Ahead Log crash recovery, and idempotent exactly-once semantics.

## Architecture

```
Client → HTTP API (net/http)
            ↓
        Rate Limiter (token bucket)
            ↓
        Logging middleware (slog)
            ↓
        Engine (Template Method)
         ├── Idempotency check (key dedup)
         ├── WAL append (PENDING)
         ├── LockingStrategy (Strategy)
         │    ├── OptimisticLock (version-based)
         │    └── PessimisticLock (SELECT FOR UPDATE)
         ├── Operation.Validate
         ├── Operation.Apply
         ├── Persist via repos (tx-aware q(ctx))
         └── WAL commit (COMMITTED)
            ↓
        PostgreSQL (pgxpool)
```

```mermaid
flowchart TD
    Client[Client] --> API[HTTP API]
    API --> RL[Rate Limiter]
    RL --> MW[Logging Middleware]
    MW --> ENG[Engine]
    ENG --> IDEMP[Idempotency Check]
    IDEMP --> WAL[WAL Append]
    WAL --> LOCK[Locking Strategy]
    LOCK --> OP[Validate + Apply Operation]
    OP --> REPO[Repositories]
    REPO --> PG[(PostgreSQL)]
    REPO --> WALC[WAL Commit]
```

On startup, `RecoverWAL()` reconciles any PENDING entries — if a matching transaction exists it's marked COMMITTED, otherwise ROLLED_BACK.

## Design Patterns

| Pattern | Where | Why |
|---|---|---|
| **Template Method** | `engine.go:execute()` | Defines the transaction skeleton (validate → apply → persist). Individual operations (Transfer, Deposit, Withdraw) plug in via the `Operation` interface. |
| **Strategy** | `locking.go:LockingStrategy` | Concurrency control is swappable at construction. Two implementations: `OptimisticLock` (version-based retry) and `PessimisticLock` (SELECT FOR UPDATE with pgx.Tx in context). |
| **Strategy** | `ratelimit.go:Limiter` | Rate limiting algorithm is pluggable. Current impl: token bucket. Add sliding window, GCRA, etc. by implementing `Allow(key string) bool`. |

## Features

- **Double-entry accounting** — balanced debit/credit entries per transaction
- **Idempotency** — exactly-once semantics via idempotency keys
- **Pluggable locking** — optimistic (version) or pessimistic (FOR UPDATE)
- **Write-Ahead Log** — crash-safe, auto-recovered on startup
- **Automatic retry** — exponential backoff on PG serialization failures (40001) and deadlocks (40P01)
- **Full reversal** — reverse deposits, withdrawals, or transfers
- **Rate limiting** — per-IP token bucket (10 req/s, burst 20)
- **Structured logging** — JSON output via slog with request-level lifecycle
- **No floats** — int64 amounts (paise/cents), typed currencies

## Quick Start

```bash
docker compose up -d
```

Once running:

```bash
# Create accounts
curl -s -X POST localhost:8080/accounts \
  -d '{"user_id":"u1","type":"CHECKING","currency":"INR"}' | jq .
# → {"id":"<acc1>", "balance":0, ...}

curl -s -X POST localhost:8080/accounts \
  -d '{"user_id":"u2","type":"CHECKING","currency":"INR"}' | jq .
# → {"id":"<acc2>", ...}

# Deposit ₹1000
curl -s -X POST localhost:8080/deposit \
  -H 'Idempotency-Key: dep-1' \
  -d '{"to":"<acc1>","amount":100000,"currency":"INR"}' | jq .

# Transfer ₹500
curl -s -X POST localhost:8080/transfer \
  -H 'Idempotency-Key: tx-1' \
  -d '{"from":"<acc1>","to":"<acc2>","amount":50000,"currency":"INR"}' | jq .

# Check balances
curl -s localhost:8080/accounts/<acc1>/balance | jq .
curl -s localhost:8080/accounts/<acc2>/balance | jq .

# List transactions
curl -s "localhost:8080/accounts/<acc1>/transactions?page=1&limit=10" | jq .

# Reverse the transfer
curl -s -X POST localhost:8080/transactions/<tx_id>/reverse \
  -H 'Idempotency-Key: rev-1' \
  -d '{"idempotency_key":"rev-1"}' | jq .
```

## Demo Walkthrough

This walkthrough creates two accounts, deposits funds, transfers money, and reverses the transfer while showing the expected balance changes.

```bash
# 1. Create two accounts and capture their IDs.
ACC1=$(curl -s -X POST localhost:8080/accounts \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u1","type":"CHECKING","currency":"INR"}' | jq -r '.id')

ACC2=$(curl -s -X POST localhost:8080/accounts \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"u2","type":"CHECKING","currency":"INR"}' | jq -r '.id')

# 2. Deposit INR 1000 into ACC1.
curl -s -X POST localhost:8080/deposit \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: dep-demo-1' \
  -d "{\"to\":\"$ACC1\",\"amount\":100000,\"currency\":\"INR\"}" | jq .

# Expected: ACC1 balance = 100000, ACC2 balance = 0
curl -s localhost:8080/accounts/$ACC1/balance | jq .
curl -s localhost:8080/accounts/$ACC2/balance | jq .

# 3. Transfer INR 500 from ACC1 to ACC2.
TX_ID=$(curl -s -X POST localhost:8080/transfer \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: tx-demo-1' \
  -d "{\"from\":\"$ACC1\",\"to\":\"$ACC2\",\"amount\":50000,\"currency\":\"INR\"}" | jq -r '.transaction_id')

# Expected: ACC1 balance = 50000, ACC2 balance = 50000
curl -s localhost:8080/accounts/$ACC1/balance | jq .
curl -s localhost:8080/accounts/$ACC2/balance | jq .

# 4. Reverse that transfer.
curl -s -X POST localhost:8080/transactions/$TX_ID/reverse \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: rev-demo-1' \
  -d '{"idempotency_key":"rev-demo-1"}' | jq .

# Expected after reversal: ACC1 balance = 100000, ACC2 balance = 0
curl -s localhost:8080/accounts/$ACC1/balance | jq .
curl -s localhost:8080/accounts/$ACC2/balance | jq .

# 5. Optional: inspect ACC1 transaction history.
curl -s "localhost:8080/accounts/$ACC1/transactions?page=1&limit=10" | jq .
```

### Run locally (without Docker)

```bash
# Start PostgreSQL (e.g. via docker compose up -d db)
go run ./cmd/server/
```

## API Reference

| Method | Path | Description | Idempotency Key |
|---|---|---|---|
| POST | `/accounts` | Create account | — |
| POST | `/deposit` | Credit an account | Required |
| POST | `/withdraw` | Debit an account | Required |
| POST | `/transfer` | Transfer between accounts | Required |
| GET | `/accounts/{id}/balance` | Get balance | — |
| GET | `/accounts/{id}/transactions` | List transactions (paginated) | — |
| POST | `/transactions/{id}/reverse` | Reverse a transaction | Required |
| GET | `/health` | Health check | — |

All amounts in smallest currency unit (paise/cents). Idempotency keys go in the `Idempotency-Key` header.

## Testing

```bash
# Unit tests
go test ./... -count=1 -short

# Integration tests (requires PostgreSQL)
TEST_DB_DSN="postgres://postgres:postgres@localhost:5432/transaction_engine?sslmode=disable" \
  go test ./... -count=1 -run Integration
```

## Performance

### Engine layer (mock DB, no I/O)

Measured with `go test ./internal/engine/ -bench=. -benchtime=5s -benchmem`:

| Benchmark | ns/op | B/op | allocs/op | ops/sec |
|---|---|---|---|---|
| `BenchmarkEngine_Transfer` (1 goroutine) | ~1,940 | 1,451 | 25 | ~516 K |
| `BenchmarkEngine_Transfer_Parallel` (12 goroutines) | ~3,100 | 1,457 | 25 | ~323 K |
| `BenchmarkEngine_Deposit` (1 goroutine) | ~1,840 | 1,058 | 22 | ~543 K |

### End-to-end HTTP (with PostgreSQL writes)

Measured with `go run ./cmd/loadtest/` against a local PostgreSQL 16 instance:

```
Concurrency:  50 goroutines, 30 s
Throughput:   583 req/s  (0 failures)

Latency:
  p50:  74 ms
  p90: 195 ms
  p95: 241 ms
  p99: 339 ms
```

The engine layer alone sustains ~500 K operations/sec with zero database I/O. End-to-end latency is dominated by PostgreSQL pessimistic locking (`SELECT FOR UPDATE`) and fsync. Run `go run ./cmd/loadtest/ -help` to reproduce.

## Tech Stack

- **Go 1.26** — `net/http` with method-based routing, `slog` structured logging, `context`-based tx propagation
- **PostgreSQL 16** — pgx/v5 connection pool, `SELECT FOR UPDATE` row locking, JSONB metadata
- **Docker / Orbstack** — containerized app + database

## Project Structure

```
cmd/server/main.go              # Entry point, wiring
cmd/loadtest/main.go            # HTTP load tester (unique idempotency keys, latency percentiles)
internal/
├── api/                         # HTTP handlers + middleware
│   ├── handler.go              # 8 endpoints
│   ├── middleware.go           # Structured request logging
│   ├── ratelimit.go            # Limiter interface + TokenBucket
│   ├── error.go                # Domain error → HTTP status mapping
│   └── request.go              # Request/response DTOs
├── domain/                      # Core types
│   ├── account.go, transaction.go, ledger_entry.go
│   └── errors.go
├── engine/                      # Business logic (Template Method + Strategy)
│   ├── engine.go               # Orchestrator, execute() Template Method
│   ├── operation.go            # Operation interface
│   ├── locking.go              # LockingStrategy interface
│   ├── optimistic_lock.go      # Version-based concurrency
│   ├── pessimistic_lock.go     # SELECT FOR UPDATE + pgx.Tx
│   ├── transfer.go, deposit.go, withdraw.go  # Operations
│   ├── idempotency.go          # Idempotency key check
│   ├── validator.go            # Shared validation
│   ├── wal.go                  # WAL types + interface
│   ├── recovery.go             # Crash recovery
│   ├── retry.go                # Retry config + backoff
│   └── txctx.go                # Context-based tx propagation
├── repository/                  # Interfaces
│   ├── account_repo.go
│   └── transaction_repo.go
│   └── postgres/               # PostgreSQL implementations
│       ├── db.go               # Pool + migrations
│       ├── tx.go               # q(ctx) tx-aware querier
│       ├── account_repo.go
│       ├── transaction_repo.go
│       └── wal_store.go
└── config/config.go            # Env-based config
.github/workflows/go.yml         # CI pipeline
```

## Related

This project was built as a high-impact portfolio piece demonstrating:
- **Low-Level Design (LLD)** — OOP design patterns in Go (Template Method, Strategy)
- **Database design** — double-entry ledger, optimistic/pessimistic locking, WAL
- **Production readiness** — graceful shutdown, structured logging, rate limiting, retry logic, crash recovery
