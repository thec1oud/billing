package api

import (
	"github.com/jackc/pgx/v5/pgxpool"
	accountservice "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/infra/messaging"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	planservice "github.com/thec1oud/billing/internal/plan/service"
	ppiservice "github.com/thec1oud/billing/internal/ppi/service"
	itemservice "github.com/thec1oud/billing/internal/purchasable_item/service"
	subscriptionservice "github.com/thec1oud/billing/internal/subscription/service"
	tariffservice "github.com/thec1oud/billing/internal/tariff/service"
)

// Deps holds all domain-level service dependencies required by the HTTP handlers
// registered in this package. Add a field here when wiring a new module's handler.
type Deps struct {
	Pool                   *pgxpool.Pool
	PPIService             *ppiservice.Service
	Broker                 messaging.Broker
	AccountService         *accountservice.Service
	PlanService            *planservice.Service
	TariffService          *tariffservice.Service
	PurchasableItemService *itemservice.Service
	SubscriptionService    *subscriptionservice.Service
	InvoiceService         *invoiceservice.Service
}
