package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type DBTX = ppi.DBTX

type PostgresRepository struct{}

func NewPostgresRepository() *PostgresRepository {
	return &PostgresRepository{}
}

func (r *PostgresRepository) getQuerier(db DBTX) *sqlcgen.Queries {
	return sqlcgen.New(db)
}

func (r *PostgresRepository) SaveWebhook(ctx context.Context, db DBTX, payload ppi.ProviderWebhookPayload) error {
	q := r.getQuerier(db)
	_, err := q.SavePPIWebhook(ctx, sqlcgen.SavePPIWebhookParams{
		WebhookID:    payload.WebhookID,
		ProviderCode: payload.ProviderCode,
		EventType:    payload.EventType,
		InternalTxID: pgtype.Text{String: payload.InternalTxID, Valid: payload.InternalTxID != ""},
		ProviderTxID: pgtype.Text{String: payload.ProviderTxID, Valid: payload.ProviderTxID != ""},
		Status:       string(payload.Status),
		Payload:      payload.RawPayload,
		ProcessedAt:  payload.OccurredAt,
	})
	return err
}

func (r *PostgresRepository) GetWebhookByID(ctx context.Context, db DBTX, webhookID string) (ppi.ProviderWebhookPayload, error) {
	q := r.getQuerier(db)
	row, err := q.GetPPIWebhookByID(ctx, webhookID)
	if err != nil {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("get ppi webhook by id failed: %w", err)
	}

	return ppi.ProviderWebhookPayload{
		WebhookID:    row.WebhookID,
		ProviderCode: row.ProviderCode,
		EventType:    row.EventType,
		InternalTxID: row.InternalTxID.String,
		ProviderTxID: row.ProviderTxID.String,
		Status:       ppi.WebhookPaymentStatus(row.Status),
		RawPayload:   row.Payload,
		OccurredAt:   row.ProcessedAt,
	}, nil
}

func (r *PostgresRepository) GetWebhookPublishedStatus(
	ctx context.Context,
	db DBTX,
	providerCode,
	providerTxID string,
) (isPublished bool, err error) {
	if providerTxID == "" {
		return false, nil
	}
	q := r.getQuerier(db)
	published, err := q.GetPPIWebhookPublishedStatus(ctx, sqlcgen.GetPPIWebhookPublishedStatusParams{
		ProviderCode: providerCode,
		ProviderTxID: pgtype.Text{String: providerTxID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("check ppi webhook status failed: %w", err)
	}
	return published, nil
}

func (r *PostgresRepository) MarkPublished(ctx context.Context, db DBTX, webhookID string) error {
	q := r.getQuerier(db)
	if err := q.MarkPPIWebhookPublished(ctx, webhookID); err != nil {
		return fmt.Errorf("mark ppi webhook published failed: %w", err)
	}
	return nil
}

var _ ppi.WebhookRepository = (*PostgresRepository)(nil)
