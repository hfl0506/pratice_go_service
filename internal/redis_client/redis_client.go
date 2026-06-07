package redisclient

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func Init(ctx context.Context, dsn string) (*redis.Client, error) {
	opt, err := redis.ParseURL(dsn)

	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	if cmd := client.Ping(ctx); cmd.Err() != nil {
		return nil, cmd.Err()
	}

	log.Println("connected to redis...")

	return client, nil
}
