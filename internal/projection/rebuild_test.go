package projection_test

import (
	"encoding/json"
	"testing"

	"github.com/thec1oud/billing/internal/projection"
	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
)

func TestRebuildCounterFromEvents(t *testing.T) {
	t.Parallel()
	events := []sharedEvents.Event{
		{EventType: "counter.incremented", Payload: json.RawMessage(`{"amount": 7}`)},
		{EventType: "counter.decremented", Payload: json.RawMessage(`{"amount": 2}`)},
		{EventType: "counter.incremented", Payload: json.RawMessage(`{"amount": 4}`)},
	}

	state, err := projection.Rebuild(0, events, func(current int, event sharedEvents.Event) (int, error) {
		var payload struct {
			Amount int `json:"amount"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return current, err
		}
		switch event.EventType {
		case "counter.incremented":
			return current + payload.Amount, nil
		case "counter.decremented":
			return current - payload.Amount, nil
		default:
			return current, nil
		}
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if state != 9 {
		t.Fatalf("expected counter state 9, got %d", state)
	}
}
