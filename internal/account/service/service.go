package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/account/repository"
	"github.com/thec1oud/billing/internal/ppi/adapters"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
)

type Service struct {
	repository *repository.Repository
	events     *eventservice.Service
}

func New(repository *repository.Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func NewWithEvents(
	repository *repository.Repository,
	events *eventservice.Service,
) *Service {
	return &Service{
		repository: repository,
		events:     events,
	}
}

func (s *Service) Create(
	ctx context.Context,
	in model.CreateInput,
) (model.Account, error) {
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.Timezone = strings.TrimSpace(in.Timezone)

	if _, ok := money.ParseCurrency(in.Currency); !ok {
		return model.Account{}, money.ErrInvalidCurrency
	}

	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return model.Account{}, fmt.Errorf(
			"invalid timezone %q: %w",
			in.Timezone,
			err,
		)
	}

	if in.NetTerms < 0 {
		return model.Account{}, errors.New(
			"net terms must not be negative",
		)
	}

	if s.events == nil {
		return s.repository.Create(ctx, in)
	}

	return s.inTransaction(
		ctx,
		func(tx pgx.Tx) (
			model.Account,
			eventmodel.EventType,
			any,
			error,
		) {
			result, err := s.repository.CreateTx(ctx, tx, in)

			return result,
				eventmodel.AccountCreated,
				account.AccountCreated{
					AccountID:        result.AccountID,
					ExternalID:       result.ExternalID,
					Currency:         result.Currency,
					Timezone:         result.Timezone,
					Locale:           result.Locale,
					NetTerms:         result.NetTerms,
					DunningProfileID: result.DunningProfileID,
					TaxIdentifiers:   result.TaxIdentifiers,
					BillingAddress:   result.BillingAddress,
					ComplianceFlags:  result.ComplianceFlags,
					Metadata:         result.Metadata,
				},
				err
		},
	)
}

func (s *Service) Get(
	ctx context.Context,
	id int64,
) (model.Account, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) Activate(
	ctx context.Context,
	id int64,
) (model.Account, error) {
	return s.transition(
		ctx,
		id,
		model.StatusPendingVerification,
		model.StatusActive,
		eventmodel.AccountActivated,
		func(a model.Account) any {
			return account.AccountActivated{
				AccountID: a.AccountID,
			}
		},
	)
}

func (s *Service) Reactivate(
	ctx context.Context,
	id int64,
) (model.Account, error) {
	return s.transition(
		ctx,
		id,
		model.StatusSuspended,
		model.StatusActive,
		eventmodel.AccountActivated,
		func(a model.Account) any {
			return account.AccountActivated{
				AccountID: a.AccountID,
			}
		},
	)
}

func (s *Service) Suspend(
	ctx context.Context,
	id int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"suspension reason is required",
		)
	}

	return s.transition(
		ctx,
		id,
		model.StatusActive,
		model.StatusSuspended,
		eventmodel.AccountSuspended,
		func(a model.Account) any {
			return account.AccountSuspended{
				AccountID: a.AccountID,
				Reason:    reason,
			}
		},
	)
}

func (s *Service) Close(
	ctx context.Context,
	id int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"closure reason is required",
		)
	}

	if s.events == nil {
		a, err := s.repository.Get(ctx, id)
		if err != nil {
			return model.Account{}, err
		}

		if a.Status != model.StatusActive &&
			a.Status != model.StatusSuspended {
			return model.Account{}, fmt.Errorf(
				"%w: cannot close account from %s",
				model.ErrInvalidStateTransition,
				a.Status,
			)
		}

		if err = s.repository.UpdateStatus(
			ctx,
			id,
			a.Status,
			model.StatusClosed,
		); err != nil {
			return model.Account{}, err
		}

		return s.repository.Get(ctx, id)
	}

	return s.inTransaction(
		ctx,
		func(tx pgx.Tx) (
			model.Account,
			eventmodel.EventType,
			any,
			error,
		) {
			a, err := s.repository.GetTx(ctx, tx, id)
			if err != nil {
				return model.Account{}, "", nil, err
			}

			if a.Status != model.StatusActive &&
				a.Status != model.StatusSuspended {
				return model.Account{}, "", nil, fmt.Errorf(
					"%w: cannot close account from %s",
					model.ErrInvalidStateTransition,
					a.Status,
				)
			}

			if err = s.repository.UpdateStatusTx(
				ctx,
				tx,
				id,
				a.Status,
				model.StatusClosed,
			); err != nil {
				return model.Account{}, "", nil, err
			}

			result, err := s.repository.GetTx(ctx, tx, id)

			return result,
				eventmodel.AccountClosed,
				account.AccountClosed{
					AccountID: id,
					Reason:    reason,
				},
				err
		},
	)
}

