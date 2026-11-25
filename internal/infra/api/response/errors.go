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

	reqID := w.Header().Get("X-Request-Id")
	if reqID == "" {
		reqID = w.Header().Get("X-Request-ID")
	}

	appErr, ok := apperr.AsAppError(err)
	if !ok {
		if log != nil {
			attrs := []any{slog.String("error", err.Error())}
			if reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}
			log.Error(context, attrs...)
		}
		Write(w, http.StatusInternalServerError, &ErrorResponse{
			Code:      "internal_error",
			Message:   "An internal error occurred",
			RequestID: reqID,
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
			attrs := []any{slog.String("error", err.Error())}
			if reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}
			log.Error(context, attrs...)
		}
		Write(w, status, &ErrorResponse{
			Code:      appErr.Code(),
			Message:   "An internal error occurred",
			RequestID: reqID,
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
