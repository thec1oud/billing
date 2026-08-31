package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountservice "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/shared/money"
	subscriptionmodel "github.com/thec1oud/billing/internal/subscription/model"
	subscriptionservice "github.com/thec1oud/billing/internal/subscription/service"
)

type Services struct {
	Accounts      *accountservice.Service
	Subscriptions *subscriptionservice.Service
}

func NewRouter(services Services) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	if services.Accounts != nil {
		mux.HandleFunc("POST /v1/accounts", createAccount(services.Accounts))
		mux.HandleFunc("GET /v1/accounts/{accountID}", getAccount(services.Accounts))
		mux.HandleFunc("POST /v1/accounts/{accountID}/activate", activateAccount(services.Accounts))
		mux.HandleFunc("POST /v1/accounts/{accountID}/suspend", suspendAccount(services.Accounts))
		mux.HandleFunc("POST /v1/accounts/{accountID}/reactivate", reactivateAccount(services.Accounts))
		mux.HandleFunc("POST /v1/accounts/{accountID}/close", closeAccount(services.Accounts))
		mux.HandleFunc("POST /v1/accounts/{accountID}/payment-methods", addPaymentMethod(services.Accounts))
	}
	if services.Subscriptions != nil {
		mux.HandleFunc("POST /v1/subscriptions", createSubscription(services.Subscriptions))
		mux.HandleFunc("GET /v1/subscriptions/{subscriptionID}", getSubscription(services.Subscriptions))
	}
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func createAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in accountmodel.CreateInput
		if !decodeJSON(w, r, &in) {
			return
		}
		account, err := s.Create(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, account)
	}
}
func getAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		account, err := s.Get(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}
func activateAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		account, err := s.Activate(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func reactivateAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		account, err := s.Reactivate(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}

func suspendAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		var body reasonRequest
		if !decodeJSON(w, r, &body) {
			return
		}
		account, err := s.Suspend(r.Context(), id, body.Reason)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}
func closeAccount(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		var body reasonRequest
		if !decodeJSON(w, r, &body) {
			return
		}
		account, err := s.Close(r.Context(), id, body.Reason)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}

type paymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
}

func addPaymentMethod(s *accountservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "accountID")
		if !ok {
			return
		}
		var body paymentMethodRequest
		if !decodeJSON(w, r, &body) {
			return
		}
		account, err := s.AddPaymentMethod(r.Context(), id, body.PaymentMethodID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, account)
	}
}
func createSubscription(s *subscriptionservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in subscriptionmodel.CreateInput
		if !decodeJSON(w, r, &in) {
			return
		}
		subscription, err := s.Create(r.Context(), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, subscription)
	}
}
func getSubscription(s *subscriptionservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "subscriptionID")
		if !ok {
			return
		}
		subscription, err := s.Get(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, subscription)
	}
}
func pathID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": key + " must be a positive integer"})
		return 0, false
	}
	return id, true
}
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request: " + err.Error()})
		return false
	}
	return true
}
func isClientError(err error) bool {
	return errors.Is(err, money.ErrInvalidCurrency) ||
		strings.Contains(err.Error(), "must") ||
		strings.Contains(err.Error(), "invalid ") ||
		strings.Contains(err.Error(), "not active") ||
		strings.Contains(err.Error(), "mismatch")
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, accountmodel.ErrNotFound), errors.Is(err, subscriptionmodel.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, accountmodel.ErrInvalidStateTransition), errors.Is(err, accountmodel.ErrClosed):
		status = http.StatusConflict
	case isClientError(err):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
