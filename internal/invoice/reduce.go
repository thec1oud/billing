package invoice

import (
	"encoding/json"
	"fmt"

	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
)

func Reduce(state Invoice, event sharedEvents.Event) (Invoice, error) {
	switch event.EventType {
	case sharedEvents.InvoiceCreated:
		var p CreatedPayload
		if err := json.Unmarshal(event.Payload, &p); err != nil {
			return state, fmt.Errorf("unmarshal InvoiceCreated payload: %w", err)
		}
		return Invoice{
			InvoiceID:      event.AggregateID,
			AccountID:      p.AccountID,
			SubscriptionID: p.SubscriptionID,
			Status:         StatusDraft,
			Currency:       p.Currency,
			PeriodStart:    p.PeriodStart,
			PeriodEnd:      p.PeriodEnd,
			LineItems:      p.LineItems,
			Total:          p.Total,
		}, nil

	case sharedEvents.InvoiceFinalized:
		state.Status = StatusOpen
		return state, nil
	case sharedEvents.PaymentAttempted:
		return state, nil // no state change, just a record in the stream

	case sharedEvents.PaymentSucceeded:
		return state, nil // InvoicePaid is what actually transitions status

	case sharedEvents.PaymentFailed:
		return state, nil // no retry/dunning state to track yet

	case sharedEvents.InvoicePaid:
		state.Status = StatusPaid
		return state, nil
	default:
		return state, nil
	}
}
