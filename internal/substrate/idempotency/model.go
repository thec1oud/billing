package idempotency

import (
	"encoding/json"
	"time"
)

// ProceedToken proves that its holder owns a reservation.
type ProceedToken struct {
	key   string
	value string
}

// Decision either permits one execution or contains a cached response.
type Decision struct {
	ProceedToken   *ProceedToken
	CachedResponse json.RawMessage
}

func (d Decision) ShouldProceed() bool { return d.ProceedToken != nil }

type record struct {
	OperationType string
	RequestHash   string
	Token         string
	Response      json.RawMessage
	Completed     bool
	ExpiresAt     time.Time
}
