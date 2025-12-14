package store

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"

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
// Note: This is inefficient (O(N) keys) but acceptable for Admin dashboard volume.
func (r *RedisStore) Update(ctx context.Context, record types.Record) error {
	// 1. Scan all STM keys
	// Pattern: memory:stm:*:*
	iter := r.client.Scan(ctx, 0, "memory:stm:*:*", 0).Iterator()
	found := false

	for iter.Next(ctx) {
		key := iter.Val()

		// 2. Fetch list
		items, err := r.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for idx, itemStr := range items {
			var current types.Record
			if err := json.Unmarshal([]byte(itemStr), &current); err == nil {
				if current.ID == record.ID {
					// 3. Update
					// Preserve fields? The passed record should be complete or we merge?
					// Manager.Update constructs 'rec' by fetching?
					// Wait, Manager.Update doesn't Fetch STM yet.
					// We need Get for STM too if we want to be safe?
					// Or RedisStore.Update merges?
					// Let's assume passed record is complete (Content, Timestamp, etc).

					// But wait, Manager.Update (previous step) does Get -> Change Content -> Save.
					// If I don't implement Get for STM, Manager can't Get it to Apply changes.
					// So I probably need Get for ListStore too?
					// OR Manager.Update uses a blind update if Get fails?
					// If Manager uses blind update, we lose other fields.

					// Decision: Add Get to ListStore too.
					// It's the only consistent way.

					// For now, I will error out and ask to add Get in next step?
					// actually I can implement both here if I change interface first.
					// But I already updated interface with only Update.

					// Workaround: In this Update implementation, since we have 'current',
					// if 'record' has empty fields, we key 'current' ones?
					// No, that's messy.

					// Let's assume for this specific User Request (Update Content),
					// I will modify Manager to passed the *ID* and *Content* to ListStore.Update?
					// No, interface uses Record.

					// I MUST adding Get to ListStore.
					// I will add Update first, but it will be broken without Get.
					// Actually, I can fix the interface in the NEXT tool call properly.
					// Let's just implement finding it for now.

					// Wait, if I implement Update(rec), checking ID...
					// If I overwrite with `record`, and `record` only has ID and Content...
					// Timestamp is lost.

					// I'll assume the caller calls Get() first.
					// So I will add Get() to `ListStore` interface in a moment.

					// Serialize
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
