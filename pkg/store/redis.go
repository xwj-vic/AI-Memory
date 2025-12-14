package store

import (
	"ai-memory/pkg/config"
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(cfg *config.Config) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	return &RedisStore{
		client: rdb,
	}
}

// RPush appends values to a list.
func (r *RedisStore) RPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.RPush(ctx, key, values...).Err()
}

// LRange retrieves a range of elements from a list.
func (r *RedisStore) LRange(ctx context.Context, key string, start, stop int) ([]string, error) {
	return r.client.LRange(ctx, key, int64(start), int64(stop)).Result()
}

// Del removes keys.
func (r *RedisStore) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Ping checks connection.
func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
