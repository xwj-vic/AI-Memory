package memory

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/logger"
	"ai-memory/pkg/prompts"
	"ai-memory/pkg/store"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager implements the Memory interface.
type Manager struct {
	cfg          *config.Config
	prompts      *prompts.Registry
	vectorStore  VectorStore
	stmStore     ListStore
	endUserStore EndUserStore
	embedder     Embedder
	llm          llm.LLM

	// 漏斗型记忆组件
	judge           *Judge
	stagingStore    *store.StagingStore
	decayCalculator *DecayCalculator

	// 后台任务控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(cfg *config.Config, vStore VectorStore, lStore ListStore, uStore EndUserStore, embedder Embedder, llmModel llm.LLM, redisStore *store.RedisStore) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化漏斗组件
	judge := NewJudge(llmModel, cfg.JudgeModel, cfg.ExtractTagsModel)
	stagingStore := store.NewStagingStore(redisStore.GetClient(), 30) // TTL 30天
	decayCalc := NewDecayCalculator(cfg.LTMDecayHalfLifeDays, cfg.LTMDecayMinScore)

	m := &Manager{
		cfg:             cfg,
		prompts:         prompts.NewRegistry(cfg.SummarizePrompt, cfg.ExtractProfilePrompt),
		vectorStore:     vStore,
		stmStore:        lStore,
		endUserStore:    uStore,
		embedder:        embedder,
		llm:             llmModel,
		judge:           judge,
		stagingStore:    stagingStore,
		decayCalculator: decayCalc,
		ctx:             ctx,
		cancel:          cancel,
	}

	// 启动后台协程
	m.startBackgroundTasks()

	return m
}

type Filter struct {
	UserID string
	Type   string // "short_term", "long_term", "all"
	Limit  int
	Page   int
}

// Add stores a new interaction in Short-Term Memory (Redis).
func (m *Manager) Add(ctx context.Context, userID string, sessionID string, input string, output string, metadata map[string]interface{}) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["user_id"] = userID
	metadata["session_id"] = sessionID

	record := types.Record{
		ID:        uuid.New().String(),
		Content:   fmt.Sprintf("User: %s\nAI: %s", input, output),
		Timestamp: time.Now(),
		Metadata:  metadata,
		Type:      types.ShortTerm,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Push to Redis List associated with User AND Session
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)
	if err := m.stmStore.RPushWithExpire(ctx, key, m.cfg.STMExpirationDays, data); err != nil {
		return fmt.Errorf("failed to add to STM: %w", err)
	}

	// Update EndUser Activity
	if m.endUserStore != nil {
		_ = m.endUserStore.UpsertUser(ctx, userID)
	}

	return nil
}

// Retrieve finds relevant memories from both STM (recent context) and LTM (vector search).
func (m *Manager) Retrieve(ctx context.Context, userID string, sessionID string, query string, limit int) ([]types.Record, error) {
	var allRecords []types.Record
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)

	// 1. Fetch STM (Session Context)
	stmData, err := m.stmStore.LRange(ctx, key, 0, -1)
	if err == nil {
		start := 0
		if len(stmData) > m.cfg.ContextWindow {
			start = len(stmData) - m.cfg.ContextWindow
		}

		for i := start; i < len(stmData); i++ {
			var rec types.Record
			if json.Unmarshal([]byte(stmData[i]), &rec) == nil {
				allRecords = append(allRecords, rec)
			}
		}
	}

	// 2. Search LTM (User Context)
	remainingSlots := limit
	if m.cfg.MaxRecentMemories > 0 && limit > m.cfg.MaxRecentMemories {
		remainingSlots = m.cfg.MaxRecentMemories
	}

	vector, err := m.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Filter by User ID (access to ALL past sessions)
	filters := map[string]interface{}{
		"user_id": userID,
	}

	ltmRecords, err := m.vectorStore.Search(ctx, vector, remainingSlots, 0.7, filters)
	if err == nil {
		allRecords = append(allRecords, ltmRecords...)
	}

	// Enforce global MaxRecentMemories
	if m.cfg.MaxRecentMemories > 0 && len(allRecords) > m.cfg.MaxRecentMemories {
		allRecords = allRecords[:m.cfg.MaxRecentMemories]
	}

	return allRecords, nil
}

