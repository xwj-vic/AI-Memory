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

const STMKey = "memory:stm:chat_history"

// Add stores a new interaction in Short-Term Memory (Redis).
func (m *Manager) Add(ctx context.Context, input string, output string, metadata map[string]interface{}) error {
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

	// Push to Redis List
	if err := m.stmStore.RPush(ctx, STMKey, data); err != nil {
		return fmt.Errorf("failed to add to STM: %w", err)
	}

	return nil
}

// Retrieve finds relevant memories from both STM (recent context) and LTM (vector search).
func (m *Manager) Retrieve(ctx context.Context, query string, limit int) ([]types.Record, error) {
	var allRecords []types.Record

	// 1. Fetch STM (Recent History)
	stmData, err := m.stmStore.LRange(ctx, STMKey, 0, -1) // Get all for context, or limit?
	if err == nil {
		// Parse STM
		// We usually want the LAST N items for context, but here we just return them.
		// Reverse order is often better for "recent first" in UI, but chronological is standard for LLM context.
		// Let's take the last 'limit' items if we wanted strict limit, but usually STM is strictly time-bound.
		// For RETRIEVAL (RAG), we might want semantic search even on STM?
		// For now, we append *all* STM as "Recent Context" and then search LTM.

		// Optimization: Only take last N defined in config
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

	// 2. Search LTM (Vector Store)
	vector, err := m.embedder.EmbedQuery(ctx, query)
	if err != nil {
		// Log error but prioritize returning STM if LTM fails?
		// For now, return error.
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	ltmRecords, err := m.vectorStore.Search(ctx, vector, limit, 0.7) // 0.7 threshold
	if err == nil {
		allRecords = append(allRecords, ltmRecords...)
	}

	return allRecords, nil
}

// Summarize consolidates STM into LTM.
func (m *Manager) Summarize(ctx context.Context) error {
	// 1. Fetch all STM
	stmData, err := m.stmStore.LRange(ctx, STMKey, 0, -1)
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

	// 2a. Parallel Strategy: Extract User Attributes (Entity LTM)
	// Only proceed if length is significant or config enabled (implicit in MinSummaryItems)
	attrPrompt := m.prompts.GetExtractProfilePrompt(contentBuilder)
	attributes, err := m.llm.GenerateText(ctx, attrPrompt)
	if err == nil && len(attributes) > 10 && attributes != "None" {
		// Valid attributes found, store separately
		attrVec, _ := m.embedder.EmbedQuery(ctx, attributes)
		if attrVec != nil {
			attrRecord := types.Record{
				ID:        uuid.New().String(),
				Content:   fmt.Sprintf("User Attributes identified:\n%s", attributes),
				Embedding: attrVec,
				Timestamp: time.Now(),
				Type:      types.Entity,
			}
			// Best effort store
			_ = m.vectorStore.Add(ctx, []types.Record{attrRecord})
			fmt.Printf("Identified and stored user attributes.\n")
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
		Type:      types.LongTerm,
	}

	if err := m.vectorStore.Add(ctx, []types.Record{ltmRecord}); err != nil {
		return fmt.Errorf("failed to store LTM: %w", err)
	}

	// 5. Clear STM (or Archive)
	// We clear the list. In a real system, we might keep the last few items for continuity.
	// Implementing "Rolling Window": keep last 2, delete rest.
	// For simplicity in this step: Clear All.
	if err := m.stmStore.Del(ctx, STMKey); err != nil {
		return fmt.Errorf("failed to clear STM: %w", err)
	}

	// Optional: Re-add the very last interaction? (Skip for now to keep it clean)

	fmt.Printf("Summarized %d items into LTM.\n", len(stmData))
	return nil
}

// Clear resets both stores.
func (m *Manager) Clear(ctx context.Context) error {
	if err := m.stmStore.Del(ctx, STMKey); err != nil {
		return err
	}
	// Clear LTM... VectorStore doesn't expose ClearAll except List+Delete.
	// We can implement that logic here or rely on VectorStore methods.
	// For now, reusing existing logic if method exists, else manually list and delete.
	// The interface has List and Delete.
	records, err := m.vectorStore.List(ctx)
	if err != nil {
		return err
	}
	var ids []string
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	if len(ids) > 0 {
		return m.vectorStore.Delete(ctx, ids)
	}
	return nil
}
