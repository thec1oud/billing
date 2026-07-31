package invoice

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/subscription"
	"github.com/thec1oud/billing/internal/shared/money"
)

var ErrNoPaymentMethodOnFile = errors.New("account has no payment method on file")

type PlanLookupFunc func(
	ctx context.Context,
	subscriptionID uuid.UUID,
) (accountID uuid.UUID, fee money.Money, periodStart, periodEnd time.Time, err error)

func (f PlanLookupFunc) FlatFeeForSubscription(
	ctx context.Context,
	subscriptionID uuid.UUID,
) (uuid.UUID, money.Money, time.Time, time.Time, error) {
	return f(ctx, subscriptionID)
}

// NewSubscriptionPlanLookup builds a PlanLookup backed by the real
// subscription and plan modules, replacing the CreateDraftInvoice test stub.
func NewSubscriptionPlanLookup(subscriptions *subscription.Service) PlanLookupFunc {
	return func(ctx context.Context, subscriptionID uuid.UUID) (uuid.UUID, money.Money, time.Time, time.Time, error) {
		projection, err := subscriptions.BillingProjection(ctx, subscriptionID)
		if err != nil {
			return uuid.Nil, money.Money{}, time.Time{}, time.Time{}, err
		}
		accountID, err := uuid.Parse(projection.AccountID)
		if err != nil {
			return uuid.Nil, money.Money{}, time.Time{}, time.Time{},
				errors.Join(err, errors.New("invalid account id in subscription billing projection"))
		}
		return accountID, projection.Amount, projection.PeriodStart, projection.PeriodEnd, nil
	}
}

// --- AccountLookup ---

// AccountLookupFunc adapts a plain function to the AccountLookup interface.
type AccountLookupFunc func(ctx context.Context, accountID uuid.UUID) (paymentMethodID string, err error)

func (f AccountLookupFunc) DefaultPaymentMethodID(ctx context.Context, accountID uuid.UUID) (string, error) {
	return f(ctx, accountID)
}

// NewAccountPaymentMethodLookup builds an AccountLookup backed by the real
// account module, replacing the AttemptPayment test stub.
func NewAccountPaymentMethodLookup(accounts *account.Service) AccountLookupFunc {
	return func(ctx context.Context, accountID uuid.UUID) (string, error) {
		acct, err := accounts.GetAccount(ctx, accountID)
		if err != nil {
			return "", err
		}
		if len(acct.PaymentMethods) == 0 {
			return "", ErrNoPaymentMethodOnFile
		}
		return acct.PaymentMethods[0], nil
	}
}
