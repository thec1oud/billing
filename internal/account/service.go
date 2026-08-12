package account

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/ppi/adapters"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
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
// Account creation produces AccountCreated and leaves the
// account in PENDING_VERIFICATION.
//
// A separate ActivateAccount operation is responsible for
// transitioning the account to ACTIVE.
func (s *Service) CreateAccount(
	ctx context.Context,
	currency string,
	timezone string,
	idempotencyKey string,
) (*Account, error) {
	requestHash := idempotency.HashRequest(
		[]byte(fmt.Sprintf("%s:%s", currency, timezone)),
	)

	return idempotency.Execute(
		ctx,
		s.idem,
		idempotencyKey,
		"account.create",
		requestHash,
		func() (*Account, error) {
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
				AggregateID:   accountID.String(),
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
		},
	)
}

// ActivateAccount transitions an account from
// PENDING_VERIFICATION to ACTIVE.
func (s *Service) ActivateAccount(
	ctx context.Context,
	accountID uuid.UUID,
) (*Account, error) {
	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	event := AccountActivated{
		AccountID: accountID,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   accountID.String(),
		Sequence:      account.Version + 1,
		EventType:     events.AccountActivated,
		EventVersion:  1,
		Actor:         "account.service",
		Payload:       event,
	})
	if err != nil {
		return nil, err
	}

	return s.repository.Get(ctx, accountID)
}

// SuspendAccount transitions an ACTIVE account to SUSPENDED.
func (s *Service) SuspendAccount(
	ctx context.Context,
	accountID uuid.UUID,
	reason string,
) (*Account, error) {
	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	event := AccountSuspended{
		AccountID: accountID,
		Reason:    reason,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   accountID.String(),
		Sequence:      account.Version + 1,
		EventType:     events.AccountSuspended,
		EventVersion:  1,
		Actor:         "account.service",
		Payload:       event,
	})
	if err != nil {
		return nil, err
	}

	return s.repository.Get(ctx, accountID)
}

// CloseAccount transitions an ACTIVE or SUSPENDED account to CLOSED.
func (s *Service) CloseAccount(
	ctx context.Context,
	accountID uuid.UUID,
	reason string,
) (*Account, error) {
	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	event := AccountClosed{
		AccountID: accountID,
		Reason:    reason,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   accountID.String(),
		Sequence:      account.Version + 1,
		EventType:     events.AccountClosed,
		EventVersion:  1,
		Actor:         "account.service",
		Payload:       event,
	})
	if err != nil {
		return nil, err
	}

	return s.repository.Get(ctx, accountID)
}

// AddPaymentMethod attaches a valid payment method to
// an ACTIVE account.
func (s *Service) AddPaymentMethod(
	ctx context.Context,
	accountID uuid.UUID,
	paymentMethodID string,
) (*Account, error) {
	if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
		return nil, ErrPaymentMethodNotFound
	}

	account, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}

	event := PaymentMethodAdded{
		AccountID:       accountID,
		PaymentMethodID: paymentMethodID,
	}

	err = s.repository.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   accountID.String(),
		Sequence:      account.Version + 1,
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

func (s *Service) GetAccount(
	ctx context.Context,
	accountID uuid.UUID,
) (*Account, error) {
	return s.repository.Get(ctx, accountID)
}
