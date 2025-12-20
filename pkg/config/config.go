package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// LLM Provider
	OpenAIKey            string
	OpenAIBaseURL        string
	OpenAIModel          string
	OpenAIEmbeddingModel string
	LLMProvider          string

	// Vector Store
	QdrantAddr          string
	QdrantCollection    string
	VectorStoreProvider string

	// Database
	DBHost string
	DBUser string
	DBPass string
	DBName string

	// Legacy STM settings (仍在Retrieve中使用)
	ContextWindow        int    // STM召回窗口大小
	MinSummaryItems      int    // 触发Summary的最小条目数
	MaxRecentMemories    int    // 召回记忆数量限制
	SummarizePrompt      string // Summarize prompt模板
	ExtractProfilePrompt string // 实体提取prompt模板

	// STM配置
	STMWindowSize       int // STM滑动窗口大小
	STMMaxRetentionDays int // STM最大保留天数
	STMExpirationDays   int // STM过期天数（0表示不过期）
	STMBatchJudgeSize   int // 批量判定大小

	// Staging配置
	StagingMinOccurrences int     // Staging最小出现次数
	StagingMinWaitHours   int     // Staging最小等待时长(小时)
	StagingValueThreshold float64 // 价值分数阈值
	StagingConfidenceHigh float64 // 高信心阈值
	StagingConfidenceLow  float64 // 低信心阈值

	// LTM衰减配置
	LTMDecayHalfLifeDays int     // LTM衰减半衰期(天)
	LTMDecayMinScore     float64 // LTM删除阈值

	// LLM判定模型配置
	JudgeModel       string // LLM判定模型
	ExtractTagsModel string // 标签提取模型

	// 监控系统配置
	MetricsPersistIntervalMinutes int // 指标持久化频率(分钟)
	MetricsHistoryLoadHours       int // 启动时加载历史数据范围(小时)
	MetricsMemoryRetentionHours   int // 内存中保留数据时长(小时)
	MetricsRetentionDays          int // 数据库中保留历史数据天数

	// 告警引擎配置
	AlertCheckIntervalMinutes        int     // 告警检查频率(分钟)
	AlertQueueBacklogThreshold       int     // 队列积压告警阈值
	AlertQueueBacklogCooldownMinutes int     // 队列告警冷却时间(分钟)
	AlertSuccessRateThreshold        float64 // 成功率告警阈值(百分比)
	AlertSuccessRateCooldownMinutes  int     // 成功率告警冷却时间(分钟)
	AlertCacheHitRateThreshold       float64 // 缓存命中率告警阈值(百分比)
	AlertCacheHitRateCooldownMinutes int     // 缓存告警冷却时间(分钟)
	AlertDecaySpikeThreshold         int     // 衰减突增告警阈值
	AlertDecaySpikeCooldownMinutes   int     // 衰减告警冷却时间(分钟)
	AlertHistoryMaxSize              int     // 保留的告警历史记录数量

	// 告警通知配置
	AlertWebhookEnabled bool   // 是否启用Webhook通知
	AlertWebhookURL     string // Webhook URL
	AlertWebhookTimeout int    // Webhook超时时间(秒)
	AlertEmailEnabled   bool   // 是否启用邮件通知
	AlertEmailSMTPHost  string // SMTP服务器
	AlertEmailSMTPPort  int    // SMTP端口
	AlertEmailUsername  string // SMTP用户名
	AlertEmailPassword  string // SMTP密码
	AlertEmailFrom      string // 发件人
	AlertEmailTo        string // 收件人(逗号分隔)
	AlertEmailUseTLS    bool   // 是否使用TLS
	AlertNotifyLevels   string // 需要通知的告警级别

	// 日志配置
	LogDir string // 日志目录，默认 "log"
}

