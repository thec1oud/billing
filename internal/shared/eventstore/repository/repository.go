package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type PostgresEventStore struct {
	pool *pgxpool.Pool
}

func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{pool: pool}
}

func (s *PostgresEventStore) Append(ctx context.Context, req model.AppendRequest) (model.Event, error) {
	if err := req.Validate(); err != nil {
		return model.Event{}, err
	}

	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return model.Event{}, fmt.Errorf("marshal event payload: %w", err)
	}

	q := sqlcgen.New(s.pool)
	row, err := q.AppendEvent(ctx, sqlcgen.AppendEventParams{
		EventType:     string(req.EventType),
		EventVersion:  int32(req.EventVersion),
		AggregateType: string(req.AggregateType),
		AggregateID:   req.AggregateID,
		Actor:         req.Actor,
		CausationID:   req.CausationID,
		CorrelationID: req.CorrelationID,
		Payload:       payloadBytes,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.Event{}, model.ErrSequenceConflict
		}
		return model.Event{}, fmt.Errorf("append event: %w", err)
	}

	return model.Event{
		EventID:       row.EventID,
		EventType:     req.EventType,
		EventVersion:  req.EventVersion,
		AggregateType: req.AggregateType,
		AggregateID:   req.AggregateID,
		Sequence:      row.Sequence,
		OccurredAt:    row.OccurredAt,
		Actor:         req.Actor,
		Payload:       payloadBytes,
		CausationID:   req.CausationID,
		CorrelationID: req.CorrelationID,
	}, nil
}

func (s *PostgresEventStore) ReadStream(ctx context.Context, aggType model.AggregateType, aggID string) ([]model.Event, error) {
	q := sqlcgen.New(s.pool)
	rows, err := q.ReadStream(ctx, sqlcgen.ReadStreamParams{
		AggregateType: string(aggType),
		AggregateID:   aggID,
	})
	if err != nil {
		return nil, fmt.Errorf("read stream (%s / %s): %w", aggType, aggID, err)
	}

	events := make([]model.Event, len(rows))
	for i, r := range rows {
		events[i] = model.Event{
			EventID:       r.EventID,
			EventType:     model.EventType(r.EventType),
			EventVersion:  int(r.EventVersion),
			AggregateType: model.AggregateType(r.AggregateType),
			AggregateID:   r.AggregateID,
			Sequence:      r.Sequence,
			OccurredAt:    r.OccurredAt,
			Actor:         r.Actor,
			Payload:       r.Payload,
			CausationID:   r.CausationID,
			CorrelationID: r.CorrelationID,
		}
	}
	return events, nil
}
