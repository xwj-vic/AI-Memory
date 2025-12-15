package memory

import (
	"ai-memory/pkg/logger"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ========== 漏斗流程核心方法 ==========

// JudgeAndStageFromSTM 从STM判定并添加到Staging
// 这个方法在Add()后可以调用，批量处理STM中的新记忆
func (m *Manager) JudgeAndStageFromSTM(ctx context.Context, userID, sessionID string) error {
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)

	// 获取STM数据
	stmData, err := m.stmStore.LRange(ctx, key, 0, -1)
	if err != nil {
		return fmt.Errorf("获取STM失败: %w", err)
	}

	if len(stmData) == 0 {
		return nil
	}

	// 批量判定（每批最多10条）
	batchSize := m.cfg.STMBatchJudgeSize
	for i := 0; i < len(stmData); i += batchSize {
		end := i + batchSize
		if end > len(stmData) {
			end = len(stmData)
		}

		batch := stmData[i:end]
		contents := make([]string, len(batch))
		for j, data := range batch {
			var rec types.Record
			if err := json.Unmarshal([]byte(data), &rec); err == nil {
				contents[j] = rec.Content
			}
		}

		// 调用判定模型
		results, err := m.judge.JudgeBatch(ctx, contents)
		if err != nil {
			logger.Error("批量判定失败", err)
			continue
		}

		// 添加到Staging
		for j, result := range results {
			if result.ShouldStage && result.ValueScore >= m.cfg.StagingValueThreshold {
				if err := m.stagingStore.AddOrIncrement(ctx, userID, contents[j], result); err != nil {
					logger.Error("添加到暂存区失败", err)
				}
			}
		}
	}

	return nil
}

// PromoteStagingToLTM 晋升Staging中的记忆到LTM
// 后台调度器会定期调用此方法
func (m *Manager) PromoteStagingToLTM(ctx context.Context) error {
	// 获取待晋升条目
	entries, err := m.stagingStore.GetPendingEntries(
		ctx,
		m.cfg.StagingMinOccurrences,
		m.cfg.StagingMinWaitHours,
	)
	if err != nil {
		return fmt.Errorf("获取待晋升条目失败: %w", err)
	}

	for _, entry := range entries {
		// 判断信心水平
		if entry.ConfidenceScore >= m.cfg.StagingConfidenceHigh {
			// 高信心：自动晋升
			if err := m.promoteSingleEntry(ctx, entry, "auto"); err != nil {
				logger.Error("自动晋升失败", err)
			}
		} else if entry.ConfidenceScore >= m.cfg.StagingConfidenceLow {
			// 中等信心：需要用户确认（暂时跳过，等待Admin界面确认）
			logger.MemoryCheck("pending_review", 1, fmt.Sprintf("score: %.2f, content: %s", entry.ConfidenceScore, entry.Content[:50]))
			// TODO: 触发用户确认机制(WebSocket/Admin Dashboard)
		} else {
			// 低信心：直接删除
			m.stagingStore.Delete(ctx, entry.ID)
		}
	}

	return nil
}

// promoteSingleEntry 晋升单条记忆到LTM
func (m *Manager) promoteSingleEntry(ctx context.Context, entry *types.StagingEntry, confirmedBy string) error {
	// 1. 提取结构化标签（使用更强大的模型）
	tags, entities, err := m.judge.ExtractStructuredTags(ctx, entry.Content, entry.Category)
	if err != nil {
		// 降级使用预提取的标签
		tags = entry.ExtractedTags
		entities = entry.ExtractedEntities
	}

	// 2. 生成Embedding
	vector, err := m.embedder.EmbedQuery(ctx, entry.Content)
	if err != nil {
		return fmt.Errorf("生成embedding失败: %w", err)
	}

	// 3. 构建LTM记录
	now := time.Now()
	metadata := types.LTMMetadata{
		UserID:           entry.UserID,
		CreatedAt:        now,
		Tags:             tags,
		Entities:         entities,
		Category:         entry.Category,
		LastAccessAt:     now,
		AccessCount:      0,
		DecayScore:       1.0, // 初始分数
		SourceType:       "staging",
		ConfidenceOrigin: entry.ConfidenceScore,
	}

	metadataMap := map[string]interface{}{
		"user_id":           metadata.UserID,
		"created_at":        metadata.CreatedAt,
		"tags":              metadata.Tags,
		"entities":          metadata.Entities,
		"category":          string(metadata.Category),
		"last_access_at":    metadata.LastAccessAt,
		"access_count":      metadata.AccessCount,
		"decay_score":       metadata.DecayScore,
		"source_type":       metadata.SourceType,
		"confidence_origin": metadata.ConfidenceOrigin,
	}

	ltmRecord := types.Record{
		ID:        uuid.New().String(),
		Content:   entry.Content,
		Embedding: vector,
		Timestamp: entry.LastSeenAt,
		Metadata:  metadataMap, // Keep metadataMap for compatibility with types.Record.Metadata
		Type:      types.LongTerm,
	}

	// 4. 写入LTM
	if err := m.vectorStore.Add(ctx, []types.Record{ltmRecord}); err != nil {
		return fmt.Errorf("写入LTM失败: %w", err)
	}

	// 5. 删除Staging条目
	if err := m.stagingStore.Delete(ctx, entry.ID); err != nil {
		logger.Error("删除暂存区条目失败", err)
	}

	logger.MemoryPromotion(string(entry.Category), confirmedBy, entry.ConfidenceScore, entry.Content)
	return nil
}

