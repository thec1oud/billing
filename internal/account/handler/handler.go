package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
)

type AccountService interface {
	Create(ctx context.Context, actor eventmodel.Actor, in model.CreateInput) (model.Account, error)
	Get(ctx context.Context, id int64) (model.Account, error)
	Activate(ctx context.Context, actor eventmodel.Actor, id int64) (model.Account, error)
	Reactivate(ctx context.Context, actor eventmodel.Actor, id int64) (model.Account, error)
	Suspend(ctx context.Context, actor eventmodel.Actor, id int64, reason string) (model.Account, error)
	Close(ctx context.Context, actor eventmodel.Actor, id int64, reason string) (model.Account, error)
	AddPaymentMethod(ctx context.Context, actor eventmodel.Actor, id int64, paymentMethodID string) (model.Account, error)
}

// Service is an alias for AccountService for backward compatibility with earlier drafts.
type Service = AccountService

type AccountHandler struct {
	svc AccountService
	log *slog.Logger
}

// Handler is an alias for AccountHandler for package-level flexibility.
type Handler = AccountHandler

func NewAccountHandler(svc AccountService) *AccountHandler {
	return &AccountHandler{
		svc: svc,
		log: logger.ForComponent("account_handler"),
	}
}

func New(svc AccountService) *AccountHandler {
	return NewAccountHandler(svc)
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

type paymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
}

func (h *AccountHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input model.CreateInput
	if !decode(w, r, &input) {
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	created, err := h.svc.Create(r.Context(), actor, input)
	if err != nil {
		response.WriteError(w, h.log, "create account failed", err)
		return
	}
	response.Write(w, http.StatusCreated, created)
}

func (h *AccountHandler) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	h.HandleCreate(w, r)
}

func (h *AccountHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	account, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, h.log, "get account failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func (h *AccountHandler) HandleGetAccount(w http.ResponseWriter, r *http.Request) {
	h.HandleGet(w, r)
}

func (h *AccountHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	account, err := h.svc.Activate(r.Context(), actor, id)
	if err != nil {
		response.WriteError(w, h.log, "activate account failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func (h *AccountHandler) HandleActivateAccount(w http.ResponseWriter, r *http.Request) {
	h.HandleActivate(w, r)
}

func (h *AccountHandler) HandleReactivate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	account, err := h.svc.Reactivate(r.Context(), actor, id)
	if err != nil {
		response.WriteError(w, h.log, "reactivate account failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func (h *AccountHandler) HandleSuspend(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	var body reasonRequest
	if !decode(w, r, &body) {
		return
	}
	if body.Reason == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "suspension reason is required",
		})
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	account, err := h.svc.Suspend(r.Context(), actor, id, body.Reason)
	if err != nil {
		response.WriteError(w, h.log, "suspend account failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func (h *AccountHandler) HandleClose(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	var body reasonRequest
	if !decode(w, r, &body) {
		return
	}
	if body.Reason == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "closure reason is required",
		})
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	account, err := h.svc.Close(r.Context(), actor, id, body.Reason)
	if err != nil {
		response.WriteError(w, h.log, "close account failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func (h *AccountHandler) HandleAddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	var body paymentMethodRequest
	if !decode(w, r, &body) {
		return
	}
	if body.PaymentMethodID == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "payment_method_id is required",
		})
		return
	}
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "account-handler"}
	account, err := h.svc.AddPaymentMethod(r.Context(), actor, id, body.PaymentMethodID)
	if err != nil {
		response.WriteError(w, h.log, "add payment method failed", err)
		return
	}
	response.Write(w, http.StatusOK, account)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: err.Error(),
		})
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	val := r.PathValue(key)
	if val == "" {
		val = r.PathValue("id")
	}
	if val == "" && key != "accountID" {
		val = r.PathValue("accountID")
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil || id <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: key + " must be a positive integer",
		})
		return 0, false
	}
	return id, true
}
