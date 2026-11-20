package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thec1oud/billing/internal/config"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	ppiservice "github.com/thec1oud/billing/internal/ppi/service"
	itemservice "github.com/thec1oud/billing/internal/purchasable_item/service"
)

func TestDevelopmentRoutesUnavailableWhenDisabled(t *testing.T) {
	ppi := ppiservice.NewService(nil, nil, nil, nil)
	server := NewServer(&config.Config{AppPort: "0"}, Deps{
		InvoiceService:         &invoiceservice.Service{},
		PPIService:             ppi,
		PurchasableItemService: itemservice.NewService(nil),
	})

	for _, path := range []string{
		"/api/v1/dev/invoices/generate",
		"/api/v1/webhooks/fake",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("POST %s returned %d, want %d", path, response.Code, http.StatusNotFound)
			}
		})
	}

	if _, registered := ppi.GetAdapter("fake"); registered {
		t.Fatal("fake adapter should not be registered by default")
	}
}

func TestProtectedRoutesRequireBearerAuthentication(t *testing.T) {
	server := NewServer(&config.Config{AppPort: "0"}, Deps{
		InvoiceService: &invoiceservice.Service{},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/1", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request returned %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
