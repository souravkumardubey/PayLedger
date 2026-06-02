package domain

import (
	"time"
)

type AccountType string

const (
	AccountTypeChecking AccountType = "CHECKING"
	AccountTypeSavings  AccountType = "SAVINGS"
	AccountTypeWallet   AccountType = "WALLET"
	AccountTypeMerchant AccountType = "MERCHANT"
)

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "ACTIVE"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

type Currency string

const (
	CurrencyINR Currency = "INR"
	CurrencyUSD Currency = "USD"
)

type Account struct {
	ID        string        `json:"id"`
	UserID    string        `json:"user_id"`
	Type      AccountType   `json:"type"`
	Currency  Currency      `json:"currency"`
	Balance   int64         `json:"balance"`
	Version   int64         `json:"version"`
	Status    AccountStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}
