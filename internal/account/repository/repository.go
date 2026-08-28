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

func (r *Repository) Transaction(
	ctx context.Context,
	fn func(pgx.Tx) error,
) error {
	if r == nil || r.pool == nil {
		return errors.New("account repository: database is not configured")
	}

	return pgx.BeginFunc(ctx, r.pool, fn)
}

func (r *Repository) CreateTx(
	ctx context.Context,
	tx pgx.Tx,
	in model.CreateInput,
) (model.Account, error) {
	row, err := sqlcgen.New(tx).CreateAccount(ctx, createParams(in))
	if err != nil {
		return model.Account{}, fmt.Errorf("create account: %w", err)
	}

	return r.toModelWith(
		ctx,
		tx,
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

func (r *Repository) GetTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID int64,
) (model.Account, error) {
	return r.getWith(ctx, tx, accountID)
}

func (r *Repository) UpdateStatusTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID int64,
	from,
	status model.Status,
) error {
	if accountID <= 0 {
		return errors.New("account id must be greater than zero")
	}

	const statement = `UPDATE accounts
		SET account_status_code = $3
		WHERE account_id = $1 AND account_status_code = $2`

	tag, err := tx.Exec(
		ctx,
		statement,
		accountID,
		string(from),
		string(status),
	)
	if err != nil {
		return fmt.Errorf("update account status: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return fmt.Errorf(
			"update account status: %w",
			model.ErrInvalidStateTransition,
		)
	}

	return nil
}

func (r *Repository) AddPaymentMethodTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID int64,
	reference string,
) error {
	if accountID <= 0 || reference == "" {
		return errors.New(
			"account id and payment method reference are required",
		)
	}

	const lockAccount = `SELECT account_status_code
		FROM accounts WHERE account_id = $1 FOR UPDATE`

	var status string

	if err := tx.QueryRow(
		ctx,
		lockAccount,
		accountID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrNotFound
		}

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

	if _, err := tx.Exec(
		ctx,
		`UPDATE payment_methods
		 SET is_default = FALSE
		 WHERE account_id = $1
		   AND is_default`,
		accountID,
	); err != nil {
		return fmt.Errorf("clear default payment method: %w", err)
	}

	params := sqlcgen.CreatePaymentMethodParams{
		AccountID:         accountID,
		ProviderReference: reference,
	}

	if err := sqlcgen.New(tx).CreatePaymentMethod(ctx, params); err != nil {
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

func (r *Repository) Create(
	ctx context.Context,
	in model.CreateInput,
) (model.Account, error) {
	if r == nil || r.pool == nil {
		return model.Account{}, errors.New(
			"account repository: database is not configured",
		)
	}

	row, err := sqlcgen.New(r.pool).CreateAccount(
		ctx,
		createParams(in),
	)
	if err != nil {
		return model.Account{}, fmt.Errorf(
			"create account: %w",
			err,
		)
	}

	return r.toModel(
		ctx,
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

func (r *Repository) Get(
	ctx context.Context,
	accountID int64,
) (model.Account, error) {
	if accountID <= 0 {
		return model.Account{}, errors.New(
			"account id must be greater than zero",
		)
	}

	if r == nil || r.pool == nil {
		return model.Account{}, errors.New(
			"account repository: database is not configured",
		)
	}

	row, err := sqlcgen.New(r.pool).GetAccount(ctx, accountID)
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
	accountID int64,
	from,
	status model.Status,
) error {
	if accountID <= 0 {
		return errors.New(
			"account id must be greater than zero",
		)
	}

	if r == nil || r.pool == nil {
		return errors.New(
			"account repository: database is not configured",
		)
	}

	tag, err := r.pool.Exec(
		ctx,
		`UPDATE accounts
		 SET account_status_code = $3
		 WHERE account_id = $1
		   AND account_status_code = $2`,
		accountID,
		string(from),
		string(status),
	)
	if err != nil {
		return fmt.Errorf(
			"update account status: %w",
			err,
		)
	}

	if tag.RowsAffected() != 1 {
		return fmt.Errorf(
			"update account status: %w",
			model.ErrInvalidStateTransition,
		)
	}

	return nil
}

func (r *Repository) AddPaymentMethod(
	ctx context.Context,
	accountID int64,
	reference string,
) error {
	if accountID <= 0 || reference == "" {
		return errors.New(
			"account id and payment method reference are required",
		)
	}

	if r == nil || r.pool == nil {
		return errors.New(
			"account repository: database is not configured",
		)
	}

	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var status string

		err := tx.QueryRow(
			ctx,
			`SELECT account_status_code
			 FROM accounts
			 WHERE account_id = $1
			 FOR UPDATE`,
			accountID,
		).Scan(&status)

		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf(
				"lock account: %w",
				err,
			)
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

		if _, err := tx.Exec(
			ctx,
			`UPDATE payment_methods
			 SET is_default = FALSE
			 WHERE account_id = $1
			   AND is_default`,
			accountID,
		); err != nil {
			return fmt.Errorf(
				"clear default payment method: %w",
				err,
			)
		}

		if err := sqlcgen.New(tx).CreatePaymentMethod(
			ctx,
			sqlcgen.CreatePaymentMethodParams{
				AccountID:         accountID,
				ProviderReference: reference,
			},
		); err != nil {
			return fmt.Errorf(
				"create payment method: %w",
				err,
			)
		}

		return nil
	})
}

func (r *Repository) getWith(
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

	return r.toModelWith(
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

func (r *Repository) toModelWith(
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

func (r *Repository) toModel(
	ctx context.Context,
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
	methods, err := sqlcgen.New(r.pool).ListPaymentMethodReferences(
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
		v := dunning.Int64
		dunningID = &v
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