func (s *Service) AddPaymentMethod(
	ctx context.Context,
	id int64,
	paymentMethodID string,
) (model.Account, error) {
	if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
		return model.Account{}, ErrPaymentMethodNotFound
	}

	if s.events == nil {
		if err := s.repository.AddPaymentMethod(
			ctx,
			id,
			paymentMethodID,
		); err != nil {
			return model.Account{}, err
		}

		return s.repository.Get(ctx, id)
	}

	return s.inTransaction(
		ctx,
		func(tx pgx.Tx) (
			model.Account,
			eventmodel.EventType,
			any,
			error,
		) {
			if err := s.repository.AddPaymentMethodTx(
				ctx,
				tx,
				id,
				paymentMethodID,
			); err != nil {
				return model.Account{}, "", nil, err
			}

			result, err := s.repository.GetTx(ctx, tx, id)

			return result,
				eventmodel.PaymentMethodAdded,
				account.PaymentMethodAdded{
					AccountID:       id,
					PaymentMethodID: paymentMethodID,
				},
				err
		},
	)
}

func (s *Service) transition(
	ctx context.Context,
	id int64,
	from,
	to model.Status,
	eventType eventmodel.EventType,
	payload func(model.Account) any,
) (model.Account, error) {
	if s.events == nil {
		a, err := s.repository.Get(ctx, id)
		if err != nil {
			return model.Account{}, err
		}

		if a.Status != from {
			return model.Account{}, fmt.Errorf(
				"%w: cannot transition account from %s",
				model.ErrInvalidStateTransition,
				a.Status,
			)
		}

		if err = s.repository.UpdateStatus(
			ctx,
			id,
			from,
			to,
		); err != nil {
			return model.Account{}, err
		}

		return s.repository.Get(ctx, id)
	}

	return s.inTransaction(
		ctx,
		func(tx pgx.Tx) (
			model.Account,
			eventmodel.EventType,
			any,
			error,
		) {
			a, err := s.repository.GetTx(ctx, tx, id)
			if err != nil {
				return model.Account{}, "", nil, err
			}

			if a.Status != from {
				return model.Account{}, "", nil, fmt.Errorf(
					"%w: cannot transition account from %s",
					model.ErrInvalidStateTransition,
					a.Status,
				)
			}

			if err = s.repository.UpdateStatusTx(
				ctx,
				tx,
				id,
				from,
				to,
			); err != nil {
				return model.Account{}, "", nil, err
			}

			result, err := s.repository.GetTx(ctx, tx, id)

			return result,
				eventType,
				payload(result),
				err
		},
	)
}

func (s *Service) inTransaction(
	ctx context.Context,
	work func(
		pgx.Tx,
	) (model.Account, eventmodel.EventType, any, error),
) (model.Account, error) {
	var result model.Account

	err := s.repository.Transaction(
		ctx,
		func(tx pgx.Tx) error {
			var eventType eventmodel.EventType
			var payload any
			var err error

			result, eventType, payload, err = work(tx)
			if err != nil {
				return err
			}

			_, err = s.events.AppendEvent(
				ctx,
				tx,
				eventmodel.AppendRequest{
					AggregateType: eventmodel.AggregateAccount,
					AggregateID:   strconv.FormatInt(result.AccountID, 10),
					EventType:     eventType,
					EventVersion:  1,
					Actor: eventmodel.Actor{
						ID:   "billing-service",
						Type: eventmodel.ActorTypeSystem,
					},
					Payload: payload,
				},
			)

			return err
		},
	)

	return result, err
}

var ErrPaymentMethodNotFound = errors.New(
	"payment method not found",
)
