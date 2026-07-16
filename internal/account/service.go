package account

import (
	"context"
	"fmt"

	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
	events "github.com/thec1oud/billing/internal/substrate/eventstore"
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

	return idempotency.Execute(ctx, s.idem, idempotencyKey, "account.create", requestHash, func() (*Account, error) {
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

		return s.repository.Get(ctx, accountID)
	})
}
