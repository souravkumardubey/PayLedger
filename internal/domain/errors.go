package domain

import "errors"

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountFrozen        = errors.New("account is frozen")
	ErrAccountClosed        = errors.New("account is closed")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAmount        = errors.New("amount must be positive")
	ErrCurrencyMismatch     = errors.New("currency mismatch between accounts")
	ErrSelfTransfer         = errors.New("cannot transfer to the same account")
	ErrDuplicateTransaction = errors.New("duplicate idempotency key")
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrVersionConflict      = errors.New("optimistic lock version conflict")
)
