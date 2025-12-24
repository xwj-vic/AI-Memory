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

// JudgeAndStageFromSTM 从 STM判定并添加到Staging
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

	// 解析记录并过滤已判定的
	var toJudge []types.Record
	var recordIDs []string
	for _, data := range stmData {
		var rec types.Record
		if err := json.Unmarshal([]byte(data), &rec); err != nil {
			continue
		}

		toJudge = append(toJudge, rec)
		recordIDs = append(recordIDs, rec.ID)
	}

	if len(toJudge) == 0 {
		return nil // 所有记录都已判定
	}

	// 【触发检查】
	shouldStart := false
	if len(toJudge) >= m.cfg.STMJudgeMinMessages {
		shouldStart = true
	} else if len(toJudge) > 0 {
		// 检查第一条未判定记录的等待时间
		if time.Since(toJudge[0].Timestamp).Minutes() >= float64(m.cfg.STMJudgeMaxWaitMinutes) {
			shouldStart = true
		}
	}

	if !shouldStart {
		return nil // 未达到触发阈值
	}

	logger.System("STM判定开始", "total", len(stmData), "new", len(toJudge), "user", userID, "session", sessionID)

	// 批量判定（每批最多10条）
	batchSize := m.cfg.STMBatchJudgeSize
	for i := 0; i < len(toJudge); i += batchSize {
		end := i + batchSize
		if end > len(toJudge) {
			end = len(toJudge)
		}

		batch := toJudge[i:end]
		contents := make([]string, 0, len(batch))
		results := make([]*types.JudgeResult, len(batch))
		toLLMIndices := make([]int, 0)

		// 1. 尝试从缓存获取
		if m.monitor != nil {
			for j, rec := range batch {
				if cached, ok := m.monitor.GetJudgeResultFromCache(rec.Content); ok {
					results[j] = cached
				} else {
					contents = append(contents, rec.Content)
					toLLMIndices = append(toLLMIndices, j)
				}
			}
		} else {
			for _, rec := range batch {
				contents = append(contents, rec.Content)
			}
			for j := 0; j < len(batch); j++ {
				toLLMIndices = append(toLLMIndices, j)
			}
		}

		// 2. 对于缓存未命中的，调用判定模型
		if len(contents) > 0 {
			llmResults, err := m.judge.JudgeBatch(ctx, contents)
			if err != nil {
				logger.Error("批量判定失败", err)
				// 处理失败情况... (暂时跳过本批次)
				continue
			}
			for k, res := range llmResults {
				idx := toLLMIndices[k]
				results[idx] = res
				// 存入缓存
				if m.monitor != nil {
					m.monitor.SetJudgeResultCache(batch[idx].Content, res)
				}
			}
		}

		// 3. 处理最终结果（来自缓存或LLM）
		for j, result := range results {
			if result == nil {
				continue
			}
			content := batch[j].Content

			// 日志：打印判定结果，方便排查
			logger.System("STM判定结果", "index", j, "score", result.ValueScore, "stage", result.ShouldStage, "critical", result.IsCritical, "cat", result.Category)

			if result.IsCritical || (result.ShouldStage && result.ValueScore >= m.cfg.StagingValueThreshold) {
				// 【优化】先总结重构，存储精炼后的内容到Staging
				summary, err := m.judge.SummarizeAndRestructure(ctx, content, result.Category)
				if err != nil {
					logger.Error("总结重构失败，使用原文", err)
					summary = content // 降级：使用原始内容
				}

				// 存储总结后的内容（原始内容已在STM中，无需重复存储）
				if result.IsCritical {
					// 【绿色通道】跳过暂存区，直接尝试晋升 LTM
					logger.System("🚀 [Fast-Track] 发现关键事实/强烈意图，直连 LTM", "user", userID, "category", result.Category)
					if err := m.promoteToLTMCorrelator(ctx, userID, summary, result.Category, result.ConfidenceScore, result.Tags, result.Entities, "fast-track"); err != nil {
						logger.Error("绿色通道晋升失败", err)
						// 降级：如果直连失败，依然存入 Staging 兜底
						if err := m.stagingStore.AddOrIncrement(ctx, userID, sessionID, summary, result, m.embedder); err != nil {
							logger.Error("降级存入暂存区失败", err)
						}
					}
				} else {
					// 正常流程：进入暂存区
					if err := m.stagingStore.AddOrIncrement(ctx, userID, sessionID, summary, result, m.embedder); err != nil {
						logger.Error("添加到暂存区失败", err)
					}
				}
			}

			// 【自动删除】不管是否满足价值阈值，判定过的记录都从STM物理删除，
			// 因为有价值的已经去 Staging 了，无价值的也不需要留在 STM 占用上下文。
			recordData, _ := json.Marshal(batch[j])
			if err := m.stmStore.LRem(ctx, key, 0, string(recordData)); err != nil {
				logger.Error("从STM删除记录失败", err)
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
			if err := m.promoteToLTMCorrelator(ctx, entry.UserID, entry.Content, entry.Category, entry.ConfidenceScore, entry.ExtractedTags, entry.ExtractedEntities, "auto"); err != nil {
				logger.Error("自动晋升失败", err)
			} else {
				// 晋升成功后删除 Staging 条目
				m.stagingStore.Delete(ctx, entry.ID)
			}
		} else if entry.ConfidenceScore >= m.cfg.StagingConfidenceLow {
			// 中等信心：需要用户确认（暂时跳过，等待Admin界面确认）
			logger.MemoryCheck("pending_review", 1, fmt.Sprintf("score: %.2f, content: %s", entry.ConfidenceScore, entry.Content[:50]))
			// TODO: 触发用户确认机制(WebSocket/Admin Dashboard)
		} else {
			// 低信心：直接删除
			m.stagingStore.Delete(ctx, entry.ID)
			GetGlobalMetrics().RecordPromotion(string(entry.Category), false)
		}
	}

	return nil
}

