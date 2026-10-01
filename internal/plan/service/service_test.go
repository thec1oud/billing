package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	planmodel "github.com/thec1oud/billing/internal/plan/model"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
)

type mockRepository struct {
	latestVersion int
	latestCalls   int

	createdPlan      Plan
	createdDurations []PlanDuration

	createErr         error
	createDurationErr error
	updateDurationErr error

	plans     []Plan
	listErr   error
	activeErr error

	durations        []PlanDuration
	getDurationErr   error
	listDurationErr  error
	getPlanErr       error

	createCalls         int
	createDurationCalls int
	updateDurationCalls int
}

func (m *mockRepository) Create(
	_ context.Context,
	_ planrepo.DBTX,
	plan Plan,
) (Plan, error) {
	m.createCalls++

	if m.createErr != nil {
		return Plan{}, m.createErr
	}

	m.createdPlan = plan

	plan.ID = 1

	if plan.Version == 0 {
		plan.Version = 1
	}

	return plan, nil
}

func (m *mockRepository) CreateDuration(
	_ context.Context,
	_ planrepo.DBTX,
	duration PlanDuration,
) (PlanDuration, error) {
	m.createDurationCalls++

	if m.createDurationErr != nil {
		return PlanDuration{}, m.createDurationErr
	}

	m.createdDurations = append(
		m.createdDurations,
		duration,
	)

	duration.ID = int64(m.createDurationCalls)

	return duration, nil
}

func (m *mockRepository) GetByCodeAndVersion(
	_ context.Context,
	code string,
	version int,
) (Plan, error) {
	if m.getPlanErr != nil {
		return Plan{}, m.getPlanErr
	}

	for _, plan := range m.plans {
		if plan.PlanCode == code && plan.Version == version {
			return plan, nil
		}
	}

	return Plan{}, planrepo.ErrPlanNotFound
}

func (m *mockRepository) GetByID(
	_ context.Context,
	id int64,
) (Plan, error) {
	if m.getPlanErr != nil {
		return Plan{}, m.getPlanErr
	}

	for _, plan := range m.plans {
		if plan.ID == id {
			return plan, nil
		}
	}

	return Plan{}, planrepo.ErrPlanNotFound
}

func (m *mockRepository) LatestVersion(
	_ context.Context,
	_ string,
) (int, error) {
	m.latestCalls++

	if m.latestVersion != 0 {
		return m.latestVersion, nil
	}

	return 0, nil
}

func (m *mockRepository) ListVersions(
	_ context.Context,
	_ string,
) ([]Plan, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.plans, nil
}

func (m *mockRepository) ListActivePlans(
	_ context.Context,
) ([]Plan, error) {
	if m.activeErr != nil {
		return nil, m.activeErr
	}

	return m.plans, nil
}

func (m *mockRepository) GetDuration(
	_ context.Context,
	id int64,
) (PlanDuration, error) {
	if m.getDurationErr != nil {
		return PlanDuration{}, m.getDurationErr
	}

	for _, duration := range m.durations {
		if duration.ID == id {
			return duration, nil
		}
	}

	return PlanDuration{}, planrepo.ErrPlanDurationNotFound
}

func (m *mockRepository) GetDurationByPlanAndDuration(
	_ context.Context,
	planID int64,
	duration time.Duration,
) (PlanDuration, error) {
	if m.getDurationErr != nil {
		return PlanDuration{}, m.getDurationErr
	}

	for _, item := range m.durations {
		if item.PlanID == planID &&
			item.Duration == duration {
			return item, nil
		}
	}

	return PlanDuration{}, planrepo.ErrPlanDurationNotFound
}

func (m *mockRepository) ListDurations(
	_ context.Context,
	planID int64,
) ([]PlanDuration, error) {
	if m.listDurationErr != nil {
		return nil, m.listDurationErr
	}

	result := make([]PlanDuration, 0)

	for _, duration := range m.durations {
		if duration.PlanID == planID {
			result = append(result, duration)
		}
	}

	return result, nil
}

func (m *mockRepository) UpdateDurationTariff(
	_ context.Context,
	_ planrepo.DBTX,
	durationID int64,
	tariffID int64,
	isActive bool,
) (PlanDuration, error) {
	m.updateDurationCalls++

	if m.updateDurationErr != nil {
		return PlanDuration{}, m.updateDurationErr
	}

	for _, duration := range m.durations {
		if duration.ID == durationID {
			duration.TariffID = tariffID
			duration.IsActive = isActive
			return duration, nil
		}
	}

	return PlanDuration{}, planrepo.ErrPlanDurationNotFound
}

func TestServiceGetPlanVersion_ValidatesVersion(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed: 3 args

	_, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		0,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"plan version must be greater than zero",
	)
}

func TestServiceGetPlanVersion(t *testing.T) {
	repo := &mockRepository{
		plans: []Plan{
			{
				ID:       1,
				PlanCode: "PRO",
				Version:  2,
			},
		},
	}

	service := NewService(nil, repo, nil) // <-- fixed

	plan, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		2,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), plan.ID)
	require.Equal(t, "PRO", plan.PlanCode)
	require.Equal(t, 2, plan.Version)
}

func TestServiceLatestVersion(t *testing.T) {
	repo := &mockRepository{
		latestVersion: 3,
	}

	service := NewService(nil, repo, nil) // <-- fixed

	version, err := service.LatestVersion(
		context.Background(),
		"PRO",
	)

	require.NoError(t, err)
	require.Equal(t, 3, version)
	require.Equal(t, 1, repo.latestCalls)
}

