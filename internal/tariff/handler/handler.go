package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
)

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

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
		tx pgx.Tx,
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
	pool TxBeginner
	svc  TariffService
	log  *slog.Logger
}

func NewTariffHandler(pool TxBeginner, svc TariffService) *TariffHandler {
	return &TariffHandler{
		pool: pool,
		svc:  svc,
		log:  logger.ForComponent("tariff_handler"),
	}
}

func (h *TariffHandler) HandleCreateTariff(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body: " + err.Error(),
		})
		return
	}
	defer r.Body.Close()

	var (
		tx  pgx.Tx
		err error
	)

	if h.pool != nil {
		tx, err = h.pool.Begin(r.Context())
		if err != nil {
			response.WriteError(w, h.log, "begin transaction failed", err)
			return
		}
		defer tx.Rollback(r.Context())
	}

	created, err := h.svc.CreateTariff(
		r.Context(),
		tx,
		in.Code,
		in.Name,
		in.Description,
		in.TariffType,
		in.Amount,
		in.Tiers,
		in.Metadata,
	)
	if err != nil {
		response.WriteError(w, h.log, "create tariff failed", err)
		return
	}

	if tx != nil {
		if err := tx.Commit(r.Context()); err != nil {
			response.WriteError(w, h.log, "commit tariff transaction failed", err)
			return
		}
	}

	response.Write(w, http.StatusCreated, created)
}
