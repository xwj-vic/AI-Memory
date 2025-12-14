package memory_test

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/memory"
	"ai-memory/pkg/store"
	"ai-memory/pkg/types"
	"context"
	"testing"
	"time"
)

func TestManager_DemoFlow(t *testing.T) {
	// Setup
	cfg := &config.Config{
		ContextWindow:        10,
		MaxRecentMemories:    100,
		MinSummaryItems:      2, // Low threshold for test
		SummarizePrompt:      "%s",
		ExtractProfilePrompt: "None",
	}

	mockLLM := &llm.MockLLM{}
	vStore := store.NewInMemoryVectorStore()
	lStore := store.NewInMemoryListStore()

	// Pass nil for EndUserStore
	m := memory.NewManager(cfg, vStore, lStore, nil, mockLLM, mockLLM)

	ctx := context.Background()
	userID := "test_user"
	sessionA := "session_a"
	sessionB := "session_b"

	// 1. Add interactions to Session A
	interactionsA := []struct {
		in, out string
	}{
		{"Hello A1", "Hi A1"},
		{"Hello A2", "Hi A2"},
		{"Hello A3", "Hi A3"},
	}

	for _, itr := range interactionsA {
		if err := m.Add(ctx, userID, sessionA, itr.in, itr.out, nil); err != nil {
			t.Fatalf("Failed to add to Session A: %v", err)
		}
	}

	// 2. Add interactions to Session B
	if err := m.Add(ctx, userID, sessionB, "Hello B1", "Hi B1", nil); err != nil {
		t.Fatalf("Failed to add to Session B: %v", err)
	}

	// 3. Retrieve Session A (Should only see A)
	resA, err := m.Retrieve(ctx, userID, sessionA, "context", 10)
	if err != nil {
		t.Fatalf("Retrieve A failed: %v", err)
	}
	// Depending on retrieve logic and mock embedding, we might check count
	// InMemory Search returns all if mock embedding matches, but here mock embedding is zero vector?
	// CosineSim of zero vectors might be 0.
	// But Retrieve gets STM as well!
	// STM should return all 3 recent items.
	if len(resA) < 3 {
		t.Errorf("Expected at least 3 items in Session A STM, got %d", len(resA))
	}

	// 4. Retrieve Session B (Should only see B in STM)
	resB, err := m.Retrieve(ctx, userID, sessionB, "context", 10)
	if err != nil {
		t.Fatalf("Retrieve B failed: %v", err)
	}
	if len(resB) < 1 {
		t.Errorf("Expected at least 1 item in Session B STM, got %d", len(resB))
	}

	// 5. Summarize Session A
	if err := m.Summarize(ctx, userID, sessionA); err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// 6. Verify Session A STM is cleared
	resA2, _ := m.Retrieve(ctx, userID, sessionA, "context", 10)
	stmCount := 0
	for _, r := range resA2 {
		if r.Type == "short_term" {
			stmCount++
		}
	}
	if stmCount != 0 {
		t.Errorf("Expected Session A STM to be empty after summary, got %d", stmCount)
	}

	// 7. Verify LTM exists (shared)
	// Both sessions should technically be able to see LTM if embedding search works.
	// Since MockLLM returns same vector, everything matches everything with score 1 (if normalized) or depends on impl.
	// InMemoryStore Search logic: CosineSim(zero, zero) = NaN?
	// Let's check List() instead for existence
	// 7. Verify LTM exists (shared)
	// Both sessions should technically be able to see LTM if embedding search works.
	// Since MockLLM returns same vector, everything matches everything with score 1 (if normalized) or depends on impl.
	// InMemoryStore Search logic: CosineSim(zero, zero) = NaN?
	// Let's check List() instead for existence
	ltmRecords, err := m.List(ctx, memory.Filter{Type: "long_term"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ltmRecords) == 0 {
		t.Error("Expected LTM records after summary")
	}
}

func TestManager_ListFiltering(t *testing.T) {
	// Setup
	cfg := &config.Config{
		ContextWindow:     10,
		MaxRecentMemories: 100,
	}

	mockLLM := &llm.MockLLM{}
	vStore := store.NewInMemoryVectorStore()
	lStore := store.NewInMemoryListStore()

	m := memory.NewManager(cfg, vStore, lStore, nil, mockLLM, mockLLM)
	ctx := context.Background()

	// Seed Data
	// User 1, Session A
	_ = m.Add(ctx, "u1", "sA", "in1", "out1", nil) // STM
	_ = m.Add(ctx, "u1", "sA", "in2", "out2", nil) // STM

	// User 2, Session B
	_ = m.Add(ctx, "u2", "sB", "in3", "out3", nil) // STM

	// Check STM Filter for u1
	recs1, err := m.List(ctx, memory.Filter{UserID: "u1", Type: "short_term"})
	if err != nil {
		t.Fatalf("List u1 failed: %v", err)
	}
	if len(recs1) != 2 {
		t.Errorf("Expected 2 STM records for u1, got %d", len(recs1))
	}

	// Check STM Filter for u2
	recs2, err := m.List(ctx, memory.Filter{UserID: "u2", Type: "short_term"})
	if err != nil {
		t.Fatalf("List u2 failed: %v", err)
	}
	if len(recs2) != 1 {
		t.Errorf("Expected 1 STM record for u2, got %d", len(recs2))
	}

	// Pagination Test
	// Add more to u1
	_ = m.Add(ctx, "u1", "sA", "in4", "out4", nil)
	_ = m.Add(ctx, "u1", "sA", "in5", "out5", nil)
	// Total 4 for u1

	recsPage1, err := m.List(ctx, memory.Filter{UserID: "u1", Type: "short_term", Limit: 2, Page: 1})
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}
	if len(recsPage1) != 2 {
		t.Errorf("Expected 2 records on page 1, got %d", len(recsPage1))
	}

	recsPage2, err := m.List(ctx, memory.Filter{UserID: "u1", Type: "short_term", Limit: 2, Page: 2})
	if err != nil {
		t.Fatalf("List page 2 failed: %v", err)
	}
	if len(recsPage2) != 2 {
		t.Errorf("Expected 2 records on page 2, got %d", len(recsPage2))
	}

	// Ensure they are different items (by timestamp or content)
	// Just check content uniqueness across pages if possible, or assume sort works
	if recsPage1[0].ID == recsPage2[0].ID {
		t.Errorf("Page 1 and Page 2 first item ID shouldn't match")
	}
}

