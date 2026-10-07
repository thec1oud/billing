package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
)

type PlanService interface {
	CreatePlan(
		ctx context.Context,
		plan planmodel.Plan,
	) (planmodel.Plan, error)

	CreatePlanDuration(
		ctx context.Context,
		duration planmodel.PlanDuration,
	) (planmodel.PlanDuration, error)

	ListActivePlans(
		ctx context.Context,
	) ([]planmodel.Plan, error)
}

type PlanHandler struct {
	svc PlanService
	log *slog.Logger
}

func NewPlanHandler(svc PlanService) *PlanHandler {
	return &PlanHandler{
		svc: svc,
		log: logger.ForComponent("plan_handler"),
	}
}

// New creates a PlanHandler with just a PlanService for simple or mock usage.
func New(svc PlanService) *PlanHandler {
	return NewPlanHandler(svc)
}

func (h *PlanHandler) HandleCreatePlan(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input planmodel.Plan

	if err := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	).Decode(&input); err != nil {
		response.Write(
			w,
			http.StatusBadRequest,
			&response.ErrorResponse{
				Code: "INVALID_REQUEST_BODY",
				Message: "Failed to decode request body: " + err.Error(),
			},
		)
		return
	}

	defer r.Body.Close()

	created, err := h.svc.CreatePlan(
		r.Context(),
		input,
	)
	if err != nil {
		response.WriteError(
			w,
			h.log,
			"create plan failed",
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

func (h *PlanHandler) HandleListPlans(
	w http.ResponseWriter,
	r *http.Request,
) {
	plans, err := h.svc.ListActivePlans(
		r.Context(),
	)
	if err != nil {
		response.WriteError(
			w,
			h.log,
			"list plans failed",
			err,
		)
		return
	}

	response.Write(
		w,
		http.StatusOK,
		plans,
	)
}