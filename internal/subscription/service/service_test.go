package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/subscription/model"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
)

func TestCreateValidation(t *testing.T) {
	t.Parallel()

	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "test"}

	tests := []struct {
		name        string
		svc         *Service
		input       model.CreateInput
		errContains string
	}{
		{
			name:        "invalid account id and plan id",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 0, PlanID: 0, PlanVersion: 1},
			errContains: "account id and plan id must be greater than zero",
		},
		{
			name:        "invalid plan version",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 0},
			errContains: "plan version must be greater than zero",
		},
		{
			name:        "nil plan repo",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "plan repository is not configured",
		},
		{
			name:        "nil account repo",
			svc:         New(nil, nil, &mockPlanRepo{}, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "account repository is not configured",
		},
		{
			name:        "nil subscription repo",
			svc:         New(nil, &accountrepo.Repository{}, &mockPlanRepo{}, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "subscription repository is not configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.svc.Create(context.Background(), actor, tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

func TestCalculatePeriod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		start         time.Time
		anchor        time.Time
		end           time.Time
		wantStart     time.Time
		wantAnchor    time.Time
		wantEnd       time.Time
		wantErrSubstr string
	}{
		// Acceptance criterion: 29th start date
		{
			name:       "start 29th non-leap year clamps to Feb 28",
			start:      time.Date(2025, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.February, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			name:       "start 29th leap year preserves Feb 29",
			start:      time.Date(2024, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.January, 29, 10, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: 30th start date
		{
			name:       "start 30th non-leap year clamps to Feb 28",
			start:      time.Date(2025, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.February, 28, 8, 0, 0, 0, time.UTC),
		},
		{
			name:       "start 30th leap year clamps to Feb 29",
			start:      time.Date(2024, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.January, 30, 8, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2024, time.February, 29, 8, 0, 0, 0, time.UTC),
		},
		{
			name:       "start 30th March to April 30",
			start:      time.Date(2025, time.March, 30, 9, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.March, 30, 9, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.March, 30, 9, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.April, 30, 9, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: 31st start date
		{
			name:       "start 31st Jan non-leap clamps to Feb 28",
			start:      time.Date(2025, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.February, 28, 14, 0, 0, 0, time.UTC),
		},
		{
			name:       "start 31st Jan leap year clamps to Feb 29",
			start:      time.Date(2024, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.January, 31, 14, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2024, time.February, 29, 14, 0, 0, 0, time.UTC),
		},
		{
			name:       "start 31st March clamps to April 30",
			start:      time.Date(2025, time.March, 31, 16, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.March, 31, 16, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.March, 31, 16, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.April, 30, 16, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: February in leap years
		{
			name:       "start Feb 29 in leap year rolls to March 29",
			start:      time.Date(2024, time.February, 29, 11, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.February, 29, 11, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.February, 29, 11, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2024, time.March, 29, 11, 0, 0, 0, time.UTC),
		},
		{
			name:       "start Feb 28 with anchor 29 in leap year rolls to March 29",
			start:      time.Date(2024, time.February, 28, 11, 0, 0, 0, time.UTC),
			anchor:     time.Date(2024, time.January, 29, 11, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.February, 28, 11, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.January, 29, 11, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2024, time.March, 29, 11, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: December rollover
		{
			name:       "December 31 rollover to next year January 31",
			start:      time.Date(2024, time.December, 31, 23, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2024, time.December, 31, 23, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2024, time.December, 31, 23, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.January, 31, 23, 0, 0, 0, time.UTC),
		},
		{
			name:       "December 15 rollover to next year January 15",
			start:      time.Date(2025, time.December, 15, 10, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.December, 15, 10, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.December, 15, 10, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2026, time.January, 15, 10, 0, 0, 0, time.UTC),
		},

		// Explicit end supplied
		{
			name:       "explicit end time preserved",
			start:      time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			anchor:     time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			end:        time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
			wantStart:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantAnchor: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:    time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStart, gotAnchor, gotEnd, err := CalculatePeriod(tc.start, tc.anchor, tc.end)
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !gotStart.Equal(tc.wantStart) || gotStart.Location() != time.UTC {
				t.Fatalf("start = %s, want %s", gotStart, tc.wantStart)
			}
			if !gotAnchor.Equal(tc.wantAnchor) || gotAnchor.Location() != time.UTC {
				t.Fatalf("anchor = %s, want %s", gotAnchor, tc.wantAnchor)
			}
			if !gotEnd.Equal(tc.wantEnd) || gotEnd.Location() != time.UTC {
				t.Fatalf("end = %s, want %s", gotEnd, tc.wantEnd)
			}
		})
	}
}

func TestCalculatePeriod_ZeroDefaultsToNow(t *testing.T) {
	before := time.Now().UTC()
	start, anchor, end, err := CalculatePeriod(time.Time{}, time.Time{}, time.Time{})
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start.Before(before) || start.After(after) {
		t.Fatalf("start %v not in [%v, %v]", start, before, after)
	}
	if !anchor.Equal(start) {
		t.Fatalf("expected anchor equal to start, got %v vs %v", anchor, start)
	}
	if !end.After(start) {
		t.Fatalf("expected end %v after start %v", end, start)
	}
}

func TestBillingProjection_PopulatesAllFields(t *testing.T) {
	t.Parallel()

	periodStart := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)

	sub := model.Subscription{
		SubscriptionID:     42,
		AccountID:          100,
		PlanID:             200,
		PlanVersion:        3,
		Status:             model.StatusActive,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		BillingCycleAnchor: periodStart,
	}

	tariff := tariffmodel.Tariff{
		ID:       50,
		Amount:   money.MustNew(2500, "USD"),
		IsActive: true,
	}

	mockSubRepo := &mockSubscriptionRepo{
		getFn: func(ctx context.Context, id int64) (model.Subscription, error) {
			if id == 42 {
				return sub, nil
			}
			return model.Subscription{}, model.ErrNotFound
		},
	}

	mockPlan := &mockPlanRepo{
		listDurationsFn: func(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error) {
			return []planrepo.PlanDuration{
				{PlanID: planID, TariffID: 50, IsActive: true},
			}, nil
		},
	}

	mockTariff := &mockTariffRepo{
		getByIDFn: func(ctx context.Context, id int64) (tariffmodel.Tariff, error) {
			if id == 50 {
				return tariff, nil
			}
			return tariffmodel.Tariff{}, tariffrepo.ErrTariffNotFound
		},
	}

	svc := New(mockSubRepo, nil, mockPlan, mockTariff, nil, nil)

	proj, err := svc.BillingProjection(context.Background(), 42)
	if err != nil {
		t.Fatalf("BillingProjection failed: %v", err)
	}

	if proj.SubscriptionID != 42 {
		t.Errorf("SubscriptionID = %d, want 42", proj.SubscriptionID)
	}
	if proj.AccountID != 100 {
		t.Errorf("AccountID = %d, want 100", proj.AccountID)
	}
	if proj.PlanID != 200 {
		t.Errorf("PlanID = %d, want 200", proj.PlanID)
	}
	if proj.PlanVersion != 3 {
		t.Errorf("PlanVersion = %d, want 3", proj.PlanVersion)
	}
	if proj.Amount != tariff.Amount {
		t.Errorf("Amount = %+v, want %+v", proj.Amount, tariff.Amount)
	}
	if proj.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", proj.Currency)
	}
	if !proj.PeriodStart.Equal(periodStart) {
		t.Errorf("PeriodStart = %v, want %v", proj.PeriodStart, periodStart)
	}
	if !proj.PeriodEnd.Equal(periodEnd) {
		t.Errorf("PeriodEnd = %v, want %v", proj.PeriodEnd, periodEnd)
	}
}

func TestBillingProjection_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		svc         *Service
		subID       int64
		errContains string
	}{
		{
			name:        "nil subscription repo",
			svc:         New(nil, nil, &mockPlanRepo{}, &mockTariffRepo{}, nil, nil),
			subID:       1,
			errContains: "subscription repository is not configured",
		},
		{
			name: "nil plan repo",
			svc: New(&mockSubscriptionRepo{
				getFn: func(ctx context.Context, id int64) (model.Subscription, error) {
					return model.Subscription{SubscriptionID: id}, nil
				},
			}, nil, nil, &mockTariffRepo{}, nil, nil),
			subID:       1,
			errContains: "plan repository is not configured",
		},
		{
			name: "nil tariff repo",
			svc: New(&mockSubscriptionRepo{
				getFn: func(ctx context.Context, id int64) (model.Subscription, error) {
					return model.Subscription{SubscriptionID: id, PlanID: 10}, nil
				},
			}, nil, &mockPlanRepo{
				listDurationsFn: func(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error) {
					return []planrepo.PlanDuration{{TariffID: 1, IsActive: true}}, nil
				},
			}, nil, nil, nil),
			subID:       1,
			errContains: "tariff repository is not configured",
		},
		{
			name: "inactive tariff",
			svc: New(&mockSubscriptionRepo{
				getFn: func(ctx context.Context, id int64) (model.Subscription, error) {
					return model.Subscription{SubscriptionID: id, PlanID: 10}, nil
				},
			}, nil, &mockPlanRepo{
				listDurationsFn: func(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error) {
					return []planrepo.PlanDuration{{TariffID: 1, IsActive: true}}, nil
				},
			}, &mockTariffRepo{
				getByIDFn: func(ctx context.Context, id int64) (tariffmodel.Tariff, error) {
					return tariffmodel.Tariff{ID: id, IsActive: false}, nil
				},
			}, nil, nil),
			subID:       1,
			errContains: "is inactive",
		},
		{
			name: "no active durations",
			svc: New(&mockSubscriptionRepo{
				getFn: func(ctx context.Context, id int64) (model.Subscription, error) {
					return model.Subscription{SubscriptionID: id, PlanID: 10}, nil
				},
			}, nil, &mockPlanRepo{
				listDurationsFn: func(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error) {
					return []planrepo.PlanDuration{{TariffID: 1, IsActive: false}}, nil
				},
			}, &mockTariffRepo{}, nil, nil),
			subID:       1,
			errContains: "must have exactly one active billing duration, got 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.svc.BillingProjection(context.Background(), tc.subID)
			if err == nil || !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

type mockSubscriptionRepo struct {
	getFn func(ctx context.Context, id int64) (model.Subscription, error)
}

func (m *mockSubscriptionRepo) Pool() *pgxpool.Pool { return nil }
func (m *mockSubscriptionRepo) Create(ctx context.Context, db sqlcgen.DBTX, in model.CreateInput) (model.Subscription, error) {
	return model.Subscription{}, nil
}
func (m *mockSubscriptionRepo) Get(ctx context.Context, db sqlcgen.DBTX, id int64) (model.Subscription, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return model.Subscription{}, nil
}
func (m *mockSubscriptionRepo) ListAccountSubscriptions(
	ctx context.Context, db sqlcgen.DBTX, accountID int64,
) ([]model.Subscription, error) {
	return nil, nil
}

type mockPlanRepo struct {
	planrepo.Repository
	listDurationsFn func(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error)
}

func (m *mockPlanRepo) ListDurations(ctx context.Context, planID int64) ([]planrepo.PlanDuration, error) {
	if m.listDurationsFn != nil {
		return m.listDurationsFn(ctx, planID)
	}
	return nil, nil
}

type mockTariffRepo struct {
	tariffrepo.Repository
	getByIDFn func(ctx context.Context, id int64) (tariffmodel.Tariff, error)
}

func (m *mockTariffRepo) GetByID(ctx context.Context, id int64) (tariffmodel.Tariff, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return tariffmodel.Tariff{}, nil
}
