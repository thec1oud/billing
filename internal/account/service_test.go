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

func TestCreateAccountAndReadProjection(t *testing.T) {

	service, repository := newTestService()

	account, err := service.CreateAccount(
		context.Background(),
		"USD",
		"Africa/Addis_Ababa",
		"account-create-1",
	)

	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	projected, err := repository.Get(
		context.Background(),
		account.AccountID,
	)

	if err != nil {
		t.Fatalf("read projection: %v", err)
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

	if projected.Status != StatusActive {
		t.Fatalf(
			"expected ACTIVE status, got %s",
			projected.Status,
		)
	}

	if len(projected.PaymentMethods) != 0 {
		t.Fatalf(
			"expected PaymentMethods to be empty on creation, got %v",
			projected.PaymentMethods,
		)
	}
}

func TestAddPaymentMethod_AppendsToExistingAccount(t *testing.T) {

	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(ctx, "USD", "Africa/Addis_Ababa", "account-create-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	updated, err := service.AddPaymentMethod(ctx, account.AccountID, "pm_chapa_active")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	if len(updated.PaymentMethods) != 1 || updated.PaymentMethods[0] != "pm_chapa_active" {
		t.Fatalf(
			"expected PaymentMethods to contain pm_good, got %v",
			updated.PaymentMethods,
		)
	}
}

func TestAddPaymentMethod_RejectsUnknownPaymentMethodID(t *testing.T) {

	service, _ := newTestService()
	ctx := context.Background()

	account, err := service.CreateAccount(ctx, "USD", "Africa/Addis_Ababa", "account-create-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	_, err = service.AddPaymentMethod(ctx, account.AccountID, "pm_does_not_exist")
	if !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Fatalf("expected ErrPaymentMethodNotFound, got %v", err)
	}
}
