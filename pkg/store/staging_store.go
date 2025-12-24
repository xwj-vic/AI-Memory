package store

import (
	"ai-memory/pkg/types"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
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
// 【需求3.1】集成语义去重：使用向量相似度检测
func (s *StagingStore) AddOrIncrement(ctx context.Context, userID, sessionID, content string, judgeResult *types.JudgeResult, embedder Embedder) error {
	// 1. 生成embedding（用于语义去重）
	var embedding []float32
	var err error
	if embedder != nil {
		embedding, err = embedder.EmbedQuery(ctx, content)
		if err != nil {
			// 降级：embedding失败不影响主流程
			embedding = nil
		}
	}

	// 2. 语义去重：搜索相似的已有条目
	if embedding != nil {
		similarEntry, _ := s.SearchSimilar(ctx, userID, embedding, 0.95)
		if similarEntry != nil {
			// 找到相似条目，增加计数
			similarEntry.OccurrenceCount++
			similarEntry.LastSeenAt = time.Now()
			similarEntry.ValueScore = judgeResult.ValueScore
			similarEntry.ConfidenceScore = judgeResult.ConfidenceScore
			similarEntry.Category = judgeResult.Category
			similarEntry.ExtractedTags = judgeResult.Tags
			similarEntry.ExtractedEntities = judgeResult.Entities

			// 记录 SessionID (去重)
			foundSession := false
			for _, sid := range similarEntry.SessionIDs {
				if sid == sessionID {
					foundSession = true
					break
				}
			}
			if !foundSession && sessionID != "" {
				similarEntry.SessionIDs = append(similarEntry.SessionIDs, sessionID)
			}

			// 更新
			data, _ := json.Marshal(similarEntry)
			s.client.Set(ctx, similarEntry.ID, data, s.ttl)
			return nil
		}
	}

	// 3. 无相似条目或embedding失败，使用原有逻辑（hash去重）
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

		// 记录 SessionID (去重)
		foundSession := false
		for _, sid := range entry.SessionIDs {
			if sid == sessionID {
				foundSession = true
				break
			}
		}
		if !foundSession && sessionID != "" {
			entry.SessionIDs = append(entry.SessionIDs, sessionID)
		}
	} else {
		// 创建新条目
		entry = types.StagingEntry{
			ID:                entryID,
			Content:           content,
			Embedding:         embedding, // 存储embedding
			UserID:            userID,
			SessionIDs:        []string{sessionID},
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

// SearchSimilar 在Staging中搜索语义相似的条目
// 参数：
//   - userID: 用户ID（只在该用户的Staging中搜索）
//   - queryVector: 查询向量
//   - threshold: 相似度阈值（0.95表示95%相似）
//
// 返回：最相似的条目（如无则返回nil）
func (s *StagingStore) SearchSimilar(ctx context.Context, userID string, queryVector []float32, threshold float64) (*types.StagingEntry, error) {
	pattern := fmt.Sprintf("staging:%s:*", userID)
	var cursor uint64
	var bestEntry *types.StagingEntry
	var bestSimilarity float64

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

			// 跳过没有embedding的条目
			if len(entry.Embedding) == 0 {
				continue
			}

			// 计算余弦相似度
			similarity := cosineSimilarity(queryVector, entry.Embedding)

			if similarity > threshold && similarity > bestSimilarity {
				bestSimilarity = similarity
				bestEntry = &entry
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return bestEntry, nil
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

// GetBySession 获取该会话触达过的暂存区条目 (Session 隔离)
func (s *StagingStore) GetBySession(ctx context.Context, userID, sessionID string) ([]*types.StagingEntry, error) {
	allEntries, err := s.GetAllByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var sessionEntries []*types.StagingEntry
	for _, entry := range allEntries {
		for _, sid := range entry.SessionIDs {
			if sid == sessionID {
				sessionEntries = append(sessionEntries, entry)
				break
			}
		}
	}

	return sessionEntries, nil
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

// hash 使用 MD5 生成唯一哈希值
func hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))[:16] // 取前16位，足够唯一且简洁
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Embedder 接口定义
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}
