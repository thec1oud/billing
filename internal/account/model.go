package account

import (
	"encoding/json"

	"github.com/google/uuid"

	events "github.com/thec1oud/billing/internal/shared/eventstore"
)

type Status string

const (
	StatusActive Status = "ACTIVE"
)

// Account is the Account aggregate.
type Account struct {
	AccountID      uuid.UUID
	Status         Status
	Currency       string
	Timezone       string
	PaymentMethods []string
	Version        int64
}

// Apply updates the aggregate by applying a single event.
func (a *Account) Apply(event events.Event) error {
	switch event.EventType {

	case events.AccountCreated:
		var payload AccountCreated

		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}

		a.AccountID = payload.AccountID
		a.Currency = payload.Currency
		a.Timezone = payload.Timezone
		a.Status = StatusActive
		a.PaymentMethods = []string{}
		a.Version = event.Sequence

	case events.PaymentMethodAdded:
		var payload PaymentMethodAdded

		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}

		a.PaymentMethods = append(a.PaymentMethods, payload.PaymentMethodID)
		a.Version = event.Sequence
	}

	return nil
}

// Rebuild reconstructs an Account aggregate from its event stream.
func Rebuild(stream []events.Event) (*Account, error) {
	account := &Account{}

	for _, event := range stream {
		if err := account.Apply(event); err != nil {
			return nil, err
		}
	}

	return account, nil
}
