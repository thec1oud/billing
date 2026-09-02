package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/tariff"
)

type CreateInput struct {
	Code         string                 `json:"code"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	TariffType   tariff.TariffTypeCode `json:"tariff_type_code"`
	Amount       money.Money           `json:"amount"`
	Tiers        []tariff.Tier         `json:"tiers,omitempty"`
	Metadata     []byte                `json:"metadata,omitempty"`
}

type TariffService interface {
	CreateTariff(
		ctx context.Context,
		code string,
		name string,
		description string,
		tariffType tariff.TariffTypeCode,
		amount money.Money,
		tiers []tariff.Tier,
		metadata []byte,
	) (tariff.Tariff, error)
}

type TariffHandler struct {
	svc TariffService
}

func NewTariffHandler(svc TariffService) *TariffHandler {
	return &TariffHandler{svc: svc}
}

func (h *TariffHandler) HandleCreateTariff(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
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
		in.Amount,
		in.Tiers,
		in.Metadata,
	)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "CREATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusCreated, created)
}
