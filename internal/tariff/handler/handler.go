package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
)

type CreateInput struct {
	Code         string                     `json:"code"`
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	TariffType   tariffmodel.TariffTypeCode `json:"tariff_type_code"`
	TierStrategy tariffmodel.TierStrategy   `json:"tier_strategy"`
	QuantityUnit tariffmodel.QuantityUnit   `json:"quantity_unit"`
	Amount       money.Money                `json:"amount"`
	Tiers        []tariffmodel.Tier         `json:"tiers,omitempty"`
	Metadata     []byte                     `json:"metadata,omitempty"`
}

type TariffService interface {
	CreateTariff(
		ctx context.Context,
		code string,
		name string,
		description string,
		tariffType tariffmodel.TariffTypeCode,
		tierStrategy tariffmodel.TierStrategy,
		quantityUnit tariffmodel.QuantityUnit,
		amount money.Money,
		tiers []tariffmodel.Tier,
		metadata []byte,
	) (tariffmodel.Tariff, error)
}

type TariffHandler struct {
	svc TariffService
	log *slog.Logger
}

func NewTariffHandler(svc TariffService) *TariffHandler {
	return &TariffHandler{
		svc: svc,
		log: logger.ForComponent("tariff_handler"),
	}
}

func (h *TariffHandler) HandleCreateTariff(
	w http.ResponseWriter,
	r *http.Request,
) {
	var in CreateInput

	if err := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body: " + err.Error(),
		})
		return
	}

	defer r.Body.Close()

	created, err := h.svc.CreateTariff(
		r.Context(),
		in.Code,
		in.Name,
		in.Description,
		in.TariffType,
		in.TierStrategy,
		in.QuantityUnit,
		in.Amount,
		in.Tiers,
		in.Metadata,
	)
	if err != nil {
		response.WriteError(
			w,
			h.log,
			"create tariff failed",
			err,
		)
		return
	}

	response.Write(
		w,
		http.StatusCreated,
		created,
	)
}
