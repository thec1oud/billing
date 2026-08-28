package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/account/repository"
	"github.com/thec1oud/billing/internal/ppi/adapters"
	"github.com/thec1oud/billing/internal/shared/money"
)

type Service struct {
	repository *repository.Repository
}

func New(repository *repository.Repository) *Service {
	return &Service{
		repository: repository,
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
		return model.Account{}, fmt.Errorf("invalid timezone %q: %w", in.Timezone, err)
	}

	if in.NetTerms < 0 {
		return model.Account{}, errors.New("net terms must not be negative")
	}

	return s.repository.Create(ctx, in)
}

func (s *Service) Get(
	ctx context.Context,
	accountID int64,
) (model.Account, error) {
	return s.repository.Get(ctx, accountID)
}

func (s *Service) Activate(
	ctx context.Context,
	accountID int64,
) (model.Account, error) {
	return s.transition(
		ctx,
		accountID,
		model.StatusPendingVerification,
		model.StatusActive,
	)
}

func (s *Service) Suspend(
	ctx context.Context,
	accountID int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"suspension reason is required",
		)
	}

	return s.transition(
		ctx,
		accountID,
		model.StatusActive,
		model.StatusSuspended,
	)
}

func (s *Service) Close(
	ctx context.Context,
	accountID int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"closure reason is required",
		)
	}

	a, err := s.repository.Get(ctx, accountID)
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

	if err := s.repository.UpdateStatus(
		ctx,
		accountID,
		a.Status,
		model.StatusClosed,
	); err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, accountID)
}

func (s *Service) AddPaymentMethod(
	ctx context.Context,
	accountID int64,
	paymentMethodID string,
) (model.Account, error) {
	if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
		return model.Account{}, ErrPaymentMethodNotFound
	}

	if err := s.repository.AddPaymentMethod(
		ctx,
		accountID,
		paymentMethodID,
	); err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, accountID)
}

func (s *Service) transition(
	ctx context.Context,
	accountID int64,
	from,
	to model.Status,
) (model.Account, error) {
	a, err := s.repository.Get(ctx, accountID)
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

	if err := s.repository.UpdateStatus(
		ctx,
		accountID,
		from,
		to,
	); err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, accountID)
}

var ErrPaymentMethodNotFound = errors.New(
	"payment method not found",
)
