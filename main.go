package main

import (
	"context"

	"ai-memory/pkg/api"
	"ai-memory/pkg/auth"
	"ai-memory/pkg/config"
	"ai-memory/pkg/logger"

	"ai-memory/pkg/llm"

	"ai-memory/pkg/memory"
	"ai-memory/pkg/store"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Warning loading config", err)
	}

	// 初始化Logger（必须在配置加载后立即执行）
	if err := logger.Init(cfg); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Shutdown() // 确保程序退出时关闭日志文件

	ctx := context.Background()

	// 2. Initialize Infrastructure
	// Redis (STM)
	redisStore := store.NewRedisStore(cfg)
	if err := redisStore.Ping(ctx); err != nil {
		logger.Error("Warning: Redis connection failed. Ensure Redis is running", err)
	} else {
		logger.System("Connected to Redis (STM)")
	}

	// MySQL (Auth & Admin)
	mysqlDB, err := store.NewMySQLStore(cfg)
	if err != nil {
		logger.Error("Warning: MySQL connection failed", err)
	} else {
		logger.System("Connected to MySQL")
	}

	// Initialize Auth Service
	var authService *auth.Service
	if mysqlDB != nil {
		authService = auth.NewService(mysqlDB)
		if err := authService.InitSchema(); err != nil {
			logger.Error("Warning: Failed to init auth schema", err)
		} else {
			// Check if we need to seed a default admin?
			// For now just logging.
			logger.System("Auth Schema Initialized")

			// Seed Default Admin
			if err := authService.CreateUser("admin", "admin123"); err == nil {
				logger.System("Default Admin Created: admin / admin123")
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
		logger.System("Unknown provider, defaulting to OpenAI", "provider", cfg.LLMProvider)
		client := llm.NewOpenAIClient(cfg)
		llmClient = client
		embedderClient = client
	}

	// 3. Initialize Vector Store (Only Qdrant supported now)
	var vectorStore memory.VectorStore

	// 强制使用 Qdrant
	logger.System("Initializing Qdrant Vector Store", "addr", cfg.QdrantAddr, "collection", cfg.QdrantCollection)
	qs, err := store.NewQdrantStore(cfg)
	if err != nil {
		logger.Error("Failed to initialize Qdrant", err)
		panic(err)
	}
	// Ensure collection exists. Vector size 1024 for BAAI/bge-m3
	if err := qs.Init(ctx, 1024); err != nil {
		logger.Error("Failed to init Qdrant collection", err)
		panic(err)
	}
	vectorStore = qs
	logger.System("Connected to Qdrant (LTM)")

	// 3. Create Manager
	// Infrastructure for End Users (MySQL)
	var endUserStore memory.EndUserStore
	if mysqlDB != nil {
		eus := store.NewMySQLEndUserStore(mysqlDB)
		if err := eus.Init(); err != nil {
			logger.Error("Failed to init end_users table", err)
		}
		endUserStore = eus
	}

	memoryManager := memory.NewManager(cfg, vectorStore, redisStore, endUserStore, embedderClient, llmClient, redisStore)

	// 4. Start Admin API
	if authService != nil {
		apiServer := api.NewServer(authService, memoryManager)
		go func() {
			if err := apiServer.Start(":8080"); err != nil {
				logger.Error("API Server failed", err)
			}
		}()
		logger.System("Admin API Server started (background)")
	} else {
		logger.System("Warning: Auth service not available, Admin API will not start.")
	}

	// 5. Save LTM State (PERSISTENCE REMOVED - Qdrant handles this)

	// Keep the server running
	if authService != nil {
		logger.System("Server is running at http://localhost:8080")
		select {} // Block forever
	} else {
		logger.System("Server finished (Admin API not started due to missing MySQL connection).")
	}
}
