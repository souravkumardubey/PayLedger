package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/souravkumardubey/PayLedger/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func errorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusInternalServerError
	case errors.Is(err, domain.ErrAccountNotFound), errors.Is(err, domain.ErrTransactionNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrInsufficientFunds), errors.Is(err, domain.ErrCurrencyMismatch),
		errors.Is(err, domain.ErrSelfTransfer), errors.Is(err, domain.ErrInvalidAmount):
		return http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrVersionConflict), errors.Is(err, domain.ErrDuplicateTransaction):
		return http.StatusConflict
	case errors.Is(err, domain.ErrAccountFrozen), errors.Is(err, domain.ErrAccountClosed):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
