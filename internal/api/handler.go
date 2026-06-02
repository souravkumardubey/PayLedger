package api

import "net/http"

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
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

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request)         { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request)          { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request)         { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request)       { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) ReverseTransaction(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func (h *Handler) Health(w http.ResponseWriter, r *http.Request)           { w.WriteHeader(http.StatusNotImplemented) }
