package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
)

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PlanService interface {
	CreatePlan(ctx context.Context, tx pgx.Tx, p planmodel.Plan) (planmodel.Plan, error)
	CreatePlanDuration(ctx context.Context, tx pgx.Tx, duration planmodel.PlanDuration) (planmodel.PlanDuration, error)
	ListActivePlans(ctx context.Context) ([]planmodel.Plan, error)
}

type PurchasableItemService interface {
	Create(ctx context.Context, tx pgx.Tx, item itemmodel.PurchasableItem) (itemmodel.PurchasableItem, error)
}

type PlanHandler struct {
	pool    TxBeginner
	svc     PlanService
	itemSvc PurchasableItemService
	log     *slog.Logger
}

func NewPlanHandler(pool TxBeginner, svc PlanService, itemSvc PurchasableItemService) *PlanHandler {
	return &PlanHandler{
		pool:    pool,
		svc:     svc,
		itemSvc: itemSvc,
		log:     logger.ForComponent("plan_handler"),
	}
}

// New creates a PlanHandler with just a PlanService for simple or mock usage.
func New(svc PlanService) *PlanHandler {
	return NewPlanHandler(nil, svc, nil)
}

func (h *PlanHandler) HandleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var input planmodel.Plan
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
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

	created, err := h.svc.CreatePlan(r.Context(), tx, input)
	if err != nil {
		response.WriteError(w, h.log, "create plan failed", err)
		return
	}

	for i, duration := range input.Durations {
		duration.PlanID = created.ID
		createdDuration, durationErr := h.svc.CreatePlanDuration(r.Context(), tx, duration)
		if durationErr != nil {
			response.WriteError(w, h.log, fmt.Sprintf("create plan duration %d failed", i), durationErr)
			return
		}
		created.Durations = append(created.Durations, createdDuration)
	}

	if h.itemSvc != nil {
		planID := created.ID
		_, err = h.itemSvc.Create(r.Context(), tx, itemmodel.PurchasableItem{
			ItemCode:     fmt.Sprintf("%s_v%d", created.PlanCode, created.Version),
			ItemTypeCode: itemmodel.ItemTypePlan,
			Name:         created.PlanCode,
			PlanID:       &planID,
			IsActive:     true,
		})
		if err != nil {
			response.WriteError(w, h.log, "create plan purchasable item failed", err)
			return
		}
	}

	if tx != nil {
		if err := tx.Commit(r.Context()); err != nil {
			response.WriteError(w, h.log, "commit plan transaction failed", err)
			return
		}
	}

	response.Write(w, http.StatusCreated, created)
}

func (h *PlanHandler) HandleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListActivePlans(r.Context())
	if err != nil {
		response.WriteError(w, h.log, "list plans failed", err)
		return
	}
	response.Write(w, http.StatusOK, plans)
}
