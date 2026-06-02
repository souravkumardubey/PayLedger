package domain

type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

type LedgerEntry struct {
	ID             string    `json:"id"`
	TransactionID  string    `json:"transaction_id"`
	AccountID      string    `json:"account_id"`
	Direction      Direction `json:"direction"`
	Amount         int64     `json:"amount"`
	Currency       Currency  `json:"currency"`
	BalanceSnapshot int64    `json:"balance_snapshot"`
}
