package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/shared/apperr"
)

func TestAppErrorKindsAndDefaults(t *testing.T) {
	tests := []struct {
		name         string
		err          *apperr.Error
		wantKind     apperr.Kind
		wantCode     string
		wantSafeMsg  string
		wantErrorStr string
	}{
		{
			name:         "validation with custom code",
			err:          apperr.Validation("invalid_email", "email is invalid"),
			wantKind:     apperr.KindValidation,
			wantCode:     "invalid_email",
			wantSafeMsg:  "email is invalid",
			wantErrorStr: "email is invalid",
		},
		{
			name:         "validation with default code",
			err:          apperr.Validation("", "field is required"),
			wantKind:     apperr.KindValidation,
			wantCode:     "invalid_request",
			wantSafeMsg:  "field is required",
			wantErrorStr: "field is required",
		},
		{
			name:         "not found with default code",
			err:          apperr.NotFound("", "resource not found"),
			wantKind:     apperr.KindNotFound,
			wantCode:     "not_found",
			wantSafeMsg:  "resource not found",
			wantErrorStr: "resource not found",
		},
		{
			name:         "conflict with default code",
			err:          apperr.Conflict("", "resource conflict"),
			wantKind:     apperr.KindConflict,
			wantCode:     "conflict",
			wantSafeMsg:  "resource conflict",
			wantErrorStr: "resource conflict",
		},
		{
			name:         "unauthorized with default code",
			err:          apperr.Unauthorized("", "token expired"),
			wantKind:     apperr.KindUnauthorized,
			wantCode:     "unauthorized",
			wantSafeMsg:  "token expired",
			wantErrorStr: "token expired",
		},
		{
			name:         "forbidden with default code",
			err:          apperr.Forbidden("", "access denied"),
			wantKind:     apperr.KindForbidden,
			wantCode:     "forbidden",
			wantSafeMsg:  "access denied",
			wantErrorStr: "access denied",
		},
		{
			name:         "internal wrapping underlying error",
			err:          apperr.Internal(errors.New("db down"), "database query failed"),
			wantKind:     apperr.KindInternal,
			wantCode:     "internal_error",
			wantSafeMsg:  "database query failed",
			wantErrorStr: "database query failed: db down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantKind, tt.err.Kind())
			require.Equal(t, tt.wantCode, tt.err.Code())
			require.Equal(t, tt.wantSafeMsg, tt.err.SafeMessage())
			require.Equal(t, tt.wantErrorStr, tt.err.Error())
		})
	}
}

func TestAppErrorUnwrapAndIs(t *testing.T) {
	sentinel := apperr.NotFound("account_not_found", "account not found")
	wrapped := fmt.Errorf("service failed: %w", sentinel)

	require.True(t, errors.Is(wrapped, sentinel))

	sameKindSameCode := apperr.NotFound("account_not_found", "different message")
	require.True(t, errors.Is(wrapped, sameKindSameCode))

	differentCode := apperr.NotFound("invoice_not_found", "invoice not found")
	require.False(t, errors.Is(wrapped, differentCode))

	underlying := errors.New("sql: no rows in result set")
	wrappedUnderlying := apperr.Wrap(underlying, apperr.KindNotFound, "not_found", "not found")
	require.True(t, errors.Is(wrappedUnderlying, underlying))
	require.Equal(t, underlying, errors.Unwrap(wrappedUnderlying))
}

func TestAsAppError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		ae, ok := apperr.AsAppError(nil)
		require.False(t, ok)
		require.Nil(t, ae)
	})

	t.Run("direct apperr.Error", func(t *testing.T) {
		err := apperr.Validation("invalid_id", "id must be positive")
		ae, ok := apperr.AsAppError(err)
		require.True(t, ok)
		require.Equal(t, apperr.KindValidation, ae.Kind())
		require.Equal(t, "invalid_id", ae.Code())
	})

	t.Run("wrapped apperr.Error", func(t *testing.T) {
		err := fmt.Errorf("handler context: %w", apperr.Conflict("already_exists", "already exists"))
		ae, ok := apperr.AsAppError(err)
		require.True(t, ok)
		require.Equal(t, apperr.KindConflict, ae.Kind())
		require.Equal(t, "already_exists", ae.Code())
	})

	t.Run("standard non-app error", func(t *testing.T) {
		err := errors.New("plain stdlib error")
		ae, ok := apperr.AsAppError(err)
		require.False(t, ok)
		require.Nil(t, ae)
	})
}