// Summarize consolidates STM into LTM.
//
// ⚠️ 【已废弃】此方法为传统的手动汇总方案，已被漏斗型记忆系统取代
//
// 传统方案流程：
//
//	STM → 手动触发Summary → LLM生成摘要 → 直接写入LTM → 清空STM
//
// 新方案（漏斗型）推荐使用：
//
//	STM → 自动LLM判定 → Staging暂存 → 频次验证 → 人工审核 → LTM（带标签） → 衰减遗忘
//	参见：JudgeAndStageFromSTM() 和 PromoteStagingToLTM()
//
// 保留此方法的原因：
//  1. 向后兼容：现有调用代码仍可工作
//  2. 快速汇总：某些场景下需要立即汇总（不经过漏斗）
//  3. 手动控制：管理员可手动触发特定会话的汇总
//
// 建议：新项目请使用漏斗型方案，此方法仅用于兼容和特殊场景
//
// Deprecated: 推荐使用 JudgeAndStageFromSTM + PromoteStagingToLTM 实现自动化记忆管理
func (m *Manager) Summarize(ctx context.Context, userID string, sessionID string) error {
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)

	// 1. Fetch all STM
	stmData, err := m.stmStore.LRange(ctx, key, 0, -1)
	if err != nil {
		return fmt.Errorf("failed to list STM: %w", err)
	}

	if len(stmData) < m.cfg.MinSummaryItems {
		return nil // Not enough data to summarize
	}

	var contentBuilder string
	for _, data := range stmData {
		var rec types.Record
		if err := json.Unmarshal([]byte(data), &rec); err == nil {
			contentBuilder += rec.Content + "\n"
		}
	}

	// 2. Generate Summary (Episodic LTM)
	prompt := m.prompts.GetSummarizePrompt(contentBuilder)
	summary, err := m.llm.GenerateText(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// 2a. Entity Extraction
	attrPrompt := m.prompts.GetExtractProfilePrompt(contentBuilder)
	attributes, err := m.llm.GenerateText(ctx, attrPrompt)
	if err == nil && len(attributes) > 10 && attributes != "None" {
		attrVec, _ := m.embedder.EmbedQuery(ctx, attributes)
		if attrVec != nil {
			attrRecord := types.Record{
				ID:        uuid.New().String(),
				Content:   fmt.Sprintf("User Attributes identified:\n%s", attributes),
				Embedding: attrVec,
				Timestamp: time.Now(),
				Metadata:  map[string]interface{}{"user_id": userID, "source_session": sessionID},
				Type:      types.LongTerm,
			}
			_ = m.vectorStore.Add(ctx, []types.Record{attrRecord})
			logger.Info("Identified and stored user attributes", "user_id", userID)
		}
	}

	// 3. Embed Summary (Episodic)
	vector, err := m.embedder.EmbedQuery(ctx, summary)
	if err != nil {
		return fmt.Errorf("failed to embed summary: %w", err)
	}

	// 4. Store in LTM
	ltmRecord := types.Record{
		ID:        uuid.New().String(),
		Content:   summary,
		Embedding: vector,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"user_id": userID, "source_session": sessionID},
		Type:      types.LongTerm,
	}

	if err := m.vectorStore.Add(ctx, []types.Record{ltmRecord}); err != nil {
		return fmt.Errorf("failed to store LTM: %w", err)
	}

	// 5. Clear STM (Session Specific)
	if err := m.stmStore.Del(ctx, key); err != nil {
		return fmt.Errorf("failed to clear STM: %w", err)
	}

	logger.System("Summarized items into LTM", "count", len(stmData), "user_id", userID, "session_id", sessionID)
	return nil
}

// List retrieves all records with filtering.
func (m *Manager) List(ctx context.Context, filter Filter) ([]types.Record, error) {
	var results []types.Record

	// Defaults
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.Limit

	// 1. Fetch Short-Term Memory if requested
	if filter.Type == "short_term" || filter.Type == "all" || filter.Type == "" {
		// If UserID is provided, search specific session keys
		// Pattern: memory:stm:<UserID>:*
		pattern := "memory:stm:*:*"
		if filter.UserID != "" {
			pattern = fmt.Sprintf("memory:stm:%s:*", filter.UserID)
		}

		keys, err := m.stmStore.ScanKeys(ctx, pattern)
		if err == nil {
			for _, key := range keys {
				// Fetch all items from list (inefficient for large lists but STM is short by definition)
				items, _ := m.stmStore.LRange(ctx, key, 0, -1)
				for _, data := range items {
					var rec types.Record
					if err := json.Unmarshal([]byte(data), &rec); err == nil {
						// Filter by UserID check (redundant if key matched, but safe)
						if filter.UserID != "" {
							if metaUser, ok := rec.Metadata["user_id"].(string); ok && metaUser != filter.UserID {
								continue
							}
						}
						results = append(results, rec)
					}
				}
			}
		}
	}

	// 2. Fetch Long-Term Memory if requested
	if filter.Type == "long_term" || filter.Type == "all" || filter.Type == "" {
		// Call Vector Store List with filters
		vFilters := make(map[string]interface{})
		if filter.UserID != "" {
			vFilters["user_id"] = filter.UserID
		}
		// If requesting "long_term", we want ALL records in VectorStore (LTM + Legacy Entity).
		// VectorStore does not contain ShortTerm.
		// So we only apply specific type filter if it's NOT long_term (and NOT all).
		if filter.Type != "" && filter.Type != "all" && filter.Type != "long_term" {
			vFilters["type"] = filter.Type
		}

		// For LTM, we use the store's pagination if we are ONLY fetching LTM.
		// If we are mixing (All), pagination becomes complex (STM + LTM).
		// For MVP:
		// If Type == "long_term", we rely on store pagination.
		// If Type == "all" or "short_term", we fetch and paginate in memory (since we have to merge STM).

		if filter.Type == "long_term" {
			ltmRecs, err := m.vectorStore.List(ctx, vFilters, filter.Limit, offset)
			if err != nil {
				return nil, err
			}
			results = append(results, ltmRecs...)
			// If we are only doing LTM, we are done (assuming store handled offset)
			// But wait, List return type is []Record.
			return results, nil
		} else {
			// "all" or "short_term" mixed with LTM?
			// If "all", we fetch LTM too, but without offset/limit at store level? No, that's too heavy.
			// Strategy: Fetch LTM page 1 (or up to limit?)
			// If we blend, standard pagination is hard.
			// Simplified approach for "all":
			// Fetch STM.
			// Fetch LTM (with limit).
			// Combine, Sort by Timestamp Descending.
			// Slice options.

			// We'll fetch LTM with loose limit (e.g. limit + offset) just in case?
			// Or just simple: STM is usually small.
			// Let's Load STM, then append LTM.

			// If Filter is "all", we fetch LTM as well.
			if filter.Type == "all" || filter.Type == "" {
				ltmRecs, err := m.vectorStore.List(ctx, vFilters, filter.Limit+offset, 0) // Fetch from 0 to needed count
				if err == nil {
					results = append(results, ltmRecs...)
				}
			}
		}
	}

	// 3. In-Memory Sort and Paginate (for merged results)
	// Sort by Timestamp Descending
	// (Assuming we want newest first)
	// Import "sort" is needed? Manager file imports.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// Pagination
	total := len(results)
	if offset >= total {
		return []types.Record{}, nil
	}
	end := offset + filter.Limit
	if end > total {
		end = total
	}

	// Slice
	return results[offset:end], nil
}

