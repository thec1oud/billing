package api

import (
	"github.com/thec1oud/billing/internal/infra/messaging"
	ppiHandlers "github.com/thec1oud/billing/internal/ppi/handler"
)

// Deps holds all domain-level service dependencies required by the HTTP handlers
// registered in this package. Add a field here when wiring a new module's handler.
type Deps struct {
	PPIService ppiHandlers.WebhookService
	Broker     messaging.Broker

	// Future module services go here, e.g.:
	// InvoiceService invoiceHandlers.SomeServiceInterface
}
