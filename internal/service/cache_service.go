package service

import (
	"context"
	"time"
)

func (s *Service) CacheTask(ctx context.Context, id, val string) error {
	return s.rdc.Set(ctx, id, val, 5*time.Minute).Err()
}

func (s *Service) DecacheTask(ctx context.Context, id string) error {
	return s.rdc.Del(ctx, id).Err()
}

func (s *Service) RetrieveCacheTask(ctx context.Context, id string) (string, error) {
	return s.rdc.Get(ctx, id).Result()
}

func (s *Service) Ping(ctx context.Context) error {
	return s.rdc.Ping(ctx).Err()
}
