package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var (
	ErrInvoiceNotFound = errors.New("invoice not found")
	ErrInvalidStatus   = errors.New("invalid invoice status code")
)

type DBTX interface {
	sqlcgen.DBTX
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) getQuerier(db DBTX) *sqlcgen.Queries {
	if db != nil {
		return sqlcgen.New(db)
	}
	return sqlcgen.New(r.pool)
}

func (r *PostgresRepository) Get(ctx context.Context, invoiceID int64) (model.Invoice, error) {
	q := sqlcgen.New(r.pool)

	row, err := q.GetInvoice(ctx, invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return model.Invoice{}, fmt.Errorf("read invoice: %w", err)
	}

	subtotal, _ := money.New(row.SubtotalAmount, row.Currency)
	tax, _ := money.New(row.TaxAmount, row.Currency)
	discount, _ := money.New(row.DiscountAmount, row.Currency)
	total, _ := money.New(row.TotalAmount, row.Currency)
	amountPaid, _ := money.New(row.AmountPaid, row.Currency)
	amountDue, _ := money.New(row.AmountDue, row.Currency)

	lineItemRows, err := q.ListInvoiceLineItems(ctx, invoiceID)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("read line items: %w", err)
	}

	lineItems := make([]model.LineItem, 0, len(lineItemRows))
	for _, li := range lineItemRows {
		unitAmt, _ := money.New(li.UnitAmount, row.Currency)
		totalAmt, _ := money.New(li.TotalAmount, row.Currency)

		qVal, _ := li.QuantityValue.Float64Value()

		var subID int64
		if li.SubscriptionID.Valid {
			subID = li.SubscriptionID.Int64
		}

		lineItems = append(lineItems, model.LineItem{
			ItemID:         li.ItemID,
			SubscriptionID: subID,
			Description:    li.Description,
			QuantityValue:  qVal.Float64,
			QuantityUnit:   li.QuantityUnit,
			UnitAmount:     unitAmt,
			TotalAmount:    totalAmt,
		})
	}

	var dueAt, finalizedAt, paidAt *time.Time
	if row.DueAt.Valid {
		dueAt = &row.DueAt.Time
	}
	if row.FinalizedAt.Valid {
		finalizedAt = &row.FinalizedAt.Time
	}
	if row.PaidAt.Valid {
		paidAt = &row.PaidAt.Time
	}

	var invoiceNum string
	if row.InvoiceNumber.Valid {
		invoiceNum = row.InvoiceNumber.String
	}

	return model.Invoice{
		InvoiceID:     row.InvoiceID,
		AccountID:     row.AccountID,
		InvoiceNumber: invoiceNum,
		Status:        model.Status(row.InvoiceStatusCode),
		Currency:      money.Currency(row.Currency),
		Subtotal:      subtotal,
		Tax:           tax,
		Discount:      discount,
		Total:         total,
		AmountPaid:    amountPaid,
		AmountDue:     amountDue,
		DueAt:         dueAt,
		FinalizedAt:   finalizedAt,
		PaidAt:        paidAt,
		LineItems:     lineItems,
	}, nil
}

func (r *PostgresRepository) ListByAccount(ctx context.Context, db DBTX, accountID int64) ([]model.Invoice, error) {
	q := r.getQuerier(db)

	rows, err := q.ListInvoicesByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list invoices by account: %w", err)
	}

	invoices := make([]model.Invoice, 0, len(rows))
	for _, row := range rows {
		subtotal, _ := money.New(row.SubtotalAmount, row.Currency)
		tax, _ := money.New(row.TaxAmount, row.Currency)
		discount, _ := money.New(row.DiscountAmount, row.Currency)
		total, _ := money.New(row.TotalAmount, row.Currency)
		amountPaid, _ := money.New(row.AmountPaid, row.Currency)
		amountDue, _ := money.New(row.AmountDue, row.Currency)

		var dueAt, finalizedAt, paidAt *time.Time
		if row.DueAt.Valid {
			dueAt = &row.DueAt.Time
		}
		if row.FinalizedAt.Valid {
			finalizedAt = &row.FinalizedAt.Time
		}
		if row.PaidAt.Valid {
			paidAt = &row.PaidAt.Time
		}

		var invoiceNum string
		if row.InvoiceNumber.Valid {
			invoiceNum = row.InvoiceNumber.String
		}

		invoices = append(invoices, model.Invoice{
			InvoiceID:     row.InvoiceID,
			AccountID:     row.AccountID,
			InvoiceNumber: invoiceNum,
			Status:        model.Status(row.InvoiceStatusCode),
			Currency:      money.Currency(row.Currency),
			Subtotal:      subtotal,
			Tax:           tax,
			Discount:      discount,
			Total:         total,
			AmountPaid:    amountPaid,
			AmountDue:     amountDue,
			DueAt:         dueAt,
			FinalizedAt:   finalizedAt,
			PaidAt:        paidAt,
			LineItems:     nil, // Intentionally left empty for list views
		})
	}
	return invoices, nil
}

