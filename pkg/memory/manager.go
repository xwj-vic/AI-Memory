package memory

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/prompts"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Manager implements the Memory interface.
type Manager struct {
	cfg         *config.Config
	prompts     *prompts.Registry
	vectorStore VectorStore
	stmStore    ListStore
	embedder    Embedder
	llm         llm.LLM
}

func NewManager(cfg *config.Config, vStore VectorStore, lStore ListStore, embedder Embedder, llmModel llm.LLM) *Manager {
	return &Manager{
		cfg:         cfg,
		prompts:     prompts.NewRegistry(cfg.SummarizePrompt, cfg.ExtractProfilePrompt),
		vectorStore: vStore,
		stmStore:    lStore,
		embedder:    embedder,
		llm:         llmModel,
	}
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
	if err := m.stmStore.RPush(ctx, key, data); err != nil {
		return fmt.Errorf("failed to add to STM: %w", err)
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
				Type:      types.Entity,
			}
			_ = m.vectorStore.Add(ctx, []types.Record{attrRecord})
			fmt.Printf("Identified and stored user attributes for %s.\n", userID)
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

	fmt.Printf("Summarized %d items into LTM for %s (Session %s).\n", len(stmData), userID, sessionID)
	return nil
}

// Clear resets both stores.
func (m *Manager) Clear(ctx context.Context, userID string, sessionID string) error {
	key := fmt.Sprintf("memory:stm:%s:%s", userID, sessionID)
	if err := m.stmStore.Del(ctx, key); err != nil {
		return err
	}
	fmt.Printf("STM cleared for user %s session %s.\n", userID, sessionID)
	return nil
}
