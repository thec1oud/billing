package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *Repository) Create(
	ctx context.Context,
	db sqlcgen.DBTX,
	in model.CreateInput,
) (model.Account, error) {
	row, err := sqlcgen.New(db).CreateAccount(ctx, createParams(in))
	if err != nil {
		return model.Account{}, fmt.Errorf("create account: %w", err)
	}

	return r.toModel(
		ctx,
		db,
		row.AccountID,
		row.ExternalID,
		row.AccountStatusCode,
		row.Currency,
		row.Timezone,
		row.Locale,
		row.NetTerms,
		row.DunningProfileID,
		row.TaxIdentifiers,
		row.BillingAddress,
		row.ComplianceFlags,
		row.Metadata,
	)
}

func (r *Repository) UpdateStatus(
	ctx context.Context,
	db sqlcgen.DBTX,
	accountID int64,
	from,
	status model.Status,
) error {
	if accountID <= 0 {
		return errors.New("account id must be greater than zero")
	}

	rows, err := sqlcgen.New(db).UpdateAccountStatusFrom(
		ctx,
		sqlcgen.UpdateAccountStatusFromParams{
			AccountID:           accountID,
			AccountStatusCode:   string(from),
			AccountStatusCode_2: string(status),
		},
	)
	if err != nil {
		return fmt.Errorf("update account status: %w", err)
	}

	if rows != 1 {
		return fmt.Errorf(
			"update account status: %w",
			model.ErrInvalidStateTransition,
		)
	}

	return nil
}

func (r *Repository) AddPaymentMethod(
	ctx context.Context,
	db sqlcgen.DBTX,
	accountID int64,
	reference string,
) error {
	if accountID <= 0 || reference == "" {
		return errors.New(
			"account id and payment method reference are required",
		)
	}

	status, err := sqlcgen.New(db).LockAccountForUpdate(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("lock account: %w", err)
	}

	if model.Status(status) == model.StatusClosed {
		return model.ErrClosed
	}

	if model.Status(status) != model.StatusActive {
		return fmt.Errorf(
			"%w: payment method requires ACTIVE account",
			model.ErrInvalidStateTransition,
		)
	}

	if err := sqlcgen.New(db).ClearDefaultPaymentMethod(ctx, accountID); err != nil {
		return fmt.Errorf("clear default payment method: %w", err)
	}

	params := sqlcgen.CreatePaymentMethodParams{
		AccountID:         accountID,
		ProviderReference: reference,
	}

	if err := sqlcgen.New(db).CreatePaymentMethod(ctx, params); err != nil {
		return fmt.Errorf("create payment method: %w", err)
	}

	return nil
}

func createParams(in model.CreateInput) sqlcgen.CreateAccountParams {
	return sqlcgen.CreateAccountParams{
		ExternalID: pgtype.Text{
			String: in.ExternalID,
			Valid:  in.ExternalID != "",
		},
		Currency:         in.Currency,
		Timezone:         in.Timezone,
		Locale:           defaultLocale(in.Locale),
		NetTerms:         in.NetTerms,
		DunningProfileID: nullableID(in.DunningProfileID),
		TaxIdentifiers:   defaultJSON(in.TaxIdentifiers),
		BillingAddress:   defaultJSON(in.BillingAddress),
		ComplianceFlags:  defaultJSON(in.ComplianceFlags),
		Metadata:         defaultJSON(in.Metadata),
	}
}

func (r *Repository) Get(
	ctx context.Context,
	db sqlcgen.DBTX,
	accountID int64,
) (model.Account, error) {
	if accountID <= 0 {
		return model.Account{}, errors.New(
			"account id must be greater than zero",
		)
	}

	row, err := sqlcgen.New(db).GetAccount(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Account{}, model.ErrNotFound
	}

	if err != nil {
		return model.Account{}, fmt.Errorf(
			"get account %d: %w",
			accountID,
			err,
		)
	}

	return r.toModel(
		ctx,
		db,
		row.AccountID,
		row.ExternalID,
		row.AccountStatusCode,
		row.Currency,
		row.Timezone,
		row.Locale,
		row.NetTerms,
		row.DunningProfileID,
		row.TaxIdentifiers,
		row.BillingAddress,
		row.ComplianceFlags,
		row.Metadata,
	)
}

func (r *Repository) toModel(
	ctx context.Context,
	db sqlcgen.DBTX,
	id int64,
	external pgtype.Text,
	status,
	currency,
	timezone,
	locale string,
	netTerms int16,
	dunning pgtype.Int8,
	tax,
	address,
	flags,
	metadata []byte,
) (model.Account, error) {
	methods, err := sqlcgen.New(db).ListPaymentMethodReferences(
		ctx,
		id,
	)
	if err != nil {
		return model.Account{}, fmt.Errorf(
			"list payment methods for account %d: %w",
			id,
			err,
		)
	}

	var dunningID *int64

	if dunning.Valid {
		value := dunning.Int64
		dunningID = &value
	}

	return model.Account{
		AccountID:        id,
		ExternalID:       external.String,
		Status:           model.Status(status),
		Currency:         currency,
		Timezone:         timezone,
		Locale:           locale,
		NetTerms:         netTerms,
		DunningProfileID: dunningID,
		TaxIdentifiers:   tax,
		BillingAddress:   address,
		ComplianceFlags:  flags,
		Metadata:         metadata,
		PaymentMethods:   methods,
	}, nil
}

func defaultLocale(v string) string {
	if v == "" {
		return "en-US"
	}

	return v
}

func defaultJSON(v []byte) []byte {
	if len(v) == 0 {
		return []byte("{}")
	}

	return v
}

func nullableID(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}

	return pgtype.Int8{
		Int64: *v,
		Valid: true,
	}
}
