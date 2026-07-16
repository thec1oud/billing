package events_test

import (
	"encoding/json"
	"testing"

	events "github.com/thec1oud/billing/internal/substrate/eventstore"
)

func TestRebuildCounterFromEvents(t *testing.T) {
	stream := []events.Event{{EventType: "counter.incremented", Payload: json.RawMessage(`{"amount":7}`)}, {EventType: "counter.decremented", Payload: json.RawMessage(`{"amount":2}`)}, {EventType: "counter.incremented", Payload: json.RawMessage(`{"amount":4}`)}}
	state, err := events.Rebuild(0, stream, func(current int, event events.Event) (int, error) {
		var payload struct {
			Amount int `json:"amount"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return current, err
		}
		if event.EventType == "counter.incremented" {
			return current + payload.Amount, nil
		}
		return current - payload.Amount, nil
	})
	if err != nil || state != 9 {
		t.Fatalf("rebuild = %d, %v; want 9", state, err)
	}
}
