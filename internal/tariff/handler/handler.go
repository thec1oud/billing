package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
)

type CreateInput struct {
	Code        string                     `json:"code"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	TariffType  tariffmodel.TariffTypeCode `json:"tariff_type_code"`
	Amount      money.Money                `json:"amount"`
	Tiers       []tariffmodel.Tier         `json:"tiers,omitempty"`
	Metadata    []byte                     `json:"metadata,omitempty"`
}

type TariffService interface {
	CreateTariff(
		ctx context.Context,
		code string,
		name string,
		description string,
		tariffType tariffmodel.TariffTypeCode,
		amount money.Money,
		tiers []tariffmodel.Tier,
		metadata []byte,
	) (tariffmodel.Tariff, error)
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
