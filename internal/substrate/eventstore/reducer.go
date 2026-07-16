package events

// Reducer applies one event to a state value. Reducers must be deterministic.
type Reducer[State any] func(State, Event) (State, error)

// Rebuild folds an ordered stream into current state without changing the
// input stream. Aggregates and read projections use this same primitive.
func Rebuild[State any](initial State, stream []Event, reduce Reducer[State]) (State, error) {
	state := initial
	for _, event := range stream {
		var err error
		state, err = reduce(state, event)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}
