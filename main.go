package main

import (
	"context"
	"fmt"
	"log"

	"ai-memory/pkg/api"
	"ai-memory/pkg/auth"
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

	// MySQL (Auth & Admin)
	mysqlDB, err := store.NewMySQLStore(cfg)
	if err != nil {
		log.Printf("Warning: MySQL connection failed: %v", err)
	} else {
		fmt.Println(" Connected to MySQL")
	}

	// Initialize Auth Service
	var authService *auth.Service
	if mysqlDB != nil {
		authService = auth.NewService(mysqlDB)
		if err := authService.InitSchema(); err != nil {
			log.Printf("Warning: Failed to init auth schema: %v", err)
		} else {
			// Check if we need to seed a default admin?
			// For now just logging.
			fmt.Println(" Auth Schema Initialized")

			// Seed Default Admin
			if err := authService.CreateUser("admin", "admin123"); err == nil {
				fmt.Println(" Default Admin Created: admin / admin123")
			}
		}
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
	// Infrastructure for End Users (MySQL)
	var endUserStore memory.EndUserStore
	if mysqlDB != nil {
		eus := store.NewMySQLEndUserStore(mysqlDB)
		if err := eus.Init(); err != nil {
			log.Printf("Failed to init end_users table: %v", err)
		}
		endUserStore = eus
	}

	manager := memory.NewManager(cfg, vectorStore, redisStore, endUserStore, embedderClient, llmClient)

	// 4. Start Admin API
	if authService != nil {
		apiServer := api.NewServer(authService, manager)
		go func() {
			if err := apiServer.Start(":8080"); err != nil {
				log.Printf("API Server failed: %v", err)
			}
		}()
		fmt.Println(" Admin API Server started (background)")
	} else {
		log.Println("Warning: Auth service not available, Admin API will not start.")
	}

	// 5. Save LTM State (Only for In-Memory)
	if ims, ok := vectorStore.(*store.InMemoryVectorStore); ok {
		if err := ims.Save("memory.json"); err != nil {
			log.Printf("Failed to save LTM: %v", err)
		}
	}

	// Keep the server running
	if authService != nil {
		fmt.Println("Server is running at http://localhost:8080")
		select {} // Block forever
	} else {
		fmt.Println("Server finished (Admin API not started due to missing MySQL connection).")
	}
}
