package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/tariff/handler"
	tariff "github.com/thec1oud/billing/internal/tariff/model"
)

type mockTariffService struct {
	createErr error
}

func (m *mockTariffService) CreateTariff(
	ctx context.Context,
	code string,
	name string,
	description string,
	tariffType tariff.TariffTypeCode,
	amount money.Money,
	tiers []tariff.Tier,
	metadata []byte,
) (tariff.Tariff, error) {
	if m.createErr != nil {
		return tariff.Tariff{}, m.createErr
	}
	return tariff.Tariff{
		ID:             1,
		TariffCode:     code,
		Name:           name,
		Description:    description,
		TariffTypeCode: tariffType,
		Amount:         amount,
	}, nil
}

func TestHandleCreateTariff(t *testing.T) {
	svc := &mockTariffService{}
	h := handler.NewTariffHandler(svc)

	in := handler.CreateInput{
		Code:       "BASIC",
		Name:       "Basic Plan",
		TariffType: tariff.TariffTypeFlatFee,
		Amount:     money.MustNew(1000, "USD"),
	}
	body, _ := json.Marshal(in)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tariffs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleCreateTariff(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}
