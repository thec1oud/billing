package service

import (
	"testing"
	"time"

	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

func TestGeneratorService_Generate(t *testing.T) {
	svc := NewGeneratorService()

	now := time.Now()
	invoice := &invoicemodel.Invoice{
		InvoiceID:     101,
		AccountID:     202,
		InvoiceNumber: "INV-101",
		Status:        invoicemodel.StatusPaid,
		Currency:      money.DefaultCurrency,
		Subtotal:      money.MustNew(10000, money.DefaultCurrency),
		Tax:           money.MustNew(500, money.DefaultCurrency),
		Discount:      money.MustNew(1000, money.DefaultCurrency),
		Total:         money.MustNew(9500, money.DefaultCurrency),
		AmountPaid:    money.MustNew(9500, money.DefaultCurrency),
		AmountDue:     money.MustZero(money.DefaultCurrency),
		DueAt:         &now,
		FinalizedAt:   &now,
		PaidAt:        &now,
		LineItems: []invoicemodel.LineItem{
			{
				Description:   "API Usage",
				QuantityValue: 1000,
				QuantityUnit:  "requests",
				UnitAmount:    money.MustNew(10, money.DefaultCurrency),
				TotalAmount:   money.MustNew(10000, money.DefaultCurrency),
			},
		},
	}

	pdfBytes, err := svc.Generate(invoice)
	if err != nil {
		t.Fatalf("expected no error from Generate, got: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Errorf("expected generated PDF byte slice to not be empty")
	}

	// Sanity check for PDF header signature
	if len(pdfBytes) > 4 && string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected file to begin with %%PDF, got %s", string(pdfBytes[:4]))
	}
}