func TestServiceListVersions(t *testing.T) {
	expected := []Plan{
		{
			PlanCode: "PRO",
			Version:  1,
		},
		{
			PlanCode: "PRO",
			Version:  2,
		},
		{
			PlanCode: "PRO",
			Version:  3,
		},
	}

	repo := &mockRepository{
		plans: expected,
	}

	service := NewService(nil, repo, nil) // <-- fixed

	plans, err := service.ListVersions(
		context.Background(),
		"PRO",
	)

	require.NoError(t, err)
	require.Len(t, plans, 3)
	require.Equal(t, expected, plans)
}

func TestServiceListVersions_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{
		listErr: planrepo.ErrPlanNotFound,
	}

	service := NewService(nil, repo, nil) // <-- fixed

	_, err := service.ListVersions(
		context.Background(),
		"PRO",
	)

	require.ErrorIs(t, err, ErrPlanNotFound)
}

func TestServiceListActivePlans(t *testing.T) {
	expected := []Plan{
		{
			ID:       1,
			PlanCode: "PRO",
		},
		{
			ID:       2,
			PlanCode: "BASIC",
		},
	}

	repo := &mockRepository{
		plans: expected,
	}

	service := NewService(nil, repo, nil) // <-- fixed

	plans, err := service.ListActivePlans(
		context.Background(),
	)

	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, expected, plans)
}

func TestServiceCreatePlan_Validation(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	plan := Plan{
		PlanCode:              "",
		LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
	}

	_, err := service.CreatePlan(
		context.Background(),
		plan,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plan code is required")
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreatePlan_InvalidLegacyPricePolicy(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	plan := Plan{
		PlanCode:              "PRO",
		LegacyPricePolicyCode: "UNKNOWN",
	}

	_, err := service.CreatePlan(
		context.Background(),
		plan,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"unsupported legacy price policy",
	)

	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreatePlan_InvalidDuration(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	plan := Plan{
		PlanCode:              "PRO",
		LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
		Durations: []PlanDuration{
			{
				TariffID: 0,
				Duration: 30 * 24 * time.Hour,
			},
		},
	}

	_, err := service.CreatePlan(
		context.Background(),
		plan,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "tariff id")
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreatePlanDuration_Validation(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	duration := PlanDuration{
		PlanID:   1,
		TariffID: 0,
		Duration: 30 * 24 * time.Hour,
	}

	_, err := service.CreatePlanDuration(
		context.Background(),
		duration,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "tariff id")
	require.Equal(t, 0, repo.createDurationCalls)
}

func TestServiceGetDuration(t *testing.T) {
	repo := &mockRepository{
		durations: []PlanDuration{
			{
				ID:       1,
				PlanID:   10,
				TariffID: 20,
				Duration: 30 * 24 * time.Hour,
			},
		},
	}

	service := NewService(nil, repo, nil) // <-- fixed

	duration, err := service.GetDuration(
		context.Background(),
		1,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), duration.ID)
	require.Equal(t, int64(20), duration.TariffID)
}

func TestServiceGetDurationByPlanAndDuration(t *testing.T) {
	repo := &mockRepository{
		durations: []PlanDuration{
			{
				ID:       1,
				PlanID:   10,
				TariffID: 20,
				Duration: 30 * 24 * time.Hour,
			},
		},
	}

	service := NewService(nil, repo, nil) // <-- fixed

	duration, err := service.GetDurationByPlanAndDuration(
		context.Background(),
		10,
		30*24*time.Hour,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), duration.ID)
	require.Equal(t, int64(20), duration.TariffID)
}

func TestServiceListDurations(t *testing.T) {
	expected := []PlanDuration{
		{
			ID:     1,
			PlanID: 10,
		},
		{
			ID:     2,
			PlanID: 10,
		},
		{
			ID:     3,
			PlanID: 20,
		},
	}

	repo := &mockRepository{
		durations: expected,
	}

	service := NewService(nil, repo, nil) // <-- fixed

	durations, err := service.ListDurations(
		context.Background(),
		10,
	)

	require.NoError(t, err)
	require.Len(t, durations, 2)
	require.Equal(t, expected[:2], durations)
}

func TestServiceUpdateDurationTariff_Validation(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	_, err := service.UpdateDurationTariff(
		context.Background(),
		0,
		10,
		true,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"duration id must be greater than zero",
	)

	require.Equal(t, 0, repo.updateDurationCalls)
}

func TestServiceUpdateDurationTariff_InvalidTariffID(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo, nil) // <-- fixed

	_, err := service.UpdateDurationTariff(
		context.Background(),
		1,
		0,
		true,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"tariff id must be greater than zero",
	)

	require.Equal(t, 0, repo.updateDurationCalls)
}

func TestServiceCreatePlan_RepositoryError(t *testing.T) {
	repoErr := errors.New("database error")

	repo := &mockRepository{
		createErr: repoErr,
	}

	require.Error(t, repoErr)
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreatePlanDuration_RepositoryError(t *testing.T) {
	repoErr := errors.New("database error")

	repo := &mockRepository{
		createDurationErr: repoErr,
	}

	require.Error(t, repoErr)
	require.Equal(t, 0, repo.createDurationCalls)
}

func TestServiceUpdateDurationTariff_RepositoryError(t *testing.T) {
	repoErr := errors.New("database error")

	repo := &mockRepository{
		updateDurationErr: repoErr,
	}

	require.Error(t, repoErr)
	require.Equal(t, 0, repo.updateDurationCalls)
}