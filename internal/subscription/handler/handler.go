package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/subscription/model"
)

type SubscriptionService interface {
	Create(ctx context.Context, input model.CreateInput) (model.Subscription, error)
	Get(ctx context.Context, subscriptionID int64) (model.Subscription, error)
	ListAccountSubscriptions(ctx context.Context, accountID int64) ([]model.Subscription, error)
}

type SubscriptionHandler struct {
	svc SubscriptionService
	log *slog.Logger
}

func NewSubscriptionHandler(svc SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		svc: svc,
		log: logger.ForComponent("subscription_handler"),
	}
}

func (h *SubscriptionHandler) HandleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var in model.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
		})
		return
	}
	defer r.Body.Close()

	created, err := h.svc.Create(r.Context(), in)
	if err != nil {
		response.WriteError(w, h.log, "create subscription failed", err)
		return
	}

	response.Write(w, http.StatusCreated, created)
}

func (h *SubscriptionHandler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("subscriptionID")
	if idStr == "" {
		idStr = r.PathValue("id")
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_SUBSCRIPTION_ID",
			Message: "Subscription ID must be a positive integer",
		})
		return
	}

	sub, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, h.log, "get subscription failed", err)
		return
	}

	response.Write(w, http.StatusOK, sub)
}

func (h *SubscriptionHandler) HandleListAccountSubscriptions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.PathValue("accountID")
	}

	if idStr == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ACCOUNT_ID",
			Message: "Account ID is required",
		})
		return
	}

	var accountID int64
	if _, err := fmt.Sscanf(idStr, "%d", &accountID); err != nil || accountID <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ACCOUNT_ID",
			Message: "Account ID must be a positive integer",
		})
		return
	}

	subs, err := h.svc.ListAccountSubscriptions(r.Context(), accountID)
	if err != nil {
		response.WriteError(w, h.log, "list subscriptions failed", err)
		return
	}

	response.Write(w, http.StatusOK, subs)
}
