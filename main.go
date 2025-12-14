package main

import (
	"context"
	"fmt"
	"log"

	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/memory"
	"ai-memory/pkg/store"
	"ai-memory/pkg/types"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning loading config: %v", err)
	}

	ctx := context.Background()

	// 2. Initialize Infrastructure
	// Redis (STM)
	redisStore := store.NewRedisStore(cfg)
	if err := redisStore.Ping(ctx); err != nil {
		log.Printf("Warning: Redis connection failed: %v. Ensure Redis is running at %s", err, cfg.RedisAddr)
	} else {
		fmt.Println(" Connected to Redis (STM)")
	}

	// OpenAI (LLM & Embedder)
	var llmClient llm.LLM
	var embedderClient memory.Embedder

	switch cfg.LLMProvider {
	case "openai":
		client := llm.NewOpenAIClient(cfg)
		llmClient = client
		embedderClient = client
	default:
		// Default to OpenAI for now or error out
		log.Printf("Unknown provider %s, defaulting to OpenAI", cfg.LLMProvider)
		client := llm.NewOpenAIClient(cfg)
		llmClient = client
		embedderClient = client
	}

	// Vector Store (LTM)
	var vectorStore memory.VectorStore

	switch cfg.VectorStoreProvider {
	case "qdrant":
		qs, err := store.NewQdrantStore(cfg)
		if err != nil {
			log.Fatalf("Failed to initialize Qdrant: %v", err)
		}
		// Ensure collection exists. Vector size 1024 for BAAI/bge-m3
		if err := qs.Init(ctx, 1024); err != nil {
			log.Fatalf("Failed to init Qdrant collection: %v", err)
		}
		vectorStore = qs
		fmt.Println(" Connected to Qdrant (LTM)")
	case "in_memory":
		fallthrough
	default:
		// usage of In-Memory for now but with JSON persistence
		ims := store.NewInMemoryVectorStore()
		if err := ims.Load("memory.json"); err != nil {
			log.Printf("Starting with empty LTM: %v", err)
		}
		vectorStore = ims
		fmt.Println(" Connected to In-Memory Store (LTM)")
	}

	// 3. Create Manager
	manager := memory.NewManager(cfg, vectorStore, redisStore, embedderClient, llmClient)

	// 4. Interactive Demo Flow
	runDemo(ctx, manager)

	// 5. Save LTM State (Only for In-Memory)
	if ims, ok := vectorStore.(*store.InMemoryVectorStore); ok {
		if err := ims.Save("memory.json"); err != nil {
			log.Printf("Failed to save LTM: %v", err)
		}
	}
}

const DemoUserID = "demo_user"
const SessionA = "session_a"
const SessionB = "session_b"

func runDemo(ctx context.Context, m *memory.Manager) {
	fmt.Printf("=== Starting Demo for User: %s, Sessions: %s, %s ===\n", DemoUserID, SessionA, SessionB)

	// 1. Add to Session A (Coding Context)
	fmt.Println("\n--- [Session A] Adding Interactions ---")
	inputsA := []struct{ in, out string }{
		{"I want to write a Go server.", "I can help. Let's use net/http."},
		{"Do I need a framework?", "Standard library is often enough for simple services."},
	}
	for _, p := range inputsA {
		if err := m.Add(ctx, DemoUserID, SessionA, p.in, p.out, nil); err != nil {
			log.Printf("Error adding to Session A: %v", err)
		}
	}

	// 2. Add to Session B (Creative Context)
	fmt.Println("\n--- [Session B] Adding Interactions ---")
	inputsB := []struct{ in, out string }{
		{"Write a poem about rust (the metal).", "Iron red, oxidation spreads..."},
	}
	for _, p := range inputsB {
		if err := m.Add(ctx, DemoUserID, SessionB, p.in, p.out, nil); err != nil {
			log.Printf("Error adding to Session B: %v", err)
		}
	}

	// 3. Retrieve Session A (Should NOT see B)
	fmt.Println("\n--- [Session A] Retrieving Context ---")
	resultsA, _ := m.Retrieve(ctx, DemoUserID, SessionA, "context", 5)
	for i, r := range resultsA {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
	}

	// 4. Retrieve Session B (Should NOT see A)
	fmt.Println("\n--- [Session B] Retrieving Context ---")
	resultsB, _ := m.Retrieve(ctx, DemoUserID, SessionB, "context", 5)
	for i, r := range resultsB {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
	}

	// 5. Summarize Session A -> LTM
	fmt.Println("\n--- [Session A] Triggering Summarization ---")
	// Add more inputs to trigger threshold if needed, or force it
	// Just calling Summarize manually
	if err := m.Summarize(ctx, DemoUserID, SessionA); err != nil {
		log.Printf("Session A Summary Error (might be not enough items): %v", err)
	} else {
		fmt.Println("Session A Summarized.")
	}

	// 6. Verify Session A STM Cleared
	resultsA2, _ := m.Retrieve(ctx, DemoUserID, SessionA, "context", 5)
	if len(resultsA2) == 0 { // Actually might get LTM results now
		fmt.Println("\n--- [Session A] Post-Summary: STM should be empty, seeing LTM ---")
	}
	for i, r := range resultsA2 {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
	}

	// 7. Verify Session B can see Session A's Summary (via shared User LTM)
	fmt.Println("\n--- [Session B] Retrieving (Checking for A's Summary) ---")
	resultsB2, _ := m.Retrieve(ctx, DemoUserID, SessionB, "Go server", 5)
	for i, r := range resultsB2 {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
		if r.Type == types.LongTerm || r.Type == types.Entity {
			fmt.Println("  -> Confirmed: Session B accessed User LTM.")
		}
	}
}