func (r *PostgresRepository) Create(ctx context.Context, db DBTX, inv model.Invoice) (int64, error) {
	q := r.getQuerier(db)

	invoiceID, err := q.CreateInvoice(ctx, sqlcgen.CreateInvoiceParams{
		AccountID:         inv.AccountID,
		InvoiceStatusCode: string(inv.Status),
		Currency:          string(inv.Currency),
		SubtotalAmount:    inv.Subtotal.AmountMinor,
		TaxAmount:         inv.Tax.AmountMinor,
		DiscountAmount:    inv.Discount.AmountMinor,
		TotalAmount:       inv.Total.AmountMinor,
		AmountPaid:        inv.AmountPaid.AmountMinor,
		AmountDue:         inv.AmountDue.AmountMinor,
	})
	if err != nil {
		return 0, r.handleError(err, "insert invoice")
	}

	for _, li := range inv.LineItems {
		var subID pgtype.Int8
		if li.SubscriptionID != 0 {
			subID = pgtype.Int8{Int64: li.SubscriptionID, Valid: true}
		}

		var qVal pgtype.Numeric
		_ = qVal.Scan(fmt.Sprintf("%.4f", li.QuantityValue))

		metaBytes := []byte("{}")
		if len(li.Metadata) > 0 {
			if b, err := json.Marshal(li.Metadata); err == nil {
				metaBytes = b
			}
		}

		err := q.CreateInvoiceLineItem(ctx, sqlcgen.CreateInvoiceLineItemParams{
			InvoiceID:      invoiceID,
			ItemID:         li.ItemID,
			SubscriptionID: subID,
			Description:    li.Description,
			QuantityValue:  qVal,
			QuantityUnit:   li.QuantityUnit,
			UnitAmount:     li.UnitAmount.AmountMinor,
			TotalAmount:    li.TotalAmount.AmountMinor,
			Metadata:       metaBytes,
		})
		if err != nil {
			return 0, fmt.Errorf("insert line item: %w", err)
		}
	}

	return invoiceID, nil
}
func (r *PostgresRepository) Finalize(
	ctx context.Context,
	db DBTX,
	invoiceID int64,
	subtotal, tax, discount, total, amountDue money.Money,
	dueAt, finalizedAt time.Time,
) (string, error) {
	q := r.getQuerier(db)

	invoiceNumber, err := q.FinalizeInvoice(ctx, sqlcgen.FinalizeInvoiceParams{
		SubtotalAmount: subtotal.AmountMinor,
		TaxAmount:      tax.AmountMinor,
		DiscountAmount: discount.AmountMinor,
		TotalAmount:    total.AmountMinor,
		AmountDue:      amountDue.AmountMinor,
		DueAt:          pgtype.Timestamptz{Time: dueAt, Valid: true},
		FinalizedAt:    pgtype.Timestamptz{Time: finalizedAt, Valid: true},
		InvoiceID:      invoiceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: invoice %d is not in draft status or does not exist", ErrInvalidStatus, invoiceID)
		}
		return "", r.handleError(err, "finalize invoice")
	}

	return invoiceNumber.String, nil
}

func (r *PostgresRepository) MarkPaid(
	ctx context.Context,
	db DBTX,
	invoiceID int64,
	amountPaid money.Money,
	amountDue money.Money,
	paidAt time.Time,
) error {
	q := r.getQuerier(db)

	if _, err := q.MarkInvoicePaid(ctx, sqlcgen.MarkInvoicePaidParams{
		AmountPaid: amountPaid.AmountMinor,
		AmountDue:  amountDue.AmountMinor,
		PaidAt:     pgtype.Timestamptz{Time: paidAt, Valid: true},
		InvoiceID:  invoiceID,
	}); err != nil {
		return r.handleError(err, "mark invoice paid")
	}
	return nil
}

func (r *PostgresRepository) MarkVoid(
	ctx context.Context,
	db DBTX,
	invoiceID int64,
) error {
	q := r.getQuerier(db)

	if _, err := q.VoidInvoice(ctx, invoiceID); err != nil {
		return r.handleError(err, "void invoice")
	}
	return nil
}

func (r *PostgresRepository) UpdatePaymentBalances(
	ctx context.Context,
	db DBTX,
	invoiceID int64,
	amountPaid money.Money,
	amountDue money.Money,
) error {
	q := r.getQuerier(db)

	if _, err := q.UpdatePaymentBalances(ctx, sqlcgen.UpdatePaymentBalancesParams{
		AmountPaid: amountPaid.AmountMinor,
		AmountDue:  amountDue.AmountMinor,
		InvoiceID:  invoiceID,
	}); err != nil {
		return r.handleError(err, "update payment balances")
	}
	return nil
}

func (r *PostgresRepository) handleError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if pgErr.ConstraintName == "invoices_invoice_status_code_fkey" {
			return fmt.Errorf("%w: %s", ErrInvalidStatus, op)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
