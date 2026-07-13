
package projection

import sharedEvents "github.com/thec1oud/billing/internal/shared/events"

// Reducer applies one event to a state value. Reducers should be deterministic:
type Reducer[State any] func(State, sharedEvents.Event) (State, error)

// The input stream is never changed, which makes the function safe to use for replay and testing.
func Rebuild[State any](initial State, events []sharedEvents.Event, reduce Reducer[State]) (State, error) {
	state := initial
	for _, event := range events {
		var err error
		state, err = reduce(state, event)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}
