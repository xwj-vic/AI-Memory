package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"ai-memory/pkg/config"
	"ai-memory/pkg/llm"
	"ai-memory/pkg/memory"
	"ai-memory/pkg/store"
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
		// Ensure collection exists. Vector size 1536 for OpenAI ada-002
		if err := qs.Init(ctx, 1536); err != nil {
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

func runDemo(ctx context.Context, m *memory.Manager) {
	// Add interactions
	fmt.Println("\n--- Adding New Memories ---")
	inputs := []struct {
		in, out string
	}{
		{"Hello, I'm working on a memory system.", "That sounds interesting! How are you implementing it?"},
		{"I'm using Redis for short-term and Vector DB for long-term on Mac.", "Redis is a great choice for fast access. Vector DBs help with semantic retrieval."},
		{"My concern is when to summarize.", "Usually based on token count or number of turns."},
		{"I want to use OpenAI for the summarization.", "OpenAI's models are excellent for summarization tasks."},
		{"Can you help me write the Go code?", "Certainly! I can help you with Go implementation."},
		{"I love coding in Go.", "Go is efficient and simple, perfect for systems engineering."},
	}

	for _, p := range inputs {
		if err := m.Add(ctx, p.in, p.out, nil); err != nil {
			log.Printf("Error adding memory: %v", err)
		} else {
			fmt.Print(".")
		}
		time.Sleep(100 * time.Millisecond) // simulates time
	}
	fmt.Println("\nDone adding.")

	// Retrieve
	fmt.Println("\n--- Retrieving Context (STM + LTM) ---")
	results, err := m.Retrieve(ctx, "What logic am I using for implementation?", 5)
	if err != nil {
		log.Printf("Error retrieving: %v", err)
	}
	for i, r := range results {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
	}

	// Summarize
	fmt.Println("\n--- Triggering Logic: Summarization (STM -> LTM) ---")
	if err := m.Summarize(ctx); err != nil {
		log.Printf("Error summarizing: %v", err)
	} else {
		fmt.Println("Summarization complete.")
	}

	// Retrieve Again (Should see LTM now)
	fmt.Println("\n--- Retrieving After Summary ---")
	results2, err := m.Retrieve(ctx, "What logic am I using for implementation?", 5)
	if err != nil {
		log.Printf("Error retrieving: %v", err)
	}
	for i, r := range results2 {
		fmt.Printf("[%d] [%s] %s\n", i, r.Type, r.Content)
	}
}
