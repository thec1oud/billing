package account

import (
	"context"
	"errors"
	"testing"

	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
)

func newTestService() (*Service, *Repository) {
	eventStore := events.NewMemoryEventStore()

	repository := NewRepository(eventStore)

	service := NewService(
		repository,
		idempotency.NewStore(),
	)

	return service, repository
}

func TestCreateAccountStartsPendingVerification(t *testing.T) {
	service, repository := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	projected, err := repository.Get(ctx, account.AccountID)
	if err != nil {
		t.Fatalf("read account: %v", err)
	}

	if projected.AccountID != account.AccountID {
		t.Fatalf(
			"expected account id %s, got %s",
			account.AccountID,
			projected.AccountID,
		)
	}

	if projected.Currency != "USD" {
		t.Fatalf(
			"expected currency USD, got %s",
			projected.Currency,
		)
	}

	if projected.Timezone != "Africa/Addis_Ababa" {
		t.Fatalf(
			"expected timezone Africa/Addis_Ababa, got %s",
			projected.Timezone,
		)
	}

	if projected.Status != StatusPendingVerification {
		t.Fatalf(
			"expected PENDING_VERIFICATION status, got %s",
			projected.Status,
		)
	}

	if len(projected.PaymentMethods) != 0 {
		t.Fatalf(
			"expected no payment methods on creation, got %v",
			projected.PaymentMethods,
		)
	}
}

func TestActivateAccount(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	if account.Status != StatusPendingVerification {
		t.Fatalf(
			"expected PENDING_VERIFICATION after creation, got %s",
			account.Status,
		)
	}

	activated, err := service.ActivateAccount(
		ctx,
		account.AccountID,
	)
	if err != nil {
		t.Fatalf("activate account: %v", err)
	}

	if activated.Status != StatusActive {
		t.Fatalf(
			"expected ACTIVE after activation, got %s",
			activated.Status,
		)
	}
}

func TestAddPaymentMethod_AppendsToActiveAccount(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	_, err = service.ActivateAccount(
		ctx,
		account.AccountID,
	)
	if err != nil {
		t.Fatalf("activate account: %v", err)
	}

	updated, err := service.AddPaymentMethod(
		ctx,
		account.AccountID,
		"pm_chapa_active",
	)
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	if len(updated.PaymentMethods) != 1 ||
		updated.PaymentMethods[0] != "pm_chapa_active" {
		t.Fatalf(
			"expected PaymentMethods to contain pm_chapa_active, got %v",
			updated.PaymentMethods,
		)
	}
}

func TestAddPaymentMethod_RejectsPendingAccount(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	_, err = service.AddPaymentMethod(
		ctx,
		account.AccountID,
		"pm_chapa_active",
	)

	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf(
			"expected invalid state transition, got %v",
			err,
		)
	}
}

func TestAddPaymentMethod_RejectsUnknownPaymentMethodID(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	_, err = service.AddPaymentMethod(
		ctx,
		account.AccountID,
		"pm_does_not_exist",
	)

	if !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Fatalf(
			"expected ErrPaymentMethodNotFound, got %v",
			err,
		)
	}
}

func TestSuspendAccount(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	account, err = service.ActivateAccount(
		ctx,
		account.AccountID,
	)
	if err != nil {
		t.Fatalf("activate account: %v", err)
	}

	account, err = service.SuspendAccount(
		ctx,
		account.AccountID,
		"compliance review",
	)
	if err != nil {
		t.Fatalf("suspend account: %v", err)
	}

	if account.Status != StatusSuspended {
		t.Fatalf(
			"expected SUSPENDED, got %s",
			account.Status,
		)
	}
}

func TestCloseAccount(t *testing.T) {
	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	account, err = service.ActivateAccount(
		ctx,
		account.AccountID,
	)
	if err != nil {
		t.Fatalf("activate account: %v", err)
	}

	account, err = service.CloseAccount(
		ctx,
		account.AccountID,
		"customer requested closure",
	)
	if err != nil {
		t.Fatalf("close account: %v", err)
	}

	if account.Status != StatusClosed {
		t.Fatalf(
			"expected CLOSED, got %s",
			account.Status,
		)
	}
}
