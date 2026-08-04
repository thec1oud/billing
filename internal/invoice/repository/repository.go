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
	ErrVersionConflict = errors.New("invoice was modified concurrently, retry")
	ErrInvalidStatus   = errors.New("invalid invoice status code")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
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

		lineItems = append(lineItems, model.LineItem{
			ItemID:        li.ItemID,
			Description:   li.Description,
			QuantityValue: qVal.Float64,
			QuantityUnit:  li.QuantityUnit,
			UnitAmount:    unitAmt,
			TotalAmount:   totalAmt,
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
		Currency:      row.Currency,
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
		Version:       row.Version,
	}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, tx pgx.Tx, inv model.Invoice) (int64, error) {
	q := sqlcgen.New(tx)

	fmt.Println(inv.Status)

	invoiceID, err := q.CreateInvoice(ctx, sqlcgen.CreateInvoiceParams{
		AccountID:         inv.AccountID,
		InvoiceStatusCode: string(inv.Status),
		Currency:          inv.Currency,
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
		if inv.SubscriptionID != 0 {
			subID = pgtype.Int8{Int64: inv.SubscriptionID, Valid: true}
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
	tx pgx.Tx,
	invoiceID int64,
	finalizedAt time.Time,
	expectedVersion int64,
) error {
	q := sqlcgen.New(tx)

	rows, err := q.FinalizeInvoice(ctx, sqlcgen.FinalizeInvoiceParams{
		FinalizedAt: pgtype.Timestamptz{Time: finalizedAt, Valid: true},
		InvoiceID:   invoiceID,
		Version:     expectedVersion,
	})
	if err != nil {
		return r.handleError(err, "finalize invoice")
	}
	if rows == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *PostgresRepository) MarkPaid(
	ctx context.Context,
	tx pgx.Tx,
	invoiceID int64,
	amountPaid money.Money,
	amountDue money.Money,
	paidAt time.Time,
	expectedVersion int64,
) error {
	q := sqlcgen.New(tx)

	rows, err := q.MarkInvoicePaid(ctx, sqlcgen.MarkInvoicePaidParams{
		AmountPaid: amountPaid.AmountMinor,
		AmountDue:  amountDue.AmountMinor,
		PaidAt:     pgtype.Timestamptz{Time: paidAt, Valid: true},
		InvoiceID:  invoiceID,
		Version:    expectedVersion,
	})
	if err != nil {
		return r.handleError(err, "mark invoice paid")
	}
	if rows == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *PostgresRepository) MarkVoid(
	ctx context.Context,
	tx pgx.Tx,
	invoiceID int64,
	expectedVersion int64,
) error {
	q := sqlcgen.New(tx)

	rows, err := q.VoidInvoice(ctx, sqlcgen.VoidInvoiceParams{
		InvoiceID: invoiceID,
		Version:   expectedVersion,
	})
	if err != nil {
		return r.handleError(err, "void invoice")
	}
	if rows == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *PostgresRepository) UpdatePaymentBalances(
	ctx context.Context,
	tx pgx.Tx,
	invoiceID int64,
	amountPaid money.Money,
	amountDue money.Money,
	expectedVersion int64,
) error {
	q := sqlcgen.New(tx)

	rows, err := q.UpdatePaymentBalances(ctx, sqlcgen.UpdatePaymentBalancesParams{
		AmountPaid: amountPaid.AmountMinor,
		AmountDue:  amountDue.AmountMinor,
		InvoiceID:  invoiceID,
		Version:    expectedVersion,
	})
	if err != nil {
		return r.handleError(err, "update payment balances")
	}
	if rows == 0 {
		return ErrVersionConflict
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