// ScanAndEvictDecayedMemories 扫描并删除衰减的记忆
func (m *Manager) ScanAndEvictDecayedMemories(ctx context.Context) error {
	// 获取所有LTM记录
	allMemories, err := m.vectorStore.List(ctx, map[string]interface{}{}, 1000, 0)
	if err != nil {
		return fmt.Errorf("获取LTM记录失败: %w", err)
	}

	var toDelete []string
	var toUpdate []types.Record

	for _, record := range allMemories {
		// 提取metadata
		metadata, err := extractLTMMetadata(record.Metadata)
		if err != nil {
			continue
		}

		// 计算衰减分数
		m.decayCalculator.UpdateMetadataDecay(metadata)

		if m.decayCalculator.ShouldEvict(metadata.DecayScore) {
			// 标记删除
			toDelete = append(toDelete, record.ID)
			logger.System("🗑️ Evicting Memory", "decay", metadata.DecayScore, "content", record.Content[:50])
		} else {
			// 更新衰减分数
			record.Metadata["decay_score"] = metadata.DecayScore
			record.Metadata["last_access_at"] = metadata.LastAccessAt
			toUpdate = append(toUpdate, record)
		}
	}

	// 批量删除
	if len(toDelete) > 0 {
		if err := m.vectorStore.Delete(ctx, toDelete); err != nil {
			logger.Error("批量删除失败", err)
		}
	}

	// 批量更新
	for _, rec := range toUpdate {
		if err := m.vectorStore.Update(ctx, rec); err != nil {
			logger.Error("更新记忆失败", err)
		}
	}

	logger.System("Decay Scan Completed", "deleted", len(toDelete), "updated", len(toUpdate))
	return nil
}

// extractLTMMetadata 从Record.Metadata提取LTMMetadata
func extractLTMMetadata(metaMap map[string]interface{}) (*types.LTMMetadata, error) {
	metadata := &types.LTMMetadata{}

	if v, ok := metaMap["user_id"].(string); ok {
		metadata.UserID = v
	}
	if v, ok := metaMap["last_access_at"].(time.Time); ok {
		metadata.LastAccessAt = v
	} else {
		metadata.LastAccessAt = time.Now().Add(-time.Hour * 24 * 30) // 默认30天前
	}
	if v, ok := metaMap["access_count"].(int); ok {
		metadata.AccessCount = v
	}
	if v, ok := metaMap["decay_score"].(float64); ok {
		metadata.DecayScore = v
	} else {
		metadata.DecayScore = 1.0
	}

	return metadata, nil
}

// ========== 后台调度器 ==========

// startBackgroundTasks 启动后台协程
func (m *Manager) startBackgroundTasks() {
	// 任务1：定期晋升Staging记忆
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		// 修复: 确保 tickerDuration 至少为 1 小时，防止 NewTicker panic
		hours := m.cfg.StagingMinWaitHours / 2
		if hours < 1 {
			hours = 1
		}
		tickerDuration := time.Hour * time.Duration(hours)
		logger.System("Starting Staging Promotion Task", "interval_hours", hours)

		ticker := time.NewTicker(tickerDuration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.PromoteStagingToLTM(m.ctx); err != nil {
					logger.Error("Staging晋升任务失败", err)
				}
			case <-m.ctx.Done():
				return
			}
		}
	}()

	// 任务2：STM -> Staging 自动清洗 (新增)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		// 每 10 分钟自动检查一次 STM
		interval := 10 * time.Minute
		logger.System("Starting STM Autosave Task", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 方案：遍历所有 stm Key
				keys, err := m.stmStore.ScanKeys(m.ctx, "memory:stm:*:*")
				if err != nil {
					logger.Error("STM Scanner Failed", err)
					continue
				}

				processedUsers := make(map[string]bool)
				for _, key := range keys {
					// key format: memory:stm:<userID>:<sessionID>
					var userID, sessionID string
					if n, _ := fmt.Sscanf(key, "memory:stm:%s:%s", &userID, &sessionID); n == 2 {
						// 避免同一个用户重复频繁调用 (可选优化)
						if processedUsers[userID] {
							continue
						}

						if err := m.JudgeAndStageFromSTM(m.ctx, userID, sessionID); err != nil {
							logger.Error("Auto Judge Failed", err)
						} else {
							processedUsers[userID] = true
						}
					}
				}

			case <-m.ctx.Done():
				return
			}
		}
	}()

	// 任务3：定期执行遗忘机制
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Hour * 24) // 每24小时扫描一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.ScanAndEvictDecayedMemories(m.ctx); err != nil {
					logger.Error("遗忘扫描任务失败", err)
				}
			case <-m.ctx.Done():
				return
			}
		}
	}()

	logger.System("✅ 后台调度器已启动: STM清洗 + Staging晋升 + 记忆衰减")
}

// Shutdown 优雅关闭
func (m *Manager) Shutdown() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	logger.System("Manager Shutdown")
}
