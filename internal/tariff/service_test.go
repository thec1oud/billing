package tariff

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/shared/money"
)

type mockRepository struct {
	latestVersions []int
	latestCalls    int
	createErrs     []error
	createCalls    int
	created        Tariff
	listVersions   []Tariff
	listErr        error
}

func (m *mockRepository) Create(
	_ context.Context,
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

func (m *mockRepository) GetByID(_ context.Context, id int64) (Tariff, error) {
	return Tariff{ID: id}, nil
}

func (m *mockRepository) LatestVersion(
	_ context.Context,
	_ string,
) (int, error) {
	if m.latestCalls < len(m.latestVersions) {
		latest := m.latestVersions[m.latestCalls]
		m.latestCalls++
		return latest, nil
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

func TestServiceCreateTariff(t *testing.T) {
	repo := &mockRepository{
		latestVersions: []int{2},
	}

	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	created, err := service.CreateTariff(
		context.Background(),
		"BASIC",
		"Basic",
		"",
		TariffTypeFlatFee,
		amount,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, "BASIC", created.TariffCode)
	require.Equal(t, 3, created.Version)
	require.Equal(t, 1, repo.createCalls)
}

func TestServiceCreateTariff_RetriesAfterUniqueVersionConflict(t *testing.T) {
	repo := &mockRepository{
		latestVersions: []int{1, 2},
		createErrs: []error{
			&pgconn.PgError{Code: "23505", ConstraintName: "uq_tariff_code_version"},
		},
	}

	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	created, err := service.CreateTariff(
		context.Background(),
		"BASIC",
		"Basic",
		"",
		TariffTypeFlatFee,
		amount,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, 3, created.Version)
	require.Equal(t, 2, repo.createCalls)
}

func TestServiceListVersions_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{listErr: ErrTariffNotFound}
	service := NewService(repo)

	_, err := service.ListVersions(context.Background(), "BASIC")
	require.ErrorIs(t, err, ErrTariffNotFound)
}

func TestServiceCreateTariff_RetainsRepositoryError(t *testing.T) {
	repo := &mockRepository{
		latestVersions: []int{2},
		createErrs: []error{
			errors.New("database error"),
		},
	}

	service := NewService(repo)

	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	_, err = service.CreateTariff(
		context.Background(),
		"BASIC",
		"Basic",
		"",
		TariffTypeFlatFee,
		amount,
		nil,
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create tariff")
}
