package api

import (
	"github.com/thec1oud/billing/internal/infra/messaging"
	accountHandler "github.com/thec1oud/billing/internal/account/handler"
	planHandler "github.com/thec1oud/billing/internal/plan/handler"
	subHandler "github.com/thec1oud/billing/internal/subscription/handler"
	tariffHandler "github.com/thec1oud/billing/internal/tariff/handler"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	ppiservice "github.com/thec1oud/billing/internal/ppi/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds all domain-level service dependencies required by the HTTP handlers
// registered in this package. Add a field here when wiring a new module's handler.
type Deps struct {
	Pool       *pgxpool.Pool
	PPIService *ppiservice.Service
	Broker     messaging.Broker

	AccountService      accountHandler.AccountService
	TariffService       tariffHandler.TariffService
	PlanService         planHandler.PlanService
	SubscriptionService subHandler.SubscriptionService
	InvoiceService      *invoiceservice.Service
}
