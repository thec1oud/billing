package events

type AggregateType string

const (
	AggregateAccount      AggregateType = "ACCOUNT"
	AggregateSubscription AggregateType = "SUBSCRIPTION"
	AggregateInvoice      AggregateType = "INVOICE"
)