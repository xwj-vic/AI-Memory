package store

import (
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// StagingStore 暂存区存储（基于Redis Hash）
type StagingStore struct {
	client *redis.Client
	ttl    time.Duration // 暂存区TTL
}

// NewStagingStore 创建暂存区存储实例
func NewStagingStore(client *redis.Client, ttlDays int) *StagingStore {
	return &StagingStore{
		client: client,
		ttl:    time.Hour * 24 * time.Duration(ttlDays),
	}
}

// AddOrIncrement 添加或更新暂存区条目（频次+1）
func (s *StagingStore) AddOrIncrement(ctx context.Context, userID, content string, judgeResult *types.JudgeResult) error {
	// 使用content hash作为key（简化版，实际可用MD5）
	entryID := fmt.Sprintf("staging:%s:%s", userID, hash(content))

	// 检查是否已存在
	exists, err := s.client.Exists(ctx, entryID).Result()
	if err != nil {
		return fmt.Errorf("检查暂存区条目失败: %w", err)
	}

	var entry types.StagingEntry
	now := time.Now()

	if exists > 0 {
		// 更新现有条目
		data, err := s.client.Get(ctx, entryID).Result()
		if err != nil {
			return fmt.Errorf("读取暂存区条目失败: %w", err)
		}

		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			return fmt.Errorf("解析暂存区条目失败: %w", err)
		}

		// 频次+1
		entry.OccurrenceCount++
		entry.LastSeenAt = now

		// 更新分数（取最新判定结果）
		entry.ValueScore = judgeResult.ValueScore
		entry.ConfidenceScore = judgeResult.ConfidenceScore
		entry.Category = judgeResult.Category
		entry.ExtractedTags = judgeResult.Tags
		entry.ExtractedEntities = judgeResult.Entities
	} else {
		// 创建新条目
		entry = types.StagingEntry{
			ID:                entryID,
			Content:           content,
			UserID:            userID,
			FirstSeenAt:       now,
			LastSeenAt:        now,
			OccurrenceCount:   1,
			ValueScore:        judgeResult.ValueScore,
			ConfidenceScore:   judgeResult.ConfidenceScore,
			Category:          judgeResult.Category,
			ExtractedTags:     judgeResult.Tags,
			ExtractedEntities: judgeResult.Entities,
			Status:            types.StagingPending,
		}
	}

	// 序列化并存储
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("序列化暂存区条目失败: %w", err)
	}

	if err := s.client.Set(ctx, entryID, data, s.ttl).Err(); err != nil {
		return fmt.Errorf("写入暂存区失败: %w", err)
	}

	return nil
}

// GetPendingEntries 获取待晋升的暂存区条目
func (s *StagingStore) GetPendingEntries(ctx context.Context, minOccurrences int, minWaitHours int) ([]*types.StagingEntry, error) {
	// 扫描所有staging keys
	var cursor uint64
	var entries []*types.StagingEntry
	pattern := "staging:*"

	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描暂存区失败: %w", err)
		}

		for _, key := range keys {
			data, err := s.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var entry types.StagingEntry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				continue
			}

			// 筛选条件
			if entry.Status != types.StagingPending {
				continue
			}
			if entry.OccurrenceCount < minOccurrences {
				continue
			}

			waitHours := time.Since(entry.FirstSeenAt).Hours()
			if waitHours < float64(minWaitHours) {
				continue
			}

			entries = append(entries, &entry)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return entries, nil
}

// GetAllByUser 获取用户的所有暂存区条目（用于Admin界面）
func (s *StagingStore) GetAllByUser(ctx context.Context, userID string) ([]*types.StagingEntry, error) {
	pattern := fmt.Sprintf("staging:%s:*", userID)
	var cursor uint64
	var entries []*types.StagingEntry

	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			data, err := s.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var entry types.StagingEntry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				continue
			}

			entries = append(entries, &entry)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return entries, nil
}

// Update 更新暂存区条目状态
func (s *StagingStore) Update(ctx context.Context, entry *types.StagingEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, entry.ID, data, s.ttl).Err()
}

// Delete 删除暂存区条目
func (s *StagingStore) Delete(ctx context.Context, entryID string) error {
	return s.client.Del(ctx, entryID).Err()
}

// DeleteBatch 批量删除
func (s *StagingStore) DeleteBatch(ctx context.Context, entryIDs []string) error {
	if len(entryIDs) == 0 {
		return nil
	}
	return s.client.Del(ctx, entryIDs...).Err()
}

// hash 简单哈希函数（实际生产应使用MD5或更健壮的方法）
func hash(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToLower(s)
	if len(s) > 50 {
		s = s[:50]
	}
	return strconv.FormatUint(uint64(len(s))*997+uint64(s[0])*31, 36)
}
