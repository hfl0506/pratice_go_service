package service

import (
	"aws-prj/internal/repository"
	sqlcq "aws-prj/pgsql"
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repository *repository.Repository
	rdc        *redis.Client
}

func Init(repository *repository.Repository, rdc *redis.Client) *Service {
	return &Service{
		repository,
		rdc,
	}
}

func (s *Service) CreateTask(ctx context.Context, context string) (sqlcq.Task, error) {
	return s.repository.CreateTask(ctx, context)
}

func (s *Service) ListTasks(ctx context.Context, offset, limit int32) ([]sqlcq.Task, error) {
	return s.repository.ListTask(ctx, offset, limit)
}

func (s *Service) GetTaskById(ctx context.Context, id pgtype.UUID) (sqlcq.Task, error) {
	return s.repository.GetTaskById(ctx, id)
}

func (s *Service) UpdateTaskById(ctx context.Context, id pgtype.UUID, context string) (sqlcq.Task, error) {
	return s.repository.UpdateTaskById(ctx, id, context)
}

func (s *Service) DeleteTaskById(ctx context.Context, id pgtype.UUID) error {
	return s.repository.DeleteTaskById(ctx, id)
}
