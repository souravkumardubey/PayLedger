package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/souravkumardubey/PayLedger/internal/domain"
	"github.com/souravkumardubey/PayLedger/internal/engine"
	"github.com/souravkumardubey/PayLedger/internal/repository"
)

type Handler struct {
	engine          *engine.Engine
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
}

func NewHandler(eng *engine.Engine, accountRepo repository.AccountRepository, transactionRepo repository.TransactionRepository) *Handler {
	return &Handler{engine: eng, accountRepo: accountRepo, transactionRepo: transactionRepo}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts", h.CreateAccount)
	mux.HandleFunc("POST /transfer", h.Transfer)
	mux.HandleFunc("POST /deposit", h.Deposit)
	mux.HandleFunc("POST /withdraw", h.Withdraw)
	mux.HandleFunc("GET /accounts/{id}/balance", h.GetBalance)
	mux.HandleFunc("GET /accounts/{id}/transactions", h.ListTransactions)
	mux.HandleFunc("POST /transactions/{id}/reverse", h.ReverseTransaction)
	mux.HandleFunc("GET /health", h.Health)
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.UserID == "" || req.Type == "" || req.Currency == "" {
		writeError(w, http.StatusBadRequest, "user_id, type, and currency are required")
		return
	}

	now := time.Now()
	acct := &domain.Account{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		Type:      domain.AccountType(req.Type),
		Currency:  domain.Currency(req.Currency),
		Balance:   0,
		Version:   1,
		Status:    domain.AccountStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.accountRepo.Create(r.Context(), acct); err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, acct)
}

func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	tx, err := h.engine.Transfer(r.Context(), engine.TransferRequest{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Currency:       domain.Currency(req.Currency),
		IdempotencyKey: idempotencyKey,
		Metadata:       req.Metadata,
	})
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		TransactionID: tx.ID,
		Status:        string(tx.Status),
	})
}

func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	tx, err := h.engine.Deposit(r.Context(), engine.DepositRequest{
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       domain.Currency(req.Currency),
		IdempotencyKey: idempotencyKey,
		Metadata:       req.Metadata,
	})
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		TransactionID: tx.ID,
		Status:        string(tx.Status),
	})
}

func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	tx, err := h.engine.Withdraw(r.Context(), engine.DepositRequest{
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       domain.Currency(req.Currency),
		IdempotencyKey: idempotencyKey,
		Metadata:       req.Metadata,
	})
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		TransactionID: tx.ID,
		Status:        string(tx.Status),
	})
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "account id is required")
		return
	}

	acct, err := h.accountRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		AccountID: acct.ID,
		Balance:   acct.Balance,
		Currency:  string(acct.Currency),
	})
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "account id is required")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	txs, total, err := h.transactionRepo.ListByAccountID(r.Context(), id, page, limit)
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions": txs,
		"total":        total,
		"page":         page,
		"limit":        limit,
	})
}

func (h *Handler) ReverseTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "transaction id is required")
		return
	}

	var req ReverseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = r.Header.Get("Idempotency-Key")
	}

	tx, err := h.engine.ReverseTransaction(r.Context(), id, idempotencyKey)
	if err != nil {
		writeError(w, errorStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		TransactionID: tx.ID,
		Status:        string(tx.Status),
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