// promoteSingleEntry 保持 API 兼容性（可选）
func (m *Manager) promoteSingleEntry(ctx context.Context, entry *types.StagingEntry, confirmedBy string) error {
	if err := m.promoteToLTMCorrelator(ctx, entry.UserID, entry.Content, entry.Category, entry.ConfidenceScore, entry.ExtractedTags, entry.ExtractedEntities, confirmedBy); err != nil {
		return err
	}
	return m.stagingStore.Delete(ctx, entry.ID)
}

// promoteToLTMCorrelator 核心晋升关联器：处理 LTM 写入前的去重、合并与结构化提取
func (m *Manager) promoteToLTMCorrelator(ctx context.Context, userID, summary string, category types.MemoryCategory, confidence float64, fallbackTags []string, fallbackEntities map[string]string, confirmedBy string) error {
	// 1. 生成 Embedding
	vector, err := m.embedder.EmbedQuery(ctx, summary)
	if err != nil {
		return fmt.Errorf("生成embedding失败: %w", err)
	}

	// 2. 在 LTM 中搜索相似记忆进行去重/合并
	filters := map[string]interface{}{"user_id": userID}
	similarRecords, _ := m.vectorStore.Search(ctx, vector, 1, 0.95, filters)

	if len(similarRecords) > 0 {
		// 找到相似记忆，调用智能合并策略
		existing := similarRecords[0]
		strategy, mergedContent, err := m.judge.DecideMergeStrategy(ctx, existing.Content, summary)
		if err != nil {
			logger.Error("合并策略判定失败", err)
			strategy = "keep_both" // 降级：都保留
		}

		switch strategy {
		case "update_existing":
			if count, ok := existing.Metadata["access_count"].(int); ok {
				existing.Metadata["access_count"] = count + 1
			} else {
				existing.Metadata["access_count"] = 1
			}
			existing.Metadata["decay_score"] = 1.0
			existing.Metadata["last_access_at"] = time.Now()
			m.vectorStore.Update(ctx, existing)
			logger.System("LTM去重：更新计数", "strategy", strategy, "existing_id", existing.ID)

		case "merge":
			existing.Content = mergedContent
			newVector, _ := m.embedder.EmbedQuery(ctx, mergedContent)
			if newVector != nil {
				existing.Embedding = newVector
			}
			if count, ok := existing.Metadata["access_count"].(int); ok {
				existing.Metadata["access_count"] = count + 1
			}
			existing.Metadata["decay_score"] = 1.0
			m.vectorStore.Update(ctx, existing)
			logger.System("LTM去重：合并内容", "strategy", strategy, "existing_id", existing.ID)

		case "keep_newer":
			m.vectorStore.Delete(ctx, []string{existing.ID})
			goto createNew

		case "keep_both":
			goto createNew
		}

		GetGlobalMetrics().RecordPromotion(string(category), true)
		return nil
	}

createNew:
	// 正常创建或 keep_both/keep_newer 后的创建
	tags, entities, err := m.judge.ExtractStructuredTags(ctx, summary, category)
	if err != nil {
		tags = fallbackTags
		entities = fallbackEntities
	}

	now := time.Now()
	metadataMap := map[string]interface{}{
		"user_id":           userID,
		"created_at":        now,
		"tags":              tags,
		"entities":          entities,
		"category":          string(category),
		"last_access_at":    now,
		"access_count":      0,
		"decay_score":       1.0,
		"source_type":       confirmedBy,
		"confidence_origin": confidence,
	}

	ltmRecord := types.Record{
		ID:        uuid.New().String(),
		Content:   summary,
		Embedding: vector,
		Timestamp: now,
		Metadata:  metadataMap,
		Type:      types.LongTerm,
	}

	if err := m.vectorStore.Add(ctx, []types.Record{ltmRecord}); err != nil {
		return fmt.Errorf("写入LTM失败: %w", err)
	}

	GetGlobalMetrics().RecordPromotion(string(category), true)
	logger.MemoryPromotion(string(category), confirmedBy, confidence, summary)
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

	// 任务4：定期LTM去重（每周执行）
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Hour * 24 * 7) // 每周
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.DeduplicateLTM(m.ctx); err != nil {
					logger.Error("LTM去重任务失败", err)
				}
			case <-m.ctx.Done():
				return
			}
		}
	}()

	logger.System("✅ 后台调度器已启动: STM清洗 + Staging晋升 + 记忆衰减 + LTM去重")
}

// Shutdown 优雅关闭
func (m *Manager) Shutdown() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	logger.System("Manager Shutdown")
}
