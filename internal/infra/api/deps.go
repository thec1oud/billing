package api

import (
	"github.com/thec1oud/billing/internal/account/service"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/plan"
	ppiservice "github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/purchasable_item"
	subscriptionservice "github.com/thec1oud/billing/internal/subscription/service"
)

// Deps holds all domain-level service dependencies required by the HTTP handlers
// registered in this package. Add a field here when wiring a new module's handler.
type Deps struct {
	PPIService             *ppiservice.Service
	Broker                 messaging.Broker
	AccountService         *service.Service
	PlanService            *plan.Service
	PurchasableItemService *purchasable_item.Service
	SubscriptionService    *subscriptionservice.Service
	InvoiceService         *invoiceservice.Service
}
