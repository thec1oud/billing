package response

import (
	"log/slog"
	"net/http"

	"github.com/thec1oud/billing/internal/shared/apperr"
)

// WriteError inspects err, maps its classification kind to an HTTP status,
// and writes a structured response to the client.
//
// For 4xx client errors, the client-safe message and error code are returned.
// For 5xx internal errors or unclassified errors, internal details (such as database
// queries and driver errors) are logged but withheld from the client to prevent information leaks.
func WriteError(w http.ResponseWriter, log *slog.Logger, context string, err error) {
	if err == nil {
		return
	}

	appErr, ok := apperr.AsAppError(err)
	if !ok {
		if log != nil {
			log.Error(context, slog.String("error", err.Error()))
		}
		Write(w, http.StatusInternalServerError, &ErrorResponse{
			Code:    "internal_error",
			Message: "An internal error occurred",
		})
		return
	}

	status := http.StatusInternalServerError
	switch appErr.Kind() {
	case apperr.KindNotFound:
		status = http.StatusNotFound
	case apperr.KindConflict:
		status = http.StatusConflict
	case apperr.KindValidation:
		status = http.StatusBadRequest
	case apperr.KindUnauthorized:
		status = http.StatusUnauthorized
	case apperr.KindForbidden:
		status = http.StatusForbidden
	case apperr.KindInternal:
		status = http.StatusInternalServerError
	}

	if status >= 500 {
		if log != nil {
			log.Error(context, slog.String("error", err.Error()))
		}
		Write(w, status, &ErrorResponse{
			Code:    appErr.Code(),
			Message: "An internal error occurred",
		})
	} else {
		if log != nil {
			log.Warn(context, slog.String("error", err.Error()))
		}
		Write(w, status, &ErrorResponse{
			Code:    appErr.Code(),
			Message: appErr.SafeMessage(),
		})
	}
}
