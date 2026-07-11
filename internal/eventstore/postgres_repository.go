package eventstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
)

type PostgresRepository struct {
	db *pgx.Conn
}

func NewPostgresRepository(db *pgx.Conn) Repository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) Append(
	ctx context.Context,
	event *sharedEvents.Event,
) error {
	if event.Sequence < 1 {
		return ErrInvalidSequence
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	// Serialize writes to one aggregate. The database uniqueness constraint is a
	// second guard, while this lock ensures a sequence is checked against a stable
	// stream even when two writers append concurrently.
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, string(event.AggregateType)+":"+event.AggregateID.String())
	if err != nil {
		return err
	}

	var currentSequence int64

	err = tx.QueryRow(
		ctx,
		`
		SELECT COALESCE(MAX(sequence),0)
		FROM events
		WHERE aggregate_type=$1
		AND aggregate_id=$2
		`,
		event.AggregateType,
		event.AggregateID,
	).Scan(&currentSequence)

	if err != nil {
		return err
	}

	expected := currentSequence + 1

	if event.Sequence != expected {
		return ErrSequenceConflict
	}

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO events (

		    event_id,
		    event_type,
		    event_version,

		    aggregate_type,
		    aggregate_id,

		    sequence,

		    actor,

		    payload,

		    occurred_at,

		    causation_id,
		    correlation_id

		)

		VALUES(

		    $1,$2,$3,
		    $4,$5,
		    $6,
		    $7,
		    $8,
		    $9,
		    $10,
		    $11
		)
		`,
		event.EventID,
		event.EventType,
		event.EventVersion,
		event.AggregateType,
		event.AggregateID,
		event.Sequence,
		event.Actor,
		event.Payload,
		event.OccurredAt,
		event.CausationID,
		event.CorrelationID,
	)

	if err != nil {

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSequenceConflict
		}

		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAppendFailed
		}

		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) ReadStream(
	ctx context.Context,
	aggregateType sharedEvents.AggregateType,
	aggregateID uuid.UUID,
) ([]sharedEvents.Event, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT

		    event_id,
		    event_type,
		    event_version,

		    aggregate_type,
		    aggregate_id,

		    sequence,

		    actor,

		    payload,

		    occurred_at,

		    causation_id,
		    correlation_id

		FROM events

		WHERE aggregate_type=$1

		AND aggregate_id=$2

		ORDER BY sequence ASC
		`,
		aggregateType,
		aggregateID,
	)

	if err != nil {
		return nil, ErrReadFailed
	}

	defer rows.Close()

	var stream []sharedEvents.Event

	for rows.Next() {

		var event sharedEvents.Event

		err := rows.Scan(

			&event.EventID,
			&event.EventType,
			&event.EventVersion,

			&event.AggregateType,
			&event.AggregateID,

			&event.Sequence,

			&event.Actor,

			&event.Payload,

			&event.OccurredAt,

			&event.CausationID,
			&event.CorrelationID,
		)

		if err != nil {
			return nil, err
		}

		stream = append(stream, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stream, nil
}
