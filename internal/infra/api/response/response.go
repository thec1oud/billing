package response

import (
	"encoding/json"
	"net/http"
)

// Response represents a standardized API response envelope.
type Response struct {
	Success bool           `json:"success"`
	Data    any            `json:"data,omitempty"`
	Meta    any            `json:"meta,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// PaginatedPayload can be used as the payload for Write to include pagination metadata (like page, limit, total).
type PaginatedPayload struct {
	Data any
	Meta any
}

// ErrorResponse holds standard machine-readable error details.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write is a unified and robust response writer that automatically determines
// success or failure based on the HTTP status code (>= 400 is considered an error).
func Write(w http.ResponseWriter, status int, payload any, headers ...map[string]string) {
	for _, h := range headers {
		for k, v := range h {
			w.Header().Set(k, v)
		}
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(status)

	var resp Response
	if status >= 400 {
		resp.Success = false
		switch v := payload.(type) {
		case *ErrorResponse:
			resp.Error = v
		case ErrorResponse:
			resp.Error = &v
		case error:
			resp.Error = &ErrorResponse{
				Code:    "internal_error",
				Message: v.Error(),
			}
		case string:
			resp.Error = &ErrorResponse{
				Code:    "error",
				Message: v,
			}
		default:
			resp.Error = &ErrorResponse{
				Code:    "unknown_error",
				Message: "An unexpected error occurred",
			}
		}
	} else {
		resp.Success = true
		if p, ok := payload.(PaginatedPayload); ok {
			resp.Data = p.Data
			resp.Meta = p.Meta
		} else if p, ok := payload.(*PaginatedPayload); ok {
			resp.Data = p.Data
			resp.Meta = p.Meta
		} else {
			resp.Data = payload
		}
	}

	_ = json.NewEncoder(w).Encode(resp)
}

