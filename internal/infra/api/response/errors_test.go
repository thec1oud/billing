package response_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/shared/apperr"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantCode     string
		wantMessage  string
		logContains  string
		logNotLeaked string
	}{
		{
			name:        "validation error maps to 400 with safe message",
			err:         apperr.Validation("invalid_input", "email address format is invalid"),
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_input",
			wantMessage: "email address format is invalid",
		},
		{
			name:        "not found error maps to 404",
			err:         apperr.NotFound("account_not_found", "account does not exist"),
			wantStatus:  http.StatusNotFound,
			wantCode:    "account_not_found",
			wantMessage: "account does not exist",
		},
		{
			name:        "conflict error maps to 409",
			err:         apperr.Conflict("duplicate_key", "record already exists"),
			wantStatus:  http.StatusConflict,
			wantCode:    "duplicate_key",
			wantMessage: "record already exists",
		},
		{
			name:        "unauthorized error maps to 401",
			err:         apperr.Unauthorized("token_expired", "session has expired"),
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "token_expired",
			wantMessage: "session has expired",
		},
		{
			name:        "forbidden error maps to 403",
			err:         apperr.Forbidden("insufficient_permissions", "permission denied"),
			wantStatus:  http.StatusForbidden,
			wantCode:    "insufficient_permissions",
			wantMessage: "permission denied",
		},
		{
			name:        "wrapped apperr retains kind and status",
			err:         fmt.Errorf("service layer: %w", apperr.NotFound("resource_missing", "item not found")),
			wantStatus:  http.StatusNotFound,
			wantCode:    "resource_missing",
			wantMessage: "item not found",
		},
		{
			name:         "unclassified standard error maps to 500 and withholds internal detail",
			err:          errors.New("pq: syntax error at or near \"SELECT * FROM secret_table\""),
			wantStatus:   http.StatusInternalServerError,
			wantCode:     "internal_error",
			wantMessage:  "An internal error occurred",
			logContains:  "SELECT * FROM secret_table",
			logNotLeaked: "secret_table",
		},
		{
			name:         "internal app error maps to 500 and withholds internal detail",
			err:          apperr.Internal(errors.New("redis timeout: dial tcp 10.0.0.1:6379 i/o timeout"), "cache failed"),
			wantStatus:   http.StatusInternalServerError,
			wantCode:     "internal_error",
			wantMessage:  "An internal error occurred",
			logContains:  "redis timeout",
			logNotLeaked: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			rec := httptest.NewRecorder()
			response.WriteError(rec, logger, "test_context", tt.err)

			require.Equal(t, tt.wantStatus, rec.Code)

			var envelope response.Response
			err := json.Unmarshal(rec.Body.Bytes(), &envelope)
			require.NoError(t, err)
			require.False(t, envelope.Success)
			require.NotNil(t, envelope.Error)
			require.Equal(t, tt.wantCode, envelope.Error.Code)
			require.Equal(t, tt.wantMessage, envelope.Error.Message)

			if tt.logContains != "" {
				require.Contains(t, logBuf.String(), tt.logContains)
			}
			if tt.logNotLeaked != "" {
				require.NotContains(t, rec.Body.String(), tt.logNotLeaked)
			}
		})
	}
}

func TestWriteErrorNilError(t *testing.T) {
	rec := httptest.NewRecorder()
	response.WriteError(rec, nil, "test_context", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
}
