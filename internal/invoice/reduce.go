//go:build ignore

package invoice

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/thec1oud/billing/internal/invoice/model"
	sharedEvents "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/money"
)

// Reduce is retained for event-stream consumers while the write model is PostgreSQL-backed.
func Reduce(state model.Invoice, event sharedEvents.Event) (model.Invoice, error) {
	switch event.EventType {
	case sharedEvents.InvoiceCreated:
		var payload model.CreatedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return state, fmt.Errorf("decode invoice.created: %w", err)
		}
		id, err := strconv.ParseInt(event.AggregateID, 10, 64)
		if err != nil {
			return state, fmt.Errorf("invalid invoice aggregate id %q: %w", event.AggregateID, err)
		}
		return model.Invoice{
			InvoiceID: id,
			AccountID: payload.AccountID,
			Status:    model.StatusDraft,
			Currency:  money.Currency(payload.Currency),
			LineItems: payload.LineItems,
			Total:     payload.Total,
		}, nil
	case sharedEvents.InvoiceFinalized:
		state.Status = model.StatusOpen
	case sharedEvents.InvoicePaid:
		state.Status = model.StatusPaid
	}
	return state, nil
}
