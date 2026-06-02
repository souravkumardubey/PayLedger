package api

type CreateAccountRequest struct {
	UserID   string `json:"user_id"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

type TransferRequest struct {
	FromAccountID string            `json:"from"`
	ToAccountID   string            `json:"to"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type DepositRequest struct {
	AccountID string            `json:"to"`
	Amount    int64             `json:"amount"`
	Currency  string            `json:"currency"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ReverseRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type BalanceResponse struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}

type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}
