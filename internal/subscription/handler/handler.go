package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/subscription/model"
)

type SubscriptionService interface {
	Create(ctx context.Context, input model.CreateInput) (model.Subscription, error)
	ListAccountSubscriptions(ctx context.Context, accountID int64) ([]model.Subscription, error)
}

type SubscriptionHandler struct {
	svc SubscriptionService
}

func NewSubscriptionHandler(svc SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
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
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "CREATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusCreated, created)
}

func (h *SubscriptionHandler) HandleListAccountSubscriptions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ACCOUNT_ID",
			Message: "Account ID is required",
		})
		return
	}

	var accountID int64
	if _, err := fmt.Sscanf(idStr, "%d", &accountID); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ACCOUNT_ID",
			Message: "Account ID must be an integer",
		})
		return
	}

	subs, err := h.svc.ListAccountSubscriptions(r.Context(), accountID)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "LIST_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusOK, subs)
}
