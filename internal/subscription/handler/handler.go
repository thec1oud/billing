package handler

import (
	"encoding/json"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/subscription/model"
	"github.com/thec1oud/billing/internal/subscription/service"
)

var log = logger.ForComponent("subscription_handler")

type SubscriptionHandler struct {
	svc *service.Service
}

func NewSubscriptionHandler(svc *service.Service) *SubscriptionHandler {
	return &SubscriptionHandler{
		svc: svc,
	}
}

func (h *SubscriptionHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input model.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Warn("Failed to decode subscription creation payload", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	subscription, err := h.svc.Create(ctx, input)
	if err != nil {
		response.WriteError(w, log, "Failed to create subscription", err)
		return
	}

	response.Write(w, http.StatusCreated, subscription)
}
