package service

import (
	"fmt"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
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
		WithLeftMargin(15).
		WithTopMargin(20).
		WithRightMargin(15).
		Build()

	m := maroto.New(cfg)

	// Paid At formatting
	paidDateStr := "Unknown Date"
	if invoice.PaidAt != nil {
		paidDateStr = invoice.PaidAt.Format("January 2, 2006")
	} else if invoice.FinalizedAt != nil {
		paidDateStr = invoice.FinalizedAt.Format("January 2, 2006")
	} else {
		paidDateStr = time.Now().Format("January 2, 2006")
	}

	// 1. Big Header: "{Amount} paid on {Date}"
	m.AddRow(20,
		col.New(12).Add(
			text.New(fmt.Sprintf("%s paid on %s", invoice.AmountPaid.Format(), paidDateStr), props.Text{
				Style: fontstyle.Bold,
				Size:  16,
				Top:   5,
			}),
		),
	)

	m.AddRow(10, col.New(12)) // Spacing

	// 2. Line Items Table Header
	m.AddRow(8,
		col.New(6).Add(text.New("Description", props.Text{Size: 9, Style: fontstyle.Bold})),
		col.New(1).Add(text.New("Qty", props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
		col.New(2).Add(text.New("Unit price", props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
		col.New(1).Add(text.New("Tax", props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
		col.New(2).Add(text.New("Amount", props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
	)
	m.AddRow(2, col.New(12).Add(line.New(props.Line{Thickness: 0.5, SizePercent: 100.0}))) // Top Border Line

	// 3. Line Items Data
	for _, item := range invoice.LineItems {
		m.AddRow(10,
			col.New(6).Add(text.New(item.Description, props.Text{Size: 9, Top: 2})),
			col.New(1).Add(text.New(fmt.Sprintf("%.0f", item.QuantityValue), props.Text{Size: 9, Align: align.Right, Top: 2})),
			col.New(2).Add(text.New(item.UnitAmount.Format(), props.Text{Size: 9, Align: align.Right, Top: 2})),
			col.New(1).Add(text.New("-", props.Text{Size: 9, Align: align.Right, Top: 2})), // Tax per item usually empty/implied in simplified view
			col.New(2).Add(text.New(item.TotalAmount.Format(), props.Text{Size: 9, Align: align.Right, Top: 2})),
		)
	}

	m.AddRow(10, col.New(12)) // Spacing

	// 4. Totals Block (Right Aligned)
	m.AddRow(6,
		col.New(7),
		col.New(3).Add(text.New("Subtotal", props.Text{Size: 9})),
		col.New(2).Add(text.New(invoice.Subtotal.Format(), props.Text{Size: 9, Align: align.Right})),
	)
	m.AddRow(6,
		col.New(7),
		col.New(3).Add(text.New("Tax", props.Text{Size: 9})),
		col.New(2).Add(text.New(invoice.Tax.Format(), props.Text{Size: 9, Align: align.Right})),
	)
	m.AddRow(6,
		col.New(7),
		col.New(3).Add(text.New("Total", props.Text{Size: 9, Style: fontstyle.Bold})),
		col.New(2).Add(text.New(invoice.Total.Format(), props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
	)

	m.AddRow(4, col.New(12)) // Small Spacing

	// Gray highlighted amount paid block (mocking with bold text and a subtle box if we could, but text is fine)
	m.AddRow(8,
		col.New(7),
		col.New(3).Add(text.New("Amount paid", props.Text{Size: 9, Style: fontstyle.Bold})),
		col.New(2).Add(text.New(invoice.AmountPaid.Format(), props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right})),
	)

	m.AddRow(20, col.New(12)) // Spacing

	// 6. Footer disclaimer
	m.AddRow(10,
		col.New(12).Add(text.New("Thank you for your business", props.Text{
			Size:  8,
			Color: &props.Color{Red: 100, Green: 100, Blue: 100}, // Gray text
		})),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate maroto PDF: %w", err)
	}

	return doc.GetBytes(), nil
}
