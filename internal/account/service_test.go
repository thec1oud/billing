package account

import (
	"context"
	"testing"

	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
)

func TestCreateAccountAndReadProjection(t *testing.T) {

	eventStore := events.NewEventStore(
		events.NewMemoryRepository(),
	)

	repository := NewRepository(eventStore)

	service := NewService(
		repository,
		idempotency.NewStore(),
	)

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
}
