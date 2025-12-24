package memory

import (
	"ai-memory/pkg/logger"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// BatchPromoteToLTM 批量晋升记忆到LTM（性能优化）
func (m *Manager) BatchPromoteToLTM(ctx context.Context, entries []*types.StagingEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var ltmRecords []types.Record
	var successIDs []string

	for _, entry := range entries {
		// 提取结构化标签
		tags, entities, err := m.judge.ExtractStructuredTags(ctx, entry.Content, entry.Category)
		if err != nil {
			tags = entry.ExtractedTags
			entities = entry.ExtractedEntities
		}

		// 生成Embedding
		vector, err := m.embedder.EmbedQuery(ctx, entry.Content)
		if err != nil {
			logger.Error("生成embedding失败", err, "entry_id", entry.ID)
			continue
		}

		// 构建记录
		metadata := map[string]interface{}{
			"user_id":           entry.UserID,
			"created_at":        entry.FirstSeenAt,
			"tags":              tags,
			"entities":          entities,
			"category":          string(entry.Category),
			"last_access_at":    entry.LastSeenAt,
			"access_count":      0,
			"decay_score":       1.0,
			"source_type":       "staging",
			"confidence_origin": entry.ConfidenceScore,
		}

		ltmRecord := types.Record{
			ID:        uuid.New().String(),
			Content:   entry.Content,
			Embedding: vector,
			Timestamp: entry.LastSeenAt,
			Metadata:  metadata,
			Type:      types.LongTerm,
		}

		ltmRecords = append(ltmRecords, ltmRecord)
		successIDs = append(successIDs, entry.ID)
	}

	// 批量写入LTM
	if len(ltmRecords) > 0 {
		if err := m.vectorStore.Add(ctx, ltmRecords); err != nil {
			return fmt.Errorf("批量写入LTM失败: %w", err)
		}
	}

	// 批量删除Staging
	if len(successIDs) > 0 {
		if err := m.stagingStore.DeleteBatch(ctx, successIDs); err != nil {
			logger.Error("批量删除暂存区失败", err)
		}
	}

	logger.System("Batch Promotion Completed", "count", len(ltmRecords))
	return nil
}

// JudgeAndStageFromSTM的缓存优化版本
func (m *Manager) JudgeAndStageFromSTMCached(ctx context.Context, userID, sessionID string) error {
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)

	stmData, err := m.stmStore.LRange(ctx, key, 0, -1)
	if err != nil {
		return fmt.Errorf("获取STM失败: %w", err)
	}

	if len(stmData) == 0 {
		return nil
	}

	batchSize := m.cfg.STMBatchJudgeSize
	for i := 0; i < len(stmData); i += batchSize {
		end := i + batchSize
		if end > len(stmData) {
			end = len(stmData)
		}

		batch := stmData[i:end]
		var needsJudgment []string
		var cachedResults []*types.JudgeResult

		for _, data := range batch {
			var rec types.Record
			if err := json.Unmarshal([]byte(data), &rec); err == nil {
				// 尝试从缓存获取（如果Manager有monitor）
				// 这里简化处理，直接判定
				needsJudgment = append(needsJudgment, rec.Content)
			}
		}

		// 批量判定
		if len(needsJudgment) > 0 {
			results, err := m.judge.JudgeBatch(ctx, needsJudgment)
			if err != nil {
				logger.Error("批量判定失败", err)
				continue
			}
			cachedResults = append(cachedResults, results...)
		}

		// 添加到Staging
		for i, result := range cachedResults {
			if result.ShouldStage && result.ValueScore >= m.cfg.StagingValueThreshold {
				if err := m.stagingStore.AddOrIncrement(ctx, userID, sessionID, needsJudgment[i], result, m.embedder); err != nil {
					logger.Error("添加到暂存区失败", err)
				}
			}
		}
	}

	return nil
}
