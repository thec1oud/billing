package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/subscription/model"
)

type SubscriptionService interface {
	Create(ctx context.Context, input model.CreateInput) (model.Subscription, error)
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
