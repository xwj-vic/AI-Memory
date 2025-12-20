package main

import (
	"context"
	"strings"
	"time"

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

	// 初始化监控指标持久化
	if mysqlDB != nil {
		metricsPersistence := memory.NewMetricsPersistence(mysqlDB, cfg.MetricsPersistIntervalMinutes)

		// 启动时加载累计统计
		if err := metricsPersistence.LoadCumulativeStats(ctx, memory.GetGlobalMetrics()); err != nil {
			logger.Error("Failed to load cumulative stats, starting fresh", err)
		} else {
			logger.System("✅ Loaded metrics from database")
		}

		// 加载历史时间序列数据（使用配置的小时数）
		if err := metricsPersistence.LoadRecentTimeSeries(ctx, memory.GetGlobalMetrics(), cfg.MetricsHistoryLoadHours); err != nil {
			logger.Error("Failed to load timeseries data", err)
		}

		// 启动定时持久化任务（含自动清理）
		metricsPersistence.StartWithCleanup(memory.GetGlobalMetrics(), cfg.MetricsRetentionDays)
		defer metricsPersistence.Stop()
	}

	// 初始化告警通知器
	if cfg.AlertWebhookEnabled || cfg.AlertEmailEnabled {
		// 解析需要通知的告警级别
		notifyLevels := make(map[memory.AlertLevel]bool)
		for _, level := range strings.Split(cfg.AlertNotifyLevels, ",") {
			level = strings.TrimSpace(level)
			switch level {
			case "ERROR":
				notifyLevels[memory.AlertLevelError] = true
			case "WARNING":
				notifyLevels[memory.AlertLevelWarning] = true
			case "INFO":
				notifyLevels[memory.AlertLevelInfo] = true
			}
		}

		// 解析收件人列表
		var emailTo []string
		if cfg.AlertEmailTo != "" {
			for _, email := range strings.Split(cfg.AlertEmailTo, ",") {
				emailTo = append(emailTo, strings.TrimSpace(email))
			}
		}

		notifyConfig := &memory.NotifyConfig{
			WebhookEnabled: cfg.AlertWebhookEnabled,
			WebhookURL:     cfg.AlertWebhookURL,
			WebhookTimeout: time.Duration(cfg.AlertWebhookTimeout) * time.Second,
			EmailEnabled:   cfg.AlertEmailEnabled,
			EmailSMTPHost:  cfg.AlertEmailSMTPHost,
			EmailSMTPPort:  cfg.AlertEmailSMTPPort,
			EmailUsername:  cfg.AlertEmailUsername,
			EmailPassword:  cfg.AlertEmailPassword,
			EmailFrom:      cfg.AlertEmailFrom,
			EmailTo:        emailTo,
			EmailUseTLS:    cfg.AlertEmailUseTLS,
			NotifyLevels:   notifyLevels,
		}

		notifier := memory.NewAlertNotifier(notifyConfig)
		memoryManager.SetAlertNotifier(notifier)
		logger.System("✅ Alert notifier initialized",
			"webhook", cfg.AlertWebhookEnabled,
			"email", cfg.AlertEmailEnabled)
	}

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
