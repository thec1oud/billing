package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
)

var log = logger.ForComponent("account_handler")

type AccountHandler struct {
	svc *service.Service
}

func NewAccountHandler(svc *service.Service) *AccountHandler {
	return &AccountHandler{
		svc: svc,
	}
}

func (h *AccountHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input model.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Warn("Failed to decode account creation payload", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	account, err := h.svc.Create(ctx, input)
	if err != nil {
		response.WriteError(w, log, "Failed to create account", err)
		return
	}

	response.Write(w, http.StatusCreated, account)
}

func (h *AccountHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accountIDStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil {
		log.Warn("Invalid account ID format", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.svc.Activate(ctx, accountID)
	if err != nil {
		response.WriteError(w, log, "Failed to activate account", err)
		return
	}

	response.Write(w, http.StatusOK, account)
}
