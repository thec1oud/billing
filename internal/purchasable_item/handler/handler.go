package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
)

type PurchasableItemService interface {
	ListAll(ctx context.Context) ([]itemmodel.PurchasableItem, error)
}

type PurchasableItemHandler struct {
	svc PurchasableItemService
	log *slog.Logger
}

func NewPurchasableItemHandler(
	svc PurchasableItemService,
) *PurchasableItemHandler {
	return &PurchasableItemHandler{
		svc: svc,
		log: logger.ForComponent("purchasable_item_handler"),
	}
}

func New(svc PurchasableItemService) *PurchasableItemHandler {
	return NewPurchasableItemHandler(svc)
}

func (h *PurchasableItemHandler) HandleList(
	w http.ResponseWriter,
	r *http.Request,
) {
	items, err := h.svc.ListAll(r.Context())
	if err != nil {
		h.log.Error(
			"Failed to list purchasable items",
			logger.Err(err),
		)

		response.Write(
			w,
			http.StatusInternalServerError,
			"Failed to list purchasable items",
		)
		return
	}

	response.Write(
		w,
		http.StatusOK,
		items,
	)
}
