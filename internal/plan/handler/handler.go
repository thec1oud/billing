package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
)

type PlanService interface {
	CreatePlan(ctx context.Context, p planmodel.Plan) (planmodel.Plan, error)
	ListActivePlans(ctx context.Context) ([]planmodel.Plan, error)
}

type PlanHandler struct {
	svc PlanService
}

func NewPlanHandler(svc PlanService) *PlanHandler {
	return &PlanHandler{svc: svc}
}

func (h *PlanHandler) HandleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var in planmodel.Plan
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
		})
		return
	}
	defer r.Body.Close()

	created, err := h.svc.CreatePlan(r.Context(), in)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "CREATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusCreated, created)
}

func (h *PlanHandler) HandleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListActivePlans(r.Context())
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "LIST_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusOK, plans)
}
