package redisclient

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

func Init(ctx context.Context, dsn string, log *slog.Logger) (*redis.Client, error) {
	opt, err := redis.ParseURL(dsn)

	if err != nil {
		log.Error("init redis failed", "error", err)
		return nil, err
	}

	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		log.Error("redis ping failed", "error", err)
		return nil, err
	}

	log.Info("connected to redis...")

	return client, nil
}
