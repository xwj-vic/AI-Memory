package store

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// GetClient 返回底层Redis客户端(用于StagingStore)
func (r *RedisStore) GetClient() *redis.Client {
	return r.client
}

// RPush appends values to a list.
func (r *RedisStore) RPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.RPush(ctx, key, values...).Err()
}

// RPushWithExpire appends values to a list and sets expiration.
func (r *RedisStore) RPushWithExpire(ctx context.Context, key string, expirationDays int, values ...interface{}) error {
	// 如果expirationDays为0，则不设置过期时间
	if expirationDays <= 0 {
		return r.RPush(ctx, key, values...)
	}

	// 使用Pipeline确保原子性
	pipe := r.client.Pipeline()
	pipe.RPush(ctx, key, values...)
	pipe.Expire(ctx, key, time.Duration(expirationDays)*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
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

// ScanKeys finds keys matching a pattern.
func (r *RedisStore) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

// Update searches all STM lists for the record and updates it.
func (r *RedisStore) Update(ctx context.Context, record types.Record) error {
	iter := r.client.Scan(ctx, 0, "memory:stm:*:*", 0).Iterator()
	found := false

	for iter.Next(ctx) {
		key := iter.Val()
		items, err := r.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for idx, itemStr := range items {
			var current types.Record
			if err := json.Unmarshal([]byte(itemStr), &current); err == nil {
				if current.ID == record.ID {
					enc, _ := json.Marshal(record)
					r.client.LSet(ctx, key, int64(idx), enc)
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("record not found in stm")
	}
	return nil
}

// Get finds a record by ID in STM.
func (r *RedisStore) Get(ctx context.Context, id string) (*types.Record, error) {
	iter := r.client.Scan(ctx, 0, "memory:stm:*:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		items, err := r.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}
		for _, itemStr := range items {
			var current types.Record
			if err := json.Unmarshal([]byte(itemStr), &current); err == nil {
				if current.ID == id {
					return &current, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("record not found")
}

// SIsMember checks if a member exists in a set.
func (r *RedisStore) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// SAdd adds members to a set.
func (r *RedisStore) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// Expire sets a timeout on a key.
func (r *RedisStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}
