package api

import (
	"encoding/json"
	"net/http"
)

// Response represents a standardized API response envelope.
type Response struct {
	Success bool           `json:"success"`
	Data    any            `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse holds standard machine-readable error details.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteSuccess writes a standardized HTTP JSON success response.
func WriteSuccess(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    data,
	})
}

// WriteError writes a standardized HTTP JSON error response.
func WriteError(w http.ResponseWriter, status int, errorCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		Success: false,
		Error: &ErrorResponse{
			Code:    errorCode,
			Message: message,
		},
	})
}
