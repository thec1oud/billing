package model

import (
	"encoding/json"
	"fmt"
	"strconv"

	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

// Reduce is retained for event-stream consumers while the write model is PostgreSQL-backed.
func Reduce(state Invoice, event eventmodel.Event) (Invoice, error) {
	switch event.EventType {
	case eventmodel.InvoiceCreated:
		var payload CreatedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return state, fmt.Errorf("decode invoice.created: %w", err)
		}
		id, err := strconv.ParseInt(event.AggregateID, 10, 64)
		if err != nil {
			return state, fmt.Errorf("invalid invoice aggregate id %q: %w", event.AggregateID, err)
		}
		return Invoice{
			InvoiceID: id,
			AccountID: payload.AccountID,
			Status:    StatusDraft,
			Currency:  money.Currency(payload.Currency),
			LineItems: payload.LineItems,
			Total:     payload.Total,
		}, nil
	case eventmodel.InvoiceFinalized:
		state.Status = StatusOpen
	case eventmodel.InvoicePaid:
		state.Status = StatusPaid
	case eventmodel.InvoiceVoided:
		state.Status = StatusVoid
	}
	return state, nil
}
