package account

import (
	"context"
	"encoding/json"
	"fmt"

	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
)

type Service struct {
	repository *Repository
	idem       *idempotency.Store
}

func NewService(
	repository *Repository,
	idem *idempotency.Store,
) *Service {
	return &Service{
		repository: repository,
		idem:       idem,
	}
}

// CreateAccount creates a new billing account.
//
// Milestone 1 deliberately skips the PENDING_VERIFICATION state.
// Every account becomes ACTIVE immediately.
func (s *Service) CreateAccount(
	ctx context.Context,
	currency string,
	timezone string,
	idempotencyKey string,
) (*Account, error) {

	requestHash := idempotency.HashRequest(
		[]byte(fmt.Sprintf("%s:%s", currency, timezone)),
	)

	decision, err := s.idem.CheckOrReserve(
		ctx,
		idempotencyKey,
		"account.create",
		requestHash,
	)
	if err != nil {
		return nil, err
	}

	// Replay a previously completed request.
	if !decision.ShouldProceed() {
		var cached Account

		if err := json.Unmarshal(decision.CachedResponse, &cached); err != nil {
			return nil, err
		}

		return &cached, nil
	}

	accountID, err := sharedUUID.New()
	if err != nil {
		return nil, err
	}

	event := AccountCreated{
		AccountID: accountID,
		Currency:  currency,
		Timezone:  timezone,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   accountID,
		Sequence:      1,
		EventType:     events.AccountCreated,
		EventVersion:  1,
		Actor:         "account.service",
		Payload:       event,
	})
	if err != nil {
		return nil, err
	}

	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if err := s.idem.StoreResponse(
		ctx,
		*decision.ProceedToken,
		account,
	); err != nil {
		return nil, err
	}

	return account, nil
}
