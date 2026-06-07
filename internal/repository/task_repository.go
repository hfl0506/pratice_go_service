package repository

import (
	sqlcq "aws-prj/pgsql"
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct {
	queries *sqlcq.Queries
}

func Init(queries *sqlcq.Queries) *Repository {
	return &Repository{
		queries,
	}
}

func (r *Repository) ListTask(ctx context.Context, offset, limit int32) ([]sqlcq.Task, error) {
	return r.queries.ListTasks(ctx, sqlcq.ListTasksParams{
		Offset: offset,
		Limit:  limit,
	})
}

func (r *Repository) GetTaskById(ctx context.Context, id pgtype.UUID) (sqlcq.Task, error) {
	return r.queries.GetTaskById(ctx, id)
}

func (r *Repository) CreateTask(ctx context.Context, context string) (sqlcq.Task, error) {
	return r.queries.CreateTask(ctx, context)
}

func (r *Repository) UpdateTaskById(ctx context.Context, id pgtype.UUID, context string) (sqlcq.Task, error) {
	return r.queries.UpdateTaskById(ctx, sqlcq.UpdateTaskByIdParams{
		ID:      id,
		Context: context,
	})
}

func (r *Repository) DeleteTaskById(ctx context.Context, id pgtype.UUID) error {
	return r.queries.DeleteTaskById(ctx, id)
}
