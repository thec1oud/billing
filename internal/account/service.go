package account

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/thec1oud/billing/internal/ppi/adapters"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
)

var nextAccountID atomic.Int64

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
		accountID := nextAccountID.Add(1)

		event := AccountCreated{
			AccountID: accountID,
			Currency:  currency,
			Timezone:  timezone,
		}

		err := s.repository.Append(ctx, events.AppendRequest{
			AggregateType: events.AggregateAccount,
			AggregateID:   strconv.FormatInt(accountID, 10),
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

func (s *Service) AddPaymentMethod(
	ctx context.Context,
	accountID int64,
	paymentMethodID string,
) (*Account, error) {
	if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
		return nil, ErrPaymentMethodNotFound
	}

	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	nextSequence := account.Version + 1

	event := PaymentMethodAdded{
		AccountID:       accountID,
		PaymentMethodID: paymentMethodID,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   strconv.FormatInt(accountID, 10),
		Sequence:      nextSequence,
		EventType:     events.PaymentMethodAdded,
		EventVersion:  1,
		Actor:         "account.service",
		Payload:       event,
	})
	if err != nil {
		return nil, err
	}

	return s.repository.Get(ctx, accountID)

}

func (s *Service) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	return s.repository.Get(ctx, accountID)
}
