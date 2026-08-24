package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

// OutboxRow is a claimed or unclaimed row from sm_action_outbox: a queued ASYNC
// action side effect awaiting execution by outbox.Publisher.
type OutboxRow struct {
	OutboxID   int64
	InstanceID uuid.UUID
	HistoryID  *int64
	BindingID  *uuid.UUID
	ActionName string
	Params     json.RawMessage
	Status     model.OutboxStatus
	Attempts   int
	ClaimedAt  *time.Time
	ClaimedBy  string
	LastError  string
	CreatedAt  time.Time
}

func (r *PostgresRepository) InsertOutboxRow(
	ctx context.Context, db DBTX,
	instanceID uuid.UUID, historyID *int64, bindingID *uuid.UUID, actionName string, params json.RawMessage,
) error {
	q := r.getQuerier(db)

	var hID pgtype.Int8
	if historyID != nil {
		hID = pgtype.Int8{Int64: *historyID, Valid: true}
	}

	err := q.InsertOutboxRow(ctx, sqlcgen.InsertOutboxRowParams{
		InstanceID: instanceID,
		HistoryID:  hID,
		BindingID:  bindingID,
		ActionName: actionName,
		Params:     nonEmptyJSON(params),
	})
	if err != nil {
		return fmt.Errorf("insert outbox row: %w", err)
	}
	return nil
}

// ClaimOutboxRows atomically claims up to limit PENDING rows via a single
// UPDATE ... FOR UPDATE SKIP LOCKED ... RETURNING statement (see engine package
// docs for why this must not be a separate SELECT-then-UPDATE across a held
// transaction). Claimed rows move to PROCESSING with claimed_at/claimed_by set and
// attempts incremented; the caller executes the action outside any DB transaction
// and then calls MarkOutboxPublished/Retry/Failed.
func (r *PostgresRepository) ClaimOutboxRows(ctx context.Context, db DBTX, limit int, workerID string) ([]OutboxRow, error) {
	q := r.getQuerier(db)

	rows, err := q.ClaimOutboxRows(ctx, sqlcgen.ClaimOutboxRowsParams{
		Limit:     int32(limit),
		ClaimedBy: textOrNil(workerID),
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox rows: %w", err)
	}

	result := make([]OutboxRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapOutboxRow(row))
	}
	return result, nil
}

func (r *PostgresRepository) MarkOutboxPublished(ctx context.Context, db DBTX, outboxID int64) error {
	q := r.getQuerier(db)
	if err := q.MarkOutboxPublished(ctx, outboxID); err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

// MarkOutboxRetry returns a PROCESSING row to PENDING for another attempt.
// attempts was already incremented at claim time.
func (r *PostgresRepository) MarkOutboxRetry(ctx context.Context, db DBTX, outboxID int64, lastErr string) error {
	q := r.getQuerier(db)
	err := q.MarkOutboxRetry(ctx, sqlcgen.MarkOutboxRetryParams{OutboxID: outboxID, LastError: textOrNil(lastErr)})
	if err != nil {
		return fmt.Errorf("mark outbox retry: %w", err)
	}
	return nil
}

func (r *PostgresRepository) MarkOutboxFailed(ctx context.Context, db DBTX, outboxID int64, lastErr string) error {
	q := r.getQuerier(db)
	err := q.MarkOutboxFailed(ctx, sqlcgen.MarkOutboxFailedParams{OutboxID: outboxID, LastError: textOrNil(lastErr)})
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

// ReapStuckOutbox reclaims PROCESSING rows whose claim is older than the cutoff
// (a worker crashed after claiming but before writing a terminal status), moving
// them back to PENDING for another worker to pick up.
func (r *PostgresRepository) ReapStuckOutbox(ctx context.Context, db DBTX, claimedBefore time.Time) (int64, error) {
	q := r.getQuerier(db)
	n, err := q.ReapStuckOutbox(ctx, pgtype.Timestamptz{Time: claimedBefore, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("reap stuck outbox: %w", err)
	}
	return n, nil
}

func mapOutboxRow(row sqlcgen.SmActionOutbox) OutboxRow {
	var historyID *int64
	if row.HistoryID.Valid {
		v := row.HistoryID.Int64
		historyID = &v
	}
	var claimedAt *time.Time
	if row.ClaimedAt.Valid {
		v := row.ClaimedAt.Time
		claimedAt = &v
	}
	return OutboxRow{
		OutboxID:   row.OutboxID,
		InstanceID: row.InstanceID,
		HistoryID:  historyID,
		BindingID:  row.BindingID,
		ActionName: row.ActionName,
		Params:     row.Params,
		Status:     model.OutboxStatus(row.Status),
		Attempts:   int(row.Attempts),
		ClaimedAt:  claimedAt,
		ClaimedBy:  textValue(row.ClaimedBy),
		LastError:  textValue(row.LastError),
		CreatedAt:  row.CreatedAt,
	}
}
