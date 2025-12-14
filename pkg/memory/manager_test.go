package memory_test

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/memory"
	"ai-memory/pkg/store"
	"context"
	"testing"
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

	m := memory.NewManager(cfg, vStore, lStore, mockLLM, mockLLM)

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
	ltmRecords, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ltmRecords) == 0 {
		t.Error("Expected LTM records after summary")
	}
}
