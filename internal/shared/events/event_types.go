package events

type EventType string

const (
	// Account
	AccountCreated EventType = "billing.account.created"

	// Subscription
	SubscriptionCreated EventType = "billing.subscription.created"

	// Invoice
	InvoiceCreated   EventType = "billing.invoice.created"
	InvoiceFinalized EventType = "billing.invoice.finalized"
	InvoicePaid      EventType = "billing.invoice.paid"

	// Payment
	PaymentAttempted EventType = "billing.payment.attempted"
	PaymentSucceeded EventType = "billing.payment.succeeded"
	PaymentFailed    EventType = "billing.payment.failed"
)