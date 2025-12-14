package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	OpenAIKey            string
	OpenAIBaseURL        string
	OpenAIModel          string
	OpenAIEmbeddingModel string
	LLMProvider          string
	SummarizePrompt      string
	ExtractProfilePrompt string
	QdrantAddr           string // ... existing fields
	QdrantCollection     string
	VectorStoreProvider  string
	ContextWindow        int // Number of messages to keep in STM
	MinSummaryItems      int // Items required to trigger summary
	MaxRecentMemories    int // Max memories to retrieve (Recall limit)

	// Database Configuration
	DBHost string
	DBUser string
	DBPass string
	DBName string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // Load .env if present, ignore error if missing

	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	ctxWindow, _ := strconv.Atoi(getEnv("STM_CONTEXT_WINDOW", "10"))
	minSummary, _ := strconv.Atoi(getEnv("MIN_SUMMARY_ITEMS", "5"))
	maxRecent, _ := strconv.Atoi(getEnv("MAX_RECENT_MEMORIES", "100"))

	return &Config{
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              db,
		OpenAIKey:            getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:        getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:          getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIEmbeddingModel: getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-ada-002"),
		LLMProvider:          getEnv("LLM_PROVIDER", "openai"),
		SummarizePrompt:      getEnv("SUMMARIZE_PROMPT", "Summarize the following conversation completely regarding key facts and user preferences. Ignore casual chitchat.\n\n%s"),
		ExtractProfilePrompt: getEnv("EXTRACT_PROFILE_PROMPT", "Analyze the following interaction. Identify any persistent user preferences, traits, or facts that should be remembered for future personalization. Return ONLY these facts as a bulleted list. If none, return 'None'.\n\n%s"),
		ContextWindow:        ctxWindow,
		MinSummaryItems:      minSummary,
		MaxRecentMemories:    maxRecent,
		QdrantAddr:           getEnv("QDRANT_ADDR", "localhost"), // Client usually adds port, but let's verify usage
		QdrantCollection:     getEnv("QDRANT_COLLECTION", "ai_memory"),
		VectorStoreProvider:  getEnv("VECTOR_STORE_PROVIDER", "in_memory"),
		DBHost:               getEnv("DB_HOST", "localhost:3306"),
		DBUser:               getEnv("DB_USER", "root"),
		DBPass:               getEnv("DB_PASS", ""),
		DBName:               getEnv("DB_NAME", "ai_memory"),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
