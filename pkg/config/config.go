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
	ContextWindow     int // STM召回窗口大小
	MaxRecentMemories int // 召回记忆数量限制

	// STM配置
	STMWindowSize          int // STM滑动窗口大小
	STMMaxRetentionDays    int // STM最大保留天数
	STMExpirationDays      int // STM过期天数（0表示不过期）
	STMBatchJudgeSize      int // 批量判定大小
	STMJudgeMinMessages    int // 触发判定的最小消息数
	STMJudgeMaxWaitMinutes int // 触发判定的最大等待分钟数

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

	// Alert Configuration
	AlertCheckIntervalMinutes int
	AlertHistoryMaxSize       int

	// 注意：阈值和冷却时间已迁移到数据库管理
	// 通过 alert_rule_configs 表的 config_json 字段配置

	// 智能缓存检测配置
	AlertCacheWindowMinutes  int     // 统计窗口(分钟)
	AlertCacheMinSamples     int     // 最小样本数
	AlertCacheWarnThreshold  float64 // 警告阈值(百分比)
	AlertCacheErrorThreshold float64 // 错误阈值(百分比)
	AlertCacheTrendPeriods   int     // 趋势检测周期数

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
	maxRecent, _ := strconv.Atoi(getEnv("MAX_RECENT_MEMORIES", "100"))

	// 漏斗型配置
	stmWindowSize, _ := strconv.Atoi(getEnv("STM_WINDOW_SIZE", "100"))
	stmMaxRetentionDays, _ := strconv.Atoi(getEnv("STM_MAX_RETENTION_DAYS", "7"))
	stmExpirationDays, _ := strconv.Atoi(getEnv("STM_EXPIRATION_DAYS", "7"))
	stmBatchJudgeSize, _ := strconv.Atoi(getEnv("STM_BATCH_JUDGE_SIZE", "10"))
	stmJudgeMinMessages, _ := strconv.Atoi(getEnv("STM_JUDGE_MIN_MESSAGES", "5"))
	stmJudgeMaxWaitMinutes, _ := strconv.Atoi(getEnv("STM_JUDGE_MAX_WAIT_MINUTES", "60"))

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
	alertHistoryMaxSize, _ := strconv.Atoi(getEnv("ALERT_HISTORY_MAX_SIZE", "100"))

	// 智能缓存检测配置
	alertCacheWindowMinutes, _ := strconv.Atoi(getEnv("ALERT_CACHE_WINDOW_MINUTES", "5"))
	alertCacheMinSamples, _ := strconv.Atoi(getEnv("ALERT_CACHE_MIN_SAMPLES", "500"))
	alertCacheWarnThreshold, _ := strconv.ParseFloat(getEnv("ALERT_CACHE_WARN_THRESHOLD", "30"), 64)
	alertCacheErrorThreshold, _ := strconv.ParseFloat(getEnv("ALERT_CACHE_ERROR_THRESHOLD", "15"), 64)
	alertCacheTrendPeriods, _ := strconv.Atoi(getEnv("ALERT_CACHE_TREND_PERIODS", "3"))

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
		ContextWindow:        ctxWindow,
		MaxRecentMemories:    maxRecent,
		QdrantAddr:           getEnv("QDRANT_ADDR", "localhost"), // Client usually adds port, but let's verify usage
		QdrantCollection:     getEnv("QDRANT_COLLECTION", "ai_memory"),
		VectorStoreProvider:  getEnv("VECTOR_STORE_PROVIDER", "in_memory"),
		DBHost:               getEnv("DB_HOST", "localhost:3306"),
		DBUser:               getEnv("DB_USER", "root"),
		DBPass:               getEnv("DB_PASS", ""),
		DBName:               getEnv("DB_NAME", "ai_memory"),

		// 漏斗型配置
		STMWindowSize:          stmWindowSize,
		STMMaxRetentionDays:    stmMaxRetentionDays,
		STMExpirationDays:      stmExpirationDays,
		STMBatchJudgeSize:      stmBatchJudgeSize,
		STMJudgeMinMessages:    stmJudgeMinMessages,
		STMJudgeMaxWaitMinutes: stmJudgeMaxWaitMinutes,
		StagingMinOccurrences:  stagingMinOccurrences,
		StagingMinWaitHours:    stagingMinWaitHours,
		StagingValueThreshold:  stagingValueThreshold,
		StagingConfidenceHigh:  stagingConfidenceHigh,
		StagingConfidenceLow:   stagingConfidenceLow,
		LTMDecayHalfLifeDays:   ltmDecayHalfLifeDays,
		LTMDecayMinScore:       ltmDecayMinScore,
		JudgeModel:             getEnv("JUDGE_MODEL", "gpt-4o-mini"),
		ExtractTagsModel:       getEnv("EXTRACT_TAGS_MODEL", "gpt-4o"),

		// 监控系统配置
		MetricsPersistIntervalMinutes: metricsPersistInterval,
		MetricsHistoryLoadHours:       metricsHistoryLoadHours,
		MetricsMemoryRetentionHours:   metricsMemoryRetentionHours,
		MetricsRetentionDays:          metricsRetentionDays,
		AlertCheckIntervalMinutes:     alertCheckInterval,
		AlertHistoryMaxSize:           alertHistoryMaxSize,

		// 智能缓存检测配置
		AlertCacheWindowMinutes:  alertCacheWindowMinutes,
		AlertCacheMinSamples:     alertCacheMinSamples,
		AlertCacheWarnThreshold:  alertCacheWarnThreshold,
		AlertCacheErrorThreshold: alertCacheErrorThreshold,
		AlertCacheTrendPeriods:   alertCacheTrendPeriods,

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