func TestManager_Update(t *testing.T) {
	// Setup
	cfg := &config.Config{ContextWindow: 10}
	mockLLM := &llm.MockLLM{}
	vStore := store.NewInMemoryVectorStore()
	lStore := store.NewInMemoryListStore()
	m := memory.NewManager(cfg, vStore, lStore, nil, mockLLM, mockLLM)
	ctx := context.Background()

	// 1. Create LTM (Simulate via Summarize is hard, use manual Add to vStore or hack)
	// Manager.Add adds to STM.
	// We need to add to VectorStore directly for testing Update comfortably without summarize flow.
	// But Manager only exposes "Add" (STM) or "Summarize" (STM->LTM).
	// Let's use internal vectorStore access since we constructed it?

	id := "test-ltm-id"
	record := types.Record{
		ID:        id,
		Content:   "Original Content",
		Timestamp: time.Now(),
		Type:      types.LongTerm,
		Metadata:  map[string]interface{}{"user_id": "u1"},
	}
	_ = vStore.Add(ctx, []types.Record{record})

	// 2. Verify Original
	rec, err := vStore.Get(ctx, id)
	if err != nil {
		t.Fatalf("Failed to get original: %v", err)
	}
	if rec.Content != "Original Content" {
		t.Errorf("Content mismatch")
	}

	// 3. Update via Manager
	newContent := "Updated Content"
	if err := m.Update(ctx, id, newContent); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 4. Verify Update
	updatedRec, err := vStore.Get(ctx, id)
	if err != nil {
		t.Fatalf("Failed to get updated: %v", err)
	}
	if updatedRec.Content != newContent {
		t.Errorf("Expected %s, got %s", newContent, updatedRec.Content)
	}
}

func TestManager_Update_STM(t *testing.T) {
	// Setup
	cfg := &config.Config{ContextWindow: 10}
	mockLLM := &llm.MockLLM{}
	vStore := store.NewInMemoryVectorStore()
	lStore := store.NewInMemoryListStore()
	m := memory.NewManager(cfg, vStore, lStore, nil, mockLLM, mockLLM)
	ctx := context.Background()

	// 1. Add to STM via Manager
	if err := m.Add(ctx, "u1", "s1", "Hello STM", "Hi STM", nil); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// 2. Find the ID (List)
	recs, _ := m.List(ctx, memory.Filter{Type: "short_term"})
	if len(recs) == 0 {
		t.Fatal("STM empty")
	}
	id := recs[0].ID

	// 3. Update
	newContent := "Updated STM Content"
	if err := m.Update(ctx, id, newContent); err != nil {
		t.Fatalf("Update STM failed: %v", err)
	}

	// 4. Verify
	// Check via ListStore Get
	rec, err := lStore.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get STM failed: %v", err)
	}
	if rec.Content != newContent {
		t.Errorf("Expected %s, got %s", newContent, rec.Content)
	}
}
