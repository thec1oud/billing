package response

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	invoicerepository "github.com/thec1oud/billing/internal/invoice/repository"
	subscriptionmodel "github.com/thec1oud/billing/internal/subscription/model"
)

// WriteError inspects err, picks the right HTTP status, and writes the
// real error message back to the client instead of a hardcoded generic
// string. Handlers should call this instead of hand-rolling
// Write(w, http.StatusInternalServerError, "Failed to ...") for every
// failure path, which previously hid the actual cause (validation
// failure, not-found, FK violation, etc.) behind an opaque 500.
func WriteError(w http.ResponseWriter, log *slog.Logger, context string, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"

	switch {
	case errors.Is(err, accountmodel.ErrNotFound),
		errors.Is(err, subscriptionmodel.ErrNotFound),
		errors.Is(err, invoicerepository.ErrInvoiceNotFound):
		status = http.StatusNotFound
		code = "not_found"

	case errors.Is(err, accountmodel.ErrInvalidStateTransition),
		errors.Is(err, accountmodel.ErrClosed),
		errors.Is(err, accountmodel.ErrDuplicateExternalID),
		errors.Is(err, subscriptionmodel.ErrInvalidStateTransition):
		status = http.StatusConflict
		code = "conflict"

	case isClientError(err):
		status = http.StatusBadRequest
		code = "invalid_request"
	}

	if status >= 500 {
		log.Error(context, slog.String("error", err.Error()))
	} else {
		log.Warn(context, slog.String("error", err.Error()))
	}

	Write(w, status, &ErrorResponse{Code: code, Message: err.Error()})
}

func isClientError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "must be") ||
		strings.Contains(msg, "must not") ||
		strings.Contains(msg, "invalid ") ||
		strings.Contains(msg, "not active") ||
		strings.Contains(msg, "mismatch") ||
		strings.Contains(msg, "required") ||
		strings.Contains(msg, "cannot create draft invoice without")
}
