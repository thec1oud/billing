package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/account/repository"
	"github.com/thec1oud/billing/internal/account/statemachine"
	"github.com/thec1oud/billing/internal/ppi/adapters"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
)

type Service struct {
	repository *repository.Repository
	events     *eventservice.Service
	smEngine   *engine.Engine
}

func New(repository *repository.Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func NewWithEvents(
	repository *repository.Repository,
	events *eventservice.Service,
	smEngine *engine.Engine,
) *Service {
	return &Service{
		repository: repository,
		events:     events,
		smEngine:   smEngine,
	}
}

func (s *Service) Create(
	ctx context.Context,
	actor eventmodel.Actor,
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

	tx, err := s.repository.Pool().Begin(ctx)
	if err != nil {
		return model.Account{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := s.repository.Create(ctx, tx, in)
	if err != nil {
		return model.Account{}, err
	}

	initialContext, _ := json.Marshal(map[string]any{
		"account_id": result.AccountID,
	})
	_, err = s.smEngine.CreateInstance(
		ctx,
		sm_model.MachineType(statemachine.AccountMachineType),
		"account",
		fmt.Sprintf("%d", result.AccountID),
		initialContext,
		engine.WithTx(tx), // share the outer tx: SM instance is committed atomically with the account row and event
	)
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to create state machine instance: %w", err)
	}

	_, err = s.events.AppendEvent(
		ctx,
		tx,
		eventmodel.AppendRequest{
			AggregateType: eventmodel.AggregateAccount,
			AggregateID:   strconv.FormatInt(result.AccountID, 10),
			EventType:     eventmodel.AccountCreated,
			EventVersion:  1,
			Actor: actor,
			Payload: account.AccountCreated{
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
		},
	)
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to append event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Account{}, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func (s *Service) Get(
	ctx context.Context,
	id int64,
) (model.Account, error) {
	return s.repository.Get(ctx, s.repository.Pool(), id)
}

func (s *Service) Activate(
	ctx context.Context,
	actor eventmodel.Actor,
	id int64,
) (model.Account, error) {
	instance, err := s.smEngine.GetInstanceBySubject(ctx, "account", fmt.Sprintf("%d", id), sm_model.MachineType(statemachine.AccountMachineType))
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to find state machine instance: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "activate", nil, engine.WithEventActor(actor))
	if err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, s.repository.Pool(), id)
}

func (s *Service) Reactivate(
	ctx context.Context,
	actor eventmodel.Actor,
	id int64,
) (model.Account, error) {
	instance, err := s.smEngine.GetInstanceBySubject(ctx, "account", fmt.Sprintf("%d", id), sm_model.MachineType(statemachine.AccountMachineType))
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to find state machine instance: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "reactivate", nil, engine.WithEventActor(actor))
	if err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, s.repository.Pool(), id)
}

func (s *Service) Suspend(
	ctx context.Context,
	actor eventmodel.Actor,
	id int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"suspension reason is required",
		)
	}

	instance, err := s.smEngine.GetInstanceBySubject(ctx, "account", fmt.Sprintf("%d", id), sm_model.MachineType(statemachine.AccountMachineType))
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to find state machine instance: %w", err)
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"reason": reason,
	})
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "suspend", payloadBytes, engine.WithEventActor(actor))
	if err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, s.repository.Pool(), id)
}

func (s *Service) Close(
	ctx context.Context,
	actor eventmodel.Actor,
	id int64,
	reason string,
) (model.Account, error) {
	if strings.TrimSpace(reason) == "" {
		return model.Account{}, errors.New(
			"closure reason is required",
		)
	}

	instance, err := s.smEngine.GetInstanceBySubject(ctx, "account", fmt.Sprintf("%d", id), sm_model.MachineType(statemachine.AccountMachineType))
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to find state machine instance: %w", err)
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"reason": reason,
	})
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "close", payloadBytes, engine.WithEventActor(actor))
	if err != nil {
		return model.Account{}, err
	}

	return s.repository.Get(ctx, s.repository.Pool(), id)
}

func (s *Service) AddPaymentMethod(
	ctx context.Context,
	actor eventmodel.Actor,
	id int64,
	paymentMethodID string,
) (model.Account, error) {
	if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
		return model.Account{}, ErrPaymentMethodNotFound
	}

	tx, err := s.repository.Pool().Begin(ctx)
	if err != nil {
		return model.Account{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repository.AddPaymentMethod(
		ctx,
		tx,
		id,
		paymentMethodID,
	); err != nil {
		return model.Account{}, err
	}

	result, err := s.repository.Get(ctx, tx, id)
	if err != nil {
		return model.Account{}, err
	}

	_, err = s.events.AppendEvent(
		ctx,
		tx,
		eventmodel.AppendRequest{
			AggregateType: eventmodel.AggregateAccount,
			AggregateID:   strconv.FormatInt(id, 10),
			EventType:     eventmodel.PaymentMethodAdded,
			EventVersion:  1,
			Actor: actor,
			Payload: account.PaymentMethodAdded{
				AccountID:       id,
				PaymentMethodID: paymentMethodID,
			},
		},
	)
	if err != nil {
		return model.Account{}, fmt.Errorf("failed to append event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Account{}, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

var ErrPaymentMethodNotFound = errors.New(
	"payment method not found",
)
