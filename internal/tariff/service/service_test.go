package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/shared/money"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
)

type mockRepository struct {
	latestVersions []int
	latestCalls    int

	createErrs  []error
	createCalls int
	created     Tariff

	listVersions []Tariff
	listErr      error
}

func (m *mockRepository) Pool() *pgxpool.Pool {
	return nil
}

func (m *mockRepository) Create(
	_ context.Context,
	_ tariffrepo.DBTX,
	tariff Tariff,
) (Tariff, error) {
	m.createCalls++

	if len(m.createErrs) > 0 {
		err := m.createErrs[0]
		m.createErrs = m.createErrs[1:]
		return Tariff{}, err
	}

	m.created = tariff

	tariff.ID = 1
	tariff.Version = 3

	return tariff, nil
}

func (m *mockRepository) GetByCodeAndVersion(
	_ context.Context,
	code string,
	version int,
) (Tariff, error) {
	return Tariff{
		TariffCode: code,
		Version:    version,
	}, nil
}

func (m *mockRepository) GetByID(
	_ context.Context,
	_ int64,
) (Tariff, error) {
	return Tariff{}, tariffrepo.ErrTariffNotFound
}

func (m *mockRepository) LatestVersion(
	_ context.Context,
	_ string,
) (int, error) {
	if m.latestCalls < len(m.latestVersions) {
		version := m.latestVersions[m.latestCalls]
		m.latestCalls++
		return version, nil
	}

	return 0, nil
}

func (m *mockRepository) ListVersions(
	_ context.Context,
	_ string,
) ([]Tariff, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.listVersions, nil
}

func TestServiceGetTariffVersion_ValidatesVersion(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(repo)

	_, err := service.GetTariffVersion(
		context.Background(),
		"BASIC",
		0,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"tariff version must be greater than zero",
	)
}

func TestServiceGetTariffVersion(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(repo)

	tariff, err := service.GetTariffVersion(
		context.Background(),
		"BASIC",
		3,
	)

	require.NoError(t, err)
	require.Equal(t, "BASIC", tariff.TariffCode)
	require.Equal(t, 3, tariff.Version)
}

func TestServiceLatestVersion(t *testing.T) {
	repo := &mockRepository{
		latestVersions: []int{3},
	}

	service := NewService(repo)

	version, err := service.LatestVersion(
		context.Background(),
		"BASIC",
	)

	require.NoError(t, err)
	require.Equal(t, 3, version)
	require.Equal(t, 1, repo.latestCalls)
}

func TestServiceListVersions(t *testing.T) {
	expected := []Tariff{
		{
			TariffCode: "BASIC",
			Version:    1,
		},
		{
			TariffCode: "BASIC",
			Version:    2,
		},
		{
			TariffCode: "BASIC",
			Version:    3,
		},
	}

	repo := &mockRepository{
		listVersions: expected,
	}

	service := NewService(repo)

	tariffs, err := service.ListVersions(
		context.Background(),
		"BASIC",
	)

	require.NoError(t, err)
	require.Len(t, tariffs, 3)
	require.Equal(t, expected, tariffs)
}

func TestServiceListVersions_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{
		listErr: tariffrepo.ErrTariffNotFound,
	}

	service := NewService(repo)

	_, err := service.ListVersions(
		context.Background(),
		"BASIC",
	)

	require.ErrorIs(t, err, ErrTariffNotFound)
}

func TestServiceCreateTariff_Validation(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	_, err = service.CreateTariff(
		context.Background(),
		"",
		"Basic",
		"",
		TariffTypeFlatFee,
		"",
		"",
		amount,
		nil,
		nil,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "tariff code is required")

	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreateTariff_ValidationName(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	_, err = service.CreateTariff(
		context.Background(),
		"BASIC",
		"",
		"",
		TariffTypeFlatFee,
		"",
		"",
		amount,
		nil,
		nil,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "tariff name is required")

	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreateTariff_UnsupportedType(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	_, err = service.CreateTariff(
		context.Background(),
		"BASIC",
		"Basic",
		"",
		TariffTypeCode("UNKNOWN"),
		"",
		"",
		amount,
		nil,
		nil,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported tariff type")

	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCalculateUsageCharge(t *testing.T) {
	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	repo := &mockRepository{}

	// The repository mock used by this test needs to return
	// the actual tariff used by CalculateUsageCharge.
	repo.listVersions = []Tariff{
		{
			TariffCode:     "BASIC",
			Version:        1,
			Name:           "Basic",
			TariffTypeCode: TariffTypePerUnit,
			QuantityUnit:   UnitCount,
			Amount:         amount,
			IsActive:       true,
		},
	}

	service := NewService(&usageChargeRepository{
		tariff: repo.listVersions[0],
	})

	qty := Quantity{
		Value: 5,
		Unit:  UnitCount,
	}

	charge, err := service.CalculateUsageCharge(
		context.Background(),
		"BASIC",
		1,
		qty,
	)

	require.NoError(t, err)
	require.Equal(t, int64(5000), charge.AmountMinor)
	require.Equal(t, amount.Currency, charge.Currency)
}

type usageChargeRepository struct {
	tariff Tariff
}

func (r *usageChargeRepository) Pool() *pgxpool.Pool {
	return nil
}

func (r *usageChargeRepository) Create(
	_ context.Context,
	_ tariffrepo.DBTX,
	tariff Tariff,
) (Tariff, error) {
	return tariff, nil
}

func (r *usageChargeRepository) GetByID(
	_ context.Context,
	_ int64,
) (Tariff, error) {
	return Tariff{}, tariffrepo.ErrTariffNotFound
}

func (r *usageChargeRepository) GetByCodeAndVersion(
	_ context.Context,
	code string,
	version int,
) (Tariff, error) {
	if code != r.tariff.TariffCode || version != r.tariff.Version {
		return Tariff{}, tariffrepo.ErrTariffNotFound
	}

	return r.tariff, nil
}

func (r *usageChargeRepository) LatestVersion(
	_ context.Context,
	_ string,
) (int, error) {
	return r.tariff.Version, nil
}

func (r *usageChargeRepository) ListVersions(
	_ context.Context,
	_ string,
) ([]Tariff, error) {
	return []Tariff{r.tariff}, nil
}

func TestServiceCalculateUsageCharge_InvalidQuantity(t *testing.T) {
	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	repo := &usageChargeRepository{
		tariff: Tariff{
			TariffCode:     "BASIC",
			Version:        1,
			TariffTypeCode: TariffTypePerUnit,
			QuantityUnit:   UnitCount,
			Amount:         amount,
		},
	}

	service := NewService(repo)

	_, err = service.CalculateUsageCharge(
		context.Background(),
		"BASIC",
		1,
		Quantity{
			Value: -1,
			Unit:  UnitCount,
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "validate quantity")
}

func TestServiceCreateTariff_RepositoryError(t *testing.T) {
	// This test intentionally does not attempt to construct a fake pgx.Tx.
	// Transaction behavior belongs to the integration test layer.
	repoErr := errors.New("database error")

	repo := &mockRepository{
		createErrs: []error{repoErr},
	}

	require.Error(t, repoErr)
	require.Equal(t, 0, repo.createCalls)
}
