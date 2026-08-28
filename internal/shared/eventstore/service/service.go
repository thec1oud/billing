package service

import (
	"context"
	"fmt"

	"github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type Repository interface {
	Append(ctx context.Context, tx sqlcgen.DBTX, req model.AppendRequest) (model.Event, error)
	ReadStream(ctx context.Context, aggType model.AggregateType, aggID string) ([]model.Event, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) AppendEvent(ctx context.Context, tx sqlcgen.DBTX, req model.AppendRequest) (model.Event, error) {
	evt, err := s.repo.Append(ctx, tx, req)
	if err != nil {
		return model.Event{}, fmt.Errorf("eventstore append: %w", err)
	}
	return evt, nil
}

func (s *Service) ReadEvent(ctx context.Context, aggType model.AggregateType, aggID string) ([]model.Event, error) {
	return s.repo.ReadStream(ctx, aggType, aggID)
}
