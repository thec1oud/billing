package service

import (
	"fmt"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"

	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
)

type GeneratorService interface {
	// Generate generates a PDF receipt for a given invoice and returns it as a byte array.
	Generate(invoice *invoicemodel.Invoice) ([]byte, error)
}

type pdfGeneratorService struct {
}

func NewGeneratorService() GeneratorService {
	return &pdfGeneratorService{}
}

func (s *pdfGeneratorService) Generate(invoice *invoicemodel.Invoice) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageNumber().
		WithLeftMargin(10).
		WithTopMargin(15).
		WithRightMargin(10).
		Build()

	m := maroto.New(cfg)

	// Header: Company / Logo area
	m.AddRow(20,
		col.New(12).Add(
			text.New("RECEIPT", props.Text{
				Top:   5,
				Style: fontstyle.Bold,
				Align: align.Center,
				Size:  20,
			}),
		),
	)

	// Invoice Information
	m.AddRow(10,
		col.New(6).Add(
			text.New(fmt.Sprintf("Invoice Number: %s", invoice.InvoiceNumber), props.Text{
				Top:  5,
				Size: 10,
			}),
		),
		col.New(6).Add(
			text.New(fmt.Sprintf("Date: %s", time.Now().Format("2006-01-02")), props.Text{
				Top:   5,
				Size:  10,
				Align: align.Right,
			}),
		),
	)

	m.AddRow(10,
		col.New(6).Add(
			text.New(fmt.Sprintf("Account ID: %d", invoice.AccountID), props.Text{
				Top:  5,
				Size: 10,
			}),
		),
		col.New(6).Add(
			text.New(fmt.Sprintf("Status: %s", string(invoice.Status)), props.Text{
				Top:   5,
				Size:  10,
				Align: align.Right,
			}),
		),
	)

	m.AddRow(10, col.New(12)) // Spacing

	// Table Header
	m.AddRow(10,
		col.New(6).Add(text.New("Description", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New("Qty", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New("Unit Price", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New("Total", props.Text{Style: fontstyle.Bold, Size: 10, Align: align.Right})),
	)

	// Line Items
	for _, item := range invoice.LineItems {
		m.AddRow(10,
			col.New(6).Add(text.New(item.Description, props.Text{Size: 10})),
			col.New(2).Add(text.New(fmt.Sprintf("%.2f %s", item.QuantityValue, item.QuantityUnit), props.Text{Size: 10})),
			col.New(2).Add(text.New(item.UnitAmount.Format(), props.Text{Size: 10})),
			col.New(2).Add(text.New(item.TotalAmount.Format(), props.Text{Size: 10, Align: align.Right})),
		)
	}

	m.AddRow(10, col.New(12)) // Spacing

	// Totals
	m.AddRow(10,
		col.New(8),
		col.New(2).Add(text.New("Subtotal:", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New(invoice.Subtotal.Format(), props.Text{Size: 10, Align: align.Right})),
	)
	m.AddRow(10,
		col.New(8),
		col.New(2).Add(text.New("Tax:", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New(invoice.Tax.Format(), props.Text{Size: 10, Align: align.Right})),
	)
	m.AddRow(10,
		col.New(8),
		col.New(2).Add(text.New("Discount:", props.Text{Style: fontstyle.Bold, Size: 10})),
		col.New(2).Add(text.New(invoice.Discount.Format(), props.Text{Size: 10, Align: align.Right})),
	)
	m.AddRow(10,
		col.New(8),
		col.New(2).Add(text.New("Total Paid:", props.Text{Style: fontstyle.Bold, Size: 12})),
		col.New(2).Add(text.New(invoice.AmountPaid.Format(), props.Text{Style: fontstyle.Bold, Size: 12, Align: align.Right})),
	)

	// Footer Message
	m.AddRow(30,
		col.New(12).Add(text.New("Thank you for your business!", props.Text{
			Top:   15,
			Style: fontstyle.Italic,
			Align: align.Center,
			Size:  10,
		})),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate maroto PDF: %w", err)
	}

	return doc.GetBytes(), nil
}
