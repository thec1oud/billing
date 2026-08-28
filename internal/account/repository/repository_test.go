package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/thec1oud/billing/internal/account/model"
)

func TestUnconfiguredRepositoryReturnsErrorsInsteadOfPanicking(t *testing.T) {
	t.Parallel()

	repo := New(nil)
	ctx := context.Background()

	if err := repo.UpdateStatus(ctx, 1, model.StatusActive, model.StatusSuspended); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("UpdateStatus error = %v, want database configuration error", err)
	}
	if err := repo.AddPaymentMethod(ctx, 1, "pm_chapa_active"); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("AddPaymentMethod error = %v, want database configuration error", err)
	}
}
