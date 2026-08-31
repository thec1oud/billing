package handler

import (
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/purchasable_item"
)

var log = logger.ForComponent("purchasable_item_handler")

type PurchasableItemHandler struct {
	svc *purchasable_item.Service
}

func NewPurchasableItemHandler(svc *purchasable_item.Service) *PurchasableItemHandler {
	return &PurchasableItemHandler{
		svc: svc,
	}
}

func (h *PurchasableItemHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	items, err := h.svc.ListAll(ctx)
	if err != nil {
		log.Error("Failed to list purchasable items", logger.Err(err))
		response.Write(w, http.StatusInternalServerError, "Failed to list purchasable items")
		return
	}

	response.Write(w, http.StatusOK, items)
}
