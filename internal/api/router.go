package api

import (
	"net/http"

	accountHandlers "github.com/thec1oud/billing/internal/account/handler"
	accountservice "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/infra/api/response"
	subscriptionHandlers "github.com/thec1oud/billing/internal/subscription/handler"
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
		h := accountHandlers.NewAccountHandler(services.Accounts)
		mux.HandleFunc("POST /v1/accounts", h.HandleCreate)
		mux.HandleFunc("GET /v1/accounts/{accountID}", h.HandleGet)
		mux.HandleFunc("POST /v1/accounts/{accountID}/activate", h.HandleActivate)
		mux.HandleFunc("POST /v1/accounts/{accountID}/suspend", h.HandleSuspend)
		mux.HandleFunc("POST /v1/accounts/{accountID}/reactivate", h.HandleReactivate)
		mux.HandleFunc("POST /v1/accounts/{accountID}/close", h.HandleClose)
		mux.HandleFunc("POST /v1/accounts/{accountID}/payment-methods", h.HandleAddPaymentMethod)
	}
	if services.Subscriptions != nil {
		h := subscriptionHandlers.NewSubscriptionHandler(services.Subscriptions)
		mux.HandleFunc("POST /v1/subscriptions", h.HandleCreateSubscription)
		mux.HandleFunc("GET /v1/subscriptions/{subscriptionID}", h.HandleGetSubscription)
	}
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}
