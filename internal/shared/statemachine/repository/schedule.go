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

// ScheduleRow is a claimed or unclaimed row from sm_scheduled_transitions: a
// timer-driven transition awaiting firing by scheduler.Poller.
type ScheduleRow struct {
	ScheduleID   uuid.UUID
	InstanceID   uuid.UUID
	EventName    string
	EventPayload json.RawMessage
	FireAt       time.Time
	Status       model.ScheduleStatus
	Attempts     int
	ClaimedAt    *time.Time
	ClaimedBy    string
	LastError    string
	CreatedAt    time.Time
}

func (r *PostgresRepository) InsertScheduledTransition(
	ctx context.Context, db DBTX,
	instanceID uuid.UUID, eventName string, eventPayload json.RawMessage, fireAt time.Time,
) (uuid.UUID, error) {
	q := r.getQuerier(db)

	id, err := q.InsertScheduledTransition(ctx, sqlcgen.InsertScheduledTransitionParams{
		InstanceID:   instanceID,
		EventName:    eventName,
		EventPayload: eventPayload,
		FireAt:       fireAt,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert scheduled transition: %w", err)
	}
	return id, nil
}

// ClaimScheduledTransitions atomically claims up to limit due (fire_at <= now())
// PENDING rows via a single UPDATE ... FOR UPDATE SKIP LOCKED ... RETURNING
// statement, moving them to PROCESSING. The caller fires the transition outside
// any DB transaction the claim itself used, then calls
// MarkScheduleFired/Retry/Failed.
func (r *PostgresRepository) ClaimScheduledTransitions(ctx context.Context, db DBTX, limit int, workerID string) ([]ScheduleRow, error) {
	q := r.getQuerier(db)

	rows, err := q.ClaimScheduledTransitions(ctx, sqlcgen.ClaimScheduledTransitionsParams{
		Limit:     int32(limit),
		ClaimedBy: textOrNil(workerID),
	})
	if err != nil {
		return nil, fmt.Errorf("claim scheduled transitions: %w", err)
	}

	result := make([]ScheduleRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapScheduleRow(row))
	}
	return result, nil
}

func (r *PostgresRepository) MarkScheduleFired(ctx context.Context, db DBTX, scheduleID uuid.UUID) error {
	q := r.getQuerier(db)
	if err := q.MarkScheduleFired(ctx, scheduleID); err != nil {
		return fmt.Errorf("mark schedule fired: %w", err)
	}
	return nil
}

func (r *PostgresRepository) MarkScheduleRetry(ctx context.Context, db DBTX, scheduleID uuid.UUID, lastErr string) error {
	q := r.getQuerier(db)
	err := q.MarkScheduleRetry(ctx, sqlcgen.MarkScheduleRetryParams{ScheduleID: scheduleID, LastError: textOrNil(lastErr)})
	if err != nil {
		return fmt.Errorf("mark schedule retry: %w", err)
	}
	return nil
}

func (r *PostgresRepository) MarkScheduleFailed(ctx context.Context, db DBTX, scheduleID uuid.UUID, lastErr string) error {
	q := r.getQuerier(db)
	err := q.MarkScheduleFailed(ctx, sqlcgen.MarkScheduleFailedParams{ScheduleID: scheduleID, LastError: textOrNil(lastErr)})
	if err != nil {
		return fmt.Errorf("mark schedule failed: %w", err)
	}
	return nil
}

// ReapStuckSchedules reclaims PROCESSING rows whose claim is older than the
// cutoff, moving them back to PENDING for another worker to pick up.
func (r *PostgresRepository) ReapStuckSchedules(ctx context.Context, db DBTX, claimedBefore time.Time) (int64, error) {
	q := r.getQuerier(db)
	n, err := q.ReapStuckSchedules(ctx, pgtype.Timestamptz{Time: claimedBefore, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("reap stuck schedules: %w", err)
	}
	return n, nil
}

func mapScheduleRow(row sqlcgen.SmScheduledTransition) ScheduleRow {
	var claimedAt *time.Time
	if row.ClaimedAt.Valid {
		v := row.ClaimedAt.Time
		claimedAt = &v
	}
	return ScheduleRow{
		ScheduleID:   row.ScheduleID,
		InstanceID:   row.InstanceID,
		EventName:    row.EventName,
		EventPayload: row.EventPayload,
		FireAt:       row.FireAt,
		Status:       model.ScheduleStatus(row.Status),
		Attempts:     int(row.Attempts),
		ClaimedAt:    claimedAt,
		ClaimedBy:    textValue(row.ClaimedBy),
		LastError:    textValue(row.LastError),
		CreatedAt:    row.CreatedAt,
	}
}
