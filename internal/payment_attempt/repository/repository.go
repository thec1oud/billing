// internal/payment/repository/postgres_repository.go
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/payment_attempt/model"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var (
	ErrPaymentAttemptNotFound = errors.New("payment attempt not found")
	ErrPendingAttemptExists   = errors.New("a pending payment attempt already exists for this invoice")
	ErrDuplicateInternalTxID  = errors.New("internal transaction id already exists")
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

func (r *PostgresRepository) Create(
	ctx context.Context,
	db DBTX,
	attempt model.PaymentAttempt,
) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	var rawReq []byte
	if len(attempt.RawRequest) > 0 {
		rawReq = attempt.RawRequest
	}

	row, err := q.CreatePaymentAttempt(ctx, sqlcgen.CreatePaymentAttemptParams{
		InvoiceID:    attempt.InvoiceID,
		ProviderCode: attempt.ProviderCode,
		InternalTxID: attempt.InternalTxID,
		AmountMinor:  attempt.Amount.AmountMinor,
		Currency:     string(attempt.Amount.Currency),
		Status:       string(attempt.Status),
		RawRequest:   rawReq,
	})
	if err != nil {
		return model.PaymentAttempt{}, r.handleError(err, "create payment attempt")
	}

	attempt.AttemptID = row.AttemptID
	attempt.CreatedAt = row.CreatedAt
	attempt.UpdatedAt = row.UpdatedAt

	return attempt, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, db DBTX, attemptID int64) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPaymentAttemptByID(ctx, attemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read payment attempt by id: %w", err)
	}

	return r.mapRowToModel(sqlcgen.PaymentAttempt(row)), nil
}

func (r *PostgresRepository) GetByInternalTxID(ctx context.Context, db DBTX, internalTxID string) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPaymentAttemptByInternalTxID(ctx, internalTxID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read payment attempt by internal tx id: %w", err)
	}

	return r.mapRowToModel(sqlcgen.PaymentAttempt(row)), nil
}

func (r *PostgresRepository) GetByProviderTxID(ctx context.Context, db DBTX, providerTxID string) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPaymentAttemptByProviderTxID(ctx, pgtype.Text{
		String: providerTxID,
		Valid:  true,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read payment attempt by provider tx id: %w", err)
	}

	return r.mapRowToModel(sqlcgen.PaymentAttempt(row)), nil
}

func (r *PostgresRepository) GetPendingByInvoiceID(ctx context.Context, db DBTX, invoiceID int64) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPendingPaymentAttemptByInvoiceID(ctx, invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read pending payment attempt: %w", err)
	}

	return r.mapRowToModel(sqlcgen.PaymentAttempt(row)), nil
}

func (r *PostgresRepository) UpdateResult(
	ctx context.Context,
	db DBTX,
	attemptID int64,
	status model.Status,
	providerTxID *string,
	rawResponse json.RawMessage,
) error {
	q := r.getQuerier(db)

	var pTxID pgtype.Text
	if providerTxID != nil {
		pTxID = pgtype.Text{String: *providerTxID, Valid: true}
	}

	var rawResp []byte
	if len(rawResponse) > 0 {
		rawResp = rawResponse
	}

	err := q.UpdatePaymentAttemptResult(ctx, sqlcgen.UpdatePaymentAttemptResultParams{
		AttemptID:    attemptID,
		Status:       string(status),
		ProviderTxID: pTxID,
		RawResponse:  rawResp,
	})
	if err != nil {
		return r.handleError(err, "update payment attempt result")
	}

	return nil
}

func (r *PostgresRepository) ListByInvoiceID(ctx context.Context, db DBTX, invoiceID int64) ([]model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	rows, err := q.ListPaymentAttemptsByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list payment attempts by invoice id: %w", err)
	}

	attempts := make([]model.PaymentAttempt, 0, len(rows))
	for _, row := range rows {
		attempts = append(attempts, r.mapRowToModel(sqlcgen.PaymentAttempt(row)))
	}

	return attempts, nil
}

func (r *PostgresRepository) DeleteByAttemptID(ctx context.Context, db DBTX, attemptID int64) error {
	q := r.getQuerier(db)

	tag, err := q.DeletePaymentAttempt(ctx, attemptID)
	if err != nil {
		return r.handleError(err, "delete payment attempt")
	}

	if tag.RowsAffected() == 0 {
		return ErrPaymentAttemptNotFound
	}

	return nil
}

func (r *PostgresRepository) handleError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		switch pgErr.ConstraintName {
		case "idx_payment_attempts_single_pending_per_invoice":
			return fmt.Errorf("%w: %s", ErrPendingAttemptExists, op)
		case "payment_attempts_internal_tx_id_key":
			return fmt.Errorf("%w: %s", ErrDuplicateInternalTxID, op)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *PostgresRepository) mapRowToModel(row sqlcgen.PaymentAttempt) model.PaymentAttempt {
	amt, _ := money.New(row.AmountMinor, row.Currency)

	var providerTxID *string
	if row.ProviderTxID.Valid {
		providerTxID = &row.ProviderTxID.String
	}

	return model.PaymentAttempt{
		AttemptID:    row.AttemptID,
		InvoiceID:    row.InvoiceID,
		ProviderCode: row.ProviderCode,
		InternalTxID: row.InternalTxID,
		ProviderTxID: providerTxID,
		Amount:       amt,
		Status:       model.Status(row.Status),
		RawResponse:  row.RawResponse,
		RawRequest:   row.RawRequest,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
