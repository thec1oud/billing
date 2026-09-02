package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/infra/api/response"
)

type AccountService interface {
	Create(ctx context.Context, in model.CreateInput) (model.Account, error)
	Activate(ctx context.Context, accountID int64) (model.Account, error)
}

type AccountHandler struct {
	svc AccountService
}

func NewAccountHandler(svc AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var in model.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
		})
		return
	}
	defer r.Body.Close()

	acc, err := h.svc.Create(r.Context(), in)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "CREATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusCreated, acc)
}

func (h *AccountHandler) HandleActivateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Account ID must be a valid integer",
		})
		return
	}

	acc, err := h.svc.Activate(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			response.Write(w, http.StatusNotFound, &response.ErrorResponse{
				Code:    "NOT_FOUND",
				Message: err.Error(),
			})
			return
		}
		if errors.Is(err, model.ErrInvalidStateTransition) {
			response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
				Code:    "INVALID_STATE",
				Message: err.Error(),
			})
			return
		}

		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "ACTIVATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusOK, acc)
}
