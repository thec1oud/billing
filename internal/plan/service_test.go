package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/thec1oud/billing/internal/shared/money"
)

type mockRepository struct {
	plans        map[string]map[int]Plan
	latestVerErr error
	saveErr      error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		plans: make(map[string]map[int]Plan),
	}
}

func (m *mockRepository) Save(_ context.Context, p Plan) (Plan, error) {
	if m.saveErr != nil {
		return Plan{}, m.saveErr
	}

	if _, exists := m.plans[p.PlanCode]; !exists {
		m.plans[p.PlanCode] = make(map[int]Plan)
	}

	p.ID = int64(len(m.plans[p.PlanCode]) + 1)
	p.TariffID = p.ID + 100
	m.plans[p.PlanCode][p.Version] = p

	return p, nil
}

func (m *mockRepository) GetByCodeAndVersion(_ context.Context, code string, version int) (Plan, error) {
	versions, exists := m.plans[code]
	if !exists {
		return Plan{}, errors.New("plan not found")
	}

	p, exists := versions[version]
	if !exists {
		return Plan{}, errors.New("version not found")
	}

	return p, nil
}

func (m *mockRepository) LatestVersion(_ context.Context, code string) (int, error) {
	if m.latestVerErr != nil {
		return 0, m.latestVerErr
	}

	versions, exists := m.plans[code]
	if !exists || len(versions) == 0 {
		return 0, nil
	}

	latest := 0
	for v := range versions {
		if v > latest {
			latest = v
		}
	}

	return latest, nil
}

// ==========================================
// SERVICE UNIT TESTS
// ==========================================

func TestService_CreatePlan(t *testing.T) {
	testMoney, _ := money.New(1000, "USD")

	t.Run("creates version 1 for a brand new plan", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		result, err := svc.CreatePlan(context.Background(), "STARTER_PLAN", testMoney, BillingIntervalMonth)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if result.Version != 1 {
			t.Errorf("expected version 1, got %d", result.Version)
		}
		if result.PlanCode != "STARTER_PLAN" {
			t.Errorf("expected plan code STARTER_PLAN, got %s", result.PlanCode)
		}
		if result.Interval != BillingIntervalMonth {
			t.Errorf("expected billing interval MONTH, got %s", result.Interval)
		}
	})

	t.Run("increments version to 2 when version 1 exists", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		// Create Version 1
		_, err := svc.CreatePlan(context.Background(), "PRO_PLAN", testMoney, BillingIntervalMonth)
		if err != nil {
			t.Fatalf("failed to create initial plan: %v", err)
		}

		// Create Version 2
		v2Money, _ := money.New(1500, "USD")
		resultV2, err := svc.CreatePlan(context.Background(), "PRO_PLAN", v2Money, BillingIntervalYear)
		if err != nil {
			t.Fatalf("expected no error on v2 creation, got %v", err)
		}

		if resultV2.Version != 2 {
			t.Errorf("expected version 2, got %d", resultV2.Version)
		}
	})

	t.Run("returns error when repository save fails", func(t *testing.T) {
		repo := newMockRepository()
		repo.saveErr = errors.New("database error")
		svc := NewService(repo)

		_, err := svc.CreatePlan(context.Background(), "ENTERPRISE_PLAN", testMoney, BillingIntervalYear)
		if err == nil {
			t.Fatal("expected error from repo save failure, got nil")
		}
	})
}
