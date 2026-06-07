package db

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(ctx context.Context, dsn string, log *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)

	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		log.Error("database ping failed", "error", err)
		return nil, err
	}

	log.Info("connected to db...")

	return pool, nil
}
