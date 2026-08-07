// internal/payment/repository/postgres_repository.go
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
	ErrDuplicateIdempotency   = errors.New("idempotency key has already been used")
	ErrDuplicateAttemptNumber = errors.New("payment attempt number already exists for this invoice")
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

	var pmID pgtype.Int8
	if attempt.PaymentMethodID != nil {
		pmID = pgtype.Int8{Int64: *attempt.PaymentMethodID, Valid: true}
	}

	var providerTxID pgtype.Text
	if attempt.ProviderTxID != nil {
		providerTxID = pgtype.Text{String: *attempt.ProviderTxID, Valid: true}
	}

	var rawResp []byte
	if len(attempt.RawResponse) > 0 {
		rawResp = attempt.RawResponse
	}

	row, err := q.CreatePaymentAttempt(ctx, sqlcgen.CreatePaymentAttemptParams{
		InvoiceID:       attempt.InvoiceID,
		PaymentMethodID: pmID,
		AttemptNumber:   int32(attempt.AttemptNumber),
		IdempotencyKey:  attempt.IdempotencyKey,
		ProviderTxID:    providerTxID,
		AmountMinor:     attempt.Amount.AmountMinor,
		Currency:        string(attempt.Amount.Currency),
		Status:          string(attempt.Status),
		RawResponse:     rawResp,
	})
	if err != nil {
		return model.PaymentAttempt{}, r.handleError(err, "create payment attempt")
	}

	attempt.ID = row.ID
	attempt.CreatedAt = row.CreatedAt
	attempt.UpdatedAt = row.UpdatedAt

	return attempt, nil
}

func (r *PostgresRepository) GetNextAttemptNumber(ctx context.Context, db DBTX, invoiceID int64) (int, error) {
	q := r.getQuerier(db)

	num, err := q.GetNextAttemptNumber(ctx, invoiceID)
	if err != nil {
		return 0, fmt.Errorf("read next attempt number: %w", err)
	}

	return int(num), nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, db DBTX, id uuid.UUID) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPaymentAttemptByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read payment attempt by id: %w", err)
	}

	return r.mapRowToModel(row), nil
}

func (r *PostgresRepository) GetByIdempotencyKey(ctx context.Context, db DBTX, key string) (model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	row, err := q.GetPaymentAttemptByIdempotencyKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrPaymentAttemptNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("read payment attempt by idempotency key: %w", err)
	}

	return r.mapRowToModel(row), nil
}

func (r *PostgresRepository) GetByProviderTxID(
	ctx context.Context,
	db DBTX,
	providerCode string,
	providerTxID string,
) (model.PaymentAttempt, error) {
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

	return r.mapRowToModel(row), nil
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

	return r.mapRowToModel(row), nil
}

func (r *PostgresRepository) UpdateStatus(
	ctx context.Context,
	db DBTX,
	id uuid.UUID,
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

	err := q.UpdatePaymentAttemptStatus(ctx, sqlcgen.UpdatePaymentAttemptStatusParams{
		ID:           id,
		Status:       string(status),
		ProviderTxID: pTxID,
		RawResponse:  rawResp,
	})
	if err != nil {
		return r.handleError(err, "update payment attempt status")
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
		attempts = append(attempts, r.mapRowToModel(row))
	}

	return attempts, nil
}

func (r *PostgresRepository) ListByPaymentMethodID(ctx context.Context, db DBTX, paymentMethodID int64) ([]model.PaymentAttempt, error) {
	q := r.getQuerier(db)

	rows, err := q.ListPaymentAttemptsByPaymentMethodID(ctx, pgtype.Int8{Int64: paymentMethodID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list payment attempts by payment method id: %w", err)
	}

	attempts := make([]model.PaymentAttempt, 0, len(rows))
	for _, row := range rows {
		attempts = append(attempts, r.mapRowToModel(row))
	}

	return attempts, nil
}

func (r *PostgresRepository) handleError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		switch pgErr.ConstraintName {
		case "idx_payment_attempts_single_pending_per_invoice":
			return fmt.Errorf("%w: %s", ErrPendingAttemptExists, op)
		case "uq_payment_attempts_invoice_attempt":
			return fmt.Errorf("%w: %s", ErrDuplicateAttemptNumber, op)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *PostgresRepository) mapRowToModel(row sqlcgen.PaymentAttempt) model.PaymentAttempt {
	amt, _ := money.New(row.AmountMinor, row.Currency)

	var pmID *int64
	if row.PaymentMethodID.Valid {
		pmID = &row.PaymentMethodID.Int64
	}

	var providerTxID *string
	if row.ProviderTxID.Valid {
		providerTxID = &row.ProviderTxID.String
	}

	return model.PaymentAttempt{
		ID:              row.ID,
		InvoiceID:       row.InvoiceID,
		PaymentMethodID: pmID,
		AttemptNumber:   int(row.AttemptNumber),
		IdempotencyKey:  row.IdempotencyKey,
		ProviderTxID:    providerTxID,
		Amount:          amt,
		Status:          model.Status(row.Status),
		RawResponse:     row.RawResponse,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