// Update modifies a memory record.
func (m *Manager) Update(ctx context.Context, id string, newContent string) error {
	var rec *types.Record
	var isLTM bool

	// 1. Try LTM
	if r, err := m.vectorStore.Get(ctx, id); err == nil {
		rec = r
		isLTM = true
	} else {
		// 2. Try STM
		if r, err := m.stmStore.Get(ctx, id); err == nil {
			rec = r
			isLTM = false
		} else {
			return fmt.Errorf("record not found in LTM or STM")
		}
	}

	// 3. Re-embed
	// STM also uses embeddings in our 'Add' logic, so we should update it.
	vector, err := m.embedder.EmbedQuery(ctx, newContent)
	if err != nil {
		return fmt.Errorf("failed to embed new content: %w", err)
	}

	// 4. Update fields
	rec.Content = newContent
	rec.Embedding = vector
	// Keep Timestamp

	// 5. Save
	if isLTM {
		if err := m.vectorStore.Update(ctx, *rec); err != nil {
			return fmt.Errorf("failed to update LTM: %w", err)
		}
	} else {
		if err := m.stmStore.Update(ctx, *rec); err != nil {
			return fmt.Errorf("failed to update STM: %w", err)
		}
	}

	return nil
}

// Delete removes a record from LTM by ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.vectorStore.Delete(ctx, []string{id})
}

// Clear resets both stores.
func (m *Manager) Clear(ctx context.Context, userID string, sessionID string) error {
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)
	if err := m.stmStore.Del(ctx, key); err != nil {
		return err
	}
	logger.System("STM cleared", "user_id", userID, "session_id", sessionID)
	return nil
}

// GetUsers returns list of end users с stats.
func (m *Manager) GetUsers(ctx context.Context) ([]types.EndUser, error) {
	if m.endUserStore == nil {
		return nil, fmt.Errorf("end user store not initialized")
	}

	// 1. Fetch Users from MySQL
	users, err := m.endUserStore.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Enrich with Stats
	for i := range users {
		u := &users[i]

		// STM Sessions Count
		// Pattern: memory:stm:<UserID>:*
		pattern := fmt.Sprintf("memory:stm:%s:*", u.UserIdentifier)
		keys, _ := m.stmStore.ScanKeys(ctx, pattern)
		u.SessionCount = len(keys)

		// LTM Count
		count, _ := m.vectorStore.Count(ctx, map[string]interface{}{"user_id": u.UserIdentifier})
		u.LTMCount = int(count)
	}

	return users, nil
}

// GetSystemStatus returns basic health info.
func (m *Manager) GetSystemStatus(ctx context.Context) map[string]string {
	status := make(map[string]string)

	// Check STM
	if _, err := m.stmStore.ScanKeys(ctx, "test"); err != nil {
		status["ShortTermMemory"] = "Down"
	} else {
		status["ShortTermMemory"] = "Online"
	}

	// Check LTM
	if _, err := m.vectorStore.List(ctx, map[string]interface{}{}, 1, 0); err != nil {
		status["LongTermMemory"] = "Down / Error"
	} else {
		status["LongTermMemory"] = "Online"
	}

	return status
}