func Load() (*Config, error) {
	_ = godotenv.Load() // Load .env if present, ignore error if missing

	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	ctxWindow, _ := strconv.Atoi(getEnv("STM_CONTEXT_WINDOW", "10"))
	minSummary, _ := strconv.Atoi(getEnv("MIN_SUMMARY_ITEMS", "5"))
	maxRecent, _ := strconv.Atoi(getEnv("MAX_RECENT_MEMORIES", "100"))

	// 漏斗型配置
	stmWindowSize, _ := strconv.Atoi(getEnv("STM_WINDOW_SIZE", "100"))
	stmMaxRetentionDays, _ := strconv.Atoi(getEnv("STM_MAX_RETENTION_DAYS", "7"))
	stmExpirationDays, _ := strconv.Atoi(getEnv("STM_EXPIRATION_DAYS", "7"))
	stmBatchJudgeSize, _ := strconv.Atoi(getEnv("STM_BATCH_JUDGE_SIZE", "10"))

	stagingMinOccurrences, _ := strconv.Atoi(getEnv("STAGING_MIN_OCCURRENCES", "2"))
	stagingMinWaitHours, _ := strconv.Atoi(getEnv("STAGING_MIN_WAIT_HOURS", "48"))
	stagingValueThreshold, _ := strconv.ParseFloat(getEnv("STAGING_VALUE_THRESHOLD", "0.6"), 64)
	stagingConfidenceHigh, _ := strconv.ParseFloat(getEnv("STAGING_CONFIDENCE_HIGH", "0.8"), 64)
	stagingConfidenceLow, _ := strconv.ParseFloat(getEnv("STAGING_CONFIDENCE_LOW", "0.5"), 64)

	ltmDecayHalfLifeDays, _ := strconv.Atoi(getEnv("LTM_DECAY_HALF_LIFE_DAYS", "90"))
	ltmDecayMinScore, _ := strconv.ParseFloat(getEnv("LTM_DECAY_MIN_SCORE", "0.3"), 64)

	// 监控系统配置
	metricsPersistInterval, _ := strconv.Atoi(getEnv("METRICS_PERSIST_INTERVAL_MINUTES", "1"))
	metricsHistoryLoadHours, _ := strconv.Atoi(getEnv("METRICS_HISTORY_LOAD_HOURS", "24"))
	metricsMemoryRetentionHours, _ := strconv.Atoi(getEnv("METRICS_MEMORY_RETENTION_HOURS", "1"))
	metricsRetentionDays, _ := strconv.Atoi(getEnv("METRICS_RETENTION_DAYS", "30"))

	alertCheckInterval, _ := strconv.Atoi(getEnv("ALERT_CHECK_INTERVAL_MINUTES", "1"))
	alertQueueThreshold, _ := strconv.Atoi(getEnv("ALERT_QUEUE_BACKLOG_THRESHOLD", "100"))
	alertQueueCooldown, _ := strconv.Atoi(getEnv("ALERT_QUEUE_BACKLOG_COOLDOWN_MINUTES", "10"))
	alertSuccessRateThreshold, _ := strconv.ParseFloat(getEnv("ALERT_SUCCESS_RATE_THRESHOLD", "60"), 64)
	alertSuccessRateCooldown, _ := strconv.Atoi(getEnv("ALERT_SUCCESS_RATE_COOLDOWN_MINUTES", "30"))
	alertCacheHitRateThreshold, _ := strconv.ParseFloat(getEnv("ALERT_CACHE_HIT_RATE_THRESHOLD", "20"), 64)
	alertCacheHitRateCooldown, _ := strconv.Atoi(getEnv("ALERT_CACHE_HIT_RATE_COOLDOWN_MINUTES", "15"))
	alertDecaySpikeThreshold, _ := strconv.Atoi(getEnv("ALERT_DECAY_SPIKE_THRESHOLD", "1000"))
	alertDecaySpikeCooldown, _ := strconv.Atoi(getEnv("ALERT_DECAY_SPIKE_COOLDOWN_MINUTES", "60"))
	alertHistoryMaxSize, _ := strconv.Atoi(getEnv("ALERT_HISTORY_MAX_SIZE", "100"))

	// 告警通知配置
	alertWebhookEnabled, _ := strconv.ParseBool(getEnv("ALERT_WEBHOOK_ENABLED", "false"))
	alertWebhookTimeout, _ := strconv.Atoi(getEnv("ALERT_WEBHOOK_TIMEOUT_SECONDS", "5"))
	alertEmailEnabled, _ := strconv.ParseBool(getEnv("ALERT_EMAIL_ENABLED", "false"))
	alertEmailSMTPPort, _ := strconv.Atoi(getEnv("ALERT_EMAIL_SMTP_PORT", "587"))
	alertEmailUseTLS, _ := strconv.ParseBool(getEnv("ALERT_EMAIL_USE_TLS", "true"))

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

		// 漏斗型配置
		STMWindowSize:         stmWindowSize,
		STMMaxRetentionDays:   stmMaxRetentionDays,
		STMExpirationDays:     stmExpirationDays,
		STMBatchJudgeSize:     stmBatchJudgeSize,
		StagingMinOccurrences: stagingMinOccurrences,
		StagingMinWaitHours:   stagingMinWaitHours,
		StagingValueThreshold: stagingValueThreshold,
		StagingConfidenceHigh: stagingConfidenceHigh,
		StagingConfidenceLow:  stagingConfidenceLow,
		LTMDecayHalfLifeDays:  ltmDecayHalfLifeDays,
		LTMDecayMinScore:      ltmDecayMinScore,
		JudgeModel:            getEnv("JUDGE_MODEL", "gpt-4o-mini"),
		ExtractTagsModel:      getEnv("EXTRACT_TAGS_MODEL", "gpt-4o"),

		// 监控系统配置
		MetricsPersistIntervalMinutes:    metricsPersistInterval,
		MetricsHistoryLoadHours:          metricsHistoryLoadHours,
		MetricsMemoryRetentionHours:      metricsMemoryRetentionHours,
		MetricsRetentionDays:             metricsRetentionDays,
		AlertCheckIntervalMinutes:        alertCheckInterval,
		AlertQueueBacklogThreshold:       alertQueueThreshold,
		AlertQueueBacklogCooldownMinutes: alertQueueCooldown,
		AlertSuccessRateThreshold:        alertSuccessRateThreshold,
		AlertSuccessRateCooldownMinutes:  alertSuccessRateCooldown,
		AlertCacheHitRateThreshold:       alertCacheHitRateThreshold,
		AlertCacheHitRateCooldownMinutes: alertCacheHitRateCooldown,
		AlertDecaySpikeThreshold:         alertDecaySpikeThreshold,
		AlertDecaySpikeCooldownMinutes:   alertDecaySpikeCooldown,
		AlertHistoryMaxSize:              alertHistoryMaxSize,

		// 告警通知配置
		AlertWebhookEnabled: alertWebhookEnabled,
		AlertWebhookURL:     getEnv("ALERT_WEBHOOK_URL", ""),
		AlertWebhookTimeout: alertWebhookTimeout,
		AlertEmailEnabled:   alertEmailEnabled,
		AlertEmailSMTPHost:  getEnv("ALERT_EMAIL_SMTP_HOST", "smtp.example.com"),
		AlertEmailSMTPPort:  alertEmailSMTPPort,
		AlertEmailUsername:  getEnv("ALERT_EMAIL_USERNAME", ""),
		AlertEmailPassword:  getEnv("ALERT_EMAIL_PASSWORD", ""),
		AlertEmailFrom:      getEnv("ALERT_EMAIL_FROM", "AI-Memory Alert <alert@example.com>"),
		AlertEmailTo:        getEnv("ALERT_EMAIL_TO", ""),
		AlertEmailUseTLS:    alertEmailUseTLS,
		AlertNotifyLevels:   getEnv("ALERT_NOTIFY_LEVELS", "ERROR,WARNING"),

		LogDir: getEnv("LOG_DIR", "log"),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
