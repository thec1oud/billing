package handler

import (
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/plan"
)

var log = logger.ForComponent("plan_handler")

type PlanHandler struct {
	svc *plan.Service
}

func NewPlanHandler(svc *plan.Service) *PlanHandler {
	return &PlanHandler{
		svc: svc,
	}
}

func (h *PlanHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	plans, err := h.svc.ListAll(ctx)
	if err != nil {
		response.WriteError(w, log, "Failed to list plans", err)
		return
	}

	response.Write(w, http.StatusOK, plans)
}
