package memory

import (
	"ai-memory/pkg/logger"
	"ai-memory/pkg/store"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "INFO"
	AlertLevelWarning AlertLevel = "WARNING"
	AlertLevelError   AlertLevel = "ERROR"
)

// Alert 告警事件
type Alert struct {
	ID        string                 `json:"id"`
	Level     AlertLevel             `json:"level"`
	Rule      string                 `json:"rule"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string
	Name        string
	Description string
	CheckFunc   func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert
	Enabled     bool
	Cooldown    time.Duration

	// 并发安全的状态管理
	mu        sync.Mutex
	lastFired time.Time
}

// ShouldFire 检查是否应该触发告警（线程安全）
func (r *AlertRule) ShouldFire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Since(r.lastFired) >= r.Cooldown
}

// MarkFired 标记告警已触发（线程安全）
func (r *AlertRule) MarkFired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFired = time.Now()
}

// AlertEngine 告警引擎
type AlertEngine struct {
	mu               sync.RWMutex
	rules            []*AlertRule
	recentAlerts     []Alert // 内存热缓存（只读）
	maxRecentAlerts  int
	checkInterval    time.Duration
	notifyFunc       func(alert *Alert)
	metricsCollector *MetricsCollector
	stagingStore     *store.StagingStore
	stopChan         chan struct{}

	// 存储层（依赖注入）
	repository AlertRepository

	// 缓存告警智能化配置
	cacheCheckConfig *CacheCheckConfig

	// 统计信息
	stats *AlertEngineStats

	// 规则配置持久化
	configPersistence *RuleConfigPersistence

	// 统计信息缓存
	statsCache *StatsCache

	// 统计数据持久化
	statsPersistence *AlertStatsPersistence

	// 统计同步管理器
	statsSync *AlertEngineStatsSync
}

// AlertEngineStats 告警引擎统计
type AlertEngineStats struct {
	mu            sync.RWMutex
	TotalChecks   int64
	TotalFired    int64
	NotifySuccess int64
	NotifyFailed  int64
	RuleStats     map[string]*RuleStats
}

// RuleStats 规则统计
type RuleStats struct {
	mu               sync.RWMutex
	TotalFired       int64
	LastFiredAt      time.Time
	TotalChecks      int64
	AvgCheckDuration time.Duration
}

// CacheCheckConfig 缓存告警智能检测配置
type CacheCheckConfig struct {
	WindowMinutes  int     // 统计窗口（分钟）
	MinSamples     int     // 最小样本数
	WarnThreshold  float64 // 警告阈值（百分比）
	ErrorThreshold float64 // 错误阈值（百分比）
	TrendPeriods   int     // 趋势检测周期数
}

// AlertConfig 告警引擎配置
type AlertConfig struct {
	CheckIntervalMinutes        int
	QueueBacklogThreshold       int
	QueueBacklogCooldownMinutes int
	SuccessRateThreshold        float64
	SuccessRateCooldownMinutes  int
	CacheHitRateThreshold       float64
	CacheHitRateCooldownMinutes int
	DecaySpikeThreshold         int
	DecaySpikeCooldownMinutes   int
	HistoryMaxSize              int

	// 智能缓存检测配置
	CacheWindowMinutes  int
	CacheMinSamples     int
	CacheWarnThreshold  float64
	CacheErrorThreshold float64
	CacheTrendPeriods   int
}

// NewAlertEngine 创建告警引擎
func NewAlertEngine(repository AlertRepository, collector *MetricsCollector, stagingStore *store.StagingStore, config *AlertConfig) *AlertEngine {
	engine := &AlertEngine{
		rules:            make([]*AlertRule, 0),
		recentAlerts:     make([]Alert, 0, config.HistoryMaxSize),
		maxRecentAlerts:  config.HistoryMaxSize,
		checkInterval:    time.Duration(config.CheckIntervalMinutes) * time.Minute,
		metricsCollector: collector,
		stagingStore:     stagingStore,
		stopChan:         make(chan struct{}),
		repository:       repository,
		cacheCheckConfig: &CacheCheckConfig{
			WindowMinutes:  config.CacheWindowMinutes,
			MinSamples:     config.CacheMinSamples,
			WarnThreshold:  config.CacheWarnThreshold,
			ErrorThreshold: config.CacheErrorThreshold,
			TrendPeriods:   config.CacheTrendPeriods,
		},
	}

	// 初始化统计
	engine.stats = &AlertEngineStats{
		RuleStats: make(map[string]*RuleStats),
	}

	// 初始化统计缓存（30秒TTL）
	engine.statsCache = NewStatsCache(30 * time.Second)

	// 注册默认规则
	engine.registerDefaultRules(config)

	return engine
}

// registerDefaultRules 注册默认告警规则
func (ae *AlertEngine) registerDefaultRules(config *AlertConfig) {
	// 规则1: 队列积压告警
	ae.AddRule(&AlertRule{
		ID:          "queue_backlog",
		Name:        "队列积压告警",
		Description: "Staging队列长度超过阈值",
		CheckFunc:   ae.makeQueueBacklogCheck(config.QueueBacklogThreshold),
		Enabled:     true,
		Cooldown:    time.Duration(config.QueueBacklogCooldownMinutes) * time.Minute,
	})

	// 规则2: 晋升成功率下降
	ae.AddRule(&AlertRule{
		ID:          "low_success_rate",
		Name:        "晋升成功率过低",
		Description: "晋升成功率低于阈值",
		CheckFunc:   ae.makeLowSuccessRateCheck(config.SuccessRateThreshold),
		Enabled:     true,
		Cooldown:    time.Duration(config.SuccessRateCooldownMinutes) * time.Minute,
	})

	// 规则3: 缓存异常（智能检测）
	ae.AddRule(&AlertRule{
		ID:          "cache_anomaly",
		Name:        "缓存命中率异常",
		Description: "缓存命中率异常低或突降",
		CheckFunc:   ae.makeCacheAnomalyCheckSmart(), // 使用智能检测
		Enabled:     true,
		Cooldown:    time.Duration(config.CacheHitRateCooldownMinutes) * time.Minute,
	})

	// 规则4: 记忆衰减异常
	ae.AddRule(&AlertRule{
		ID:          "decay_spike",
		Name:        "记忆衰减突增",
		Description: "遗忘数量突增",
		CheckFunc:   ae.makeDecaySpikeCheck(config.DecaySpikeThreshold),
		Enabled:     true,
		Cooldown:    time.Duration(config.DecaySpikeCooldownMinutes) * time.Minute,
	})
}

// AddRule 添加自定义规则
func (ae *AlertEngine) AddRule(rule *AlertRule) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.rules = append(ae.rules, rule)
}

// Start 启动告警引擎
func (ae *AlertEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(ae.checkInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				ae.checkAllRules(ctx)
			case <-ae.stopChan:
				ticker.Stop()
				logger.System("Alert engine stopped")
				return
			}
		}
	}()

	logger.System("✅ Alert engine started", "interval", ae.checkInterval, "rules", len(ae.rules))
}

// Stop 停止告警引擎
func (ae *AlertEngine) Stop() {
	// 停止统计同步（会自动刷新）
	if ae.statsSync != nil {
		ae.statsSync.Stop()
	}
	close(ae.stopChan)
}

// checkAllRules 检查所有规则
func (ae *AlertEngine) checkAllRules(ctx context.Context) {
	ae.mu.RLock()
	rules := make([]*AlertRule, len(ae.rules))
	copy(rules, ae.rules)
	ae.mu.RUnlock()

	// 注意：TotalChecks 会在每个规则的 recordRuleCheck 中累积
	// 不在这里重复统计

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 使用线程安全的冷却检查
		if !rule.ShouldFire() {
			continue
		}

		// 记录执行时间
		startTime := time.Now()

		// 执行规则检查
		alert := rule.CheckFunc(ctx, ae.metricsCollector, ae.stagingStore)

		duration := time.Since(startTime)
		ae.recordRuleCheck(rule.ID, duration) // 这里会累积TotalChecks

		if alert != nil {
			ae.fireAlert(ctx, alert)
			ae.recordRuleFire(rule.ID)
			rule.MarkFired()
		}
	}
}

// fireAlert 触发告警
func (ae *AlertEngine) fireAlert(ctx context.Context, alert *Alert) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	// 告警聚合
	aggregateAlert(alert)

	// 添加到内存热缓存
	ae.recentAlerts = append(ae.recentAlerts, *alert)
	if len(ae.recentAlerts) > ae.maxRecentAlerts {
		ae.recentAlerts = ae.recentAlerts[len(ae.recentAlerts)-ae.maxRecentAlerts:]
	}

	// 日志记录
	logger.System("🚨 ALERT FIRED",
		"id", alert.ID,
		"level", alert.Level,
		"rule", alert.Rule,
		"message", alert.Message,
		"timestamp", alert.Timestamp)

	// 持久化到数据库（通过存储层）
	if ae.repository != nil {
		if err := ae.repository.Save(ctx, alert); err != nil {
			logger.Error("Failed to persist alert", err)
			ae.recordNotifyResult(false)
		} else {
			ae.recordNotifyResult(true)
			// 使统计缓存失效
			ae.InvalidateStatsCache()
		}
	}

	// 调用通知函数
	if ae.notifyFunc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic in notify function", fmt.Errorf("%v", r))
					ae.recordNotifyResult(false)
				}
			}()
			ae.notifyFunc(alert)
			ae.recordNotifyResult(true)
		}()
	}
}

// SetNotifyFunc 设置通知回调函数
func (ae *AlertEngine) SetNotifyFunc(f func(alert *Alert)) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.notifyFunc = f
}

// GetRecentAlerts 获取最近的告警记录（从内存热缓存）
func (ae *AlertEngine) GetRecentAlerts(limit int) []Alert {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	if limit <= 0 || limit > len(ae.recentAlerts) {
		limit = len(ae.recentAlerts)
	}

	// 返回最近的N条（倒序）
	result := make([]Alert, limit)
	start := len(ae.recentAlerts) - limit
	copy(result, ae.recentAlerts[start:])

	// 反转顺序（最新的在前）
	for i := 0; i < len(result)/2; i++ {
		j := len(result) - 1 - i
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// QueryAlerts 查询告警（完整数据库查询）
func (ae *AlertEngine) QueryAlerts(ctx context.Context, level, rule string, limit, offset int) ([]Alert, int, error) {
	if ae.repository == nil {
		return nil, 0, fmt.Errorf("alert repository not initialized")
	}
	return ae.repository.QueryFiltered(ctx, level, rule, limit, offset)
}

// DeleteAlert 删除告警
func (ae *AlertEngine) DeleteAlert(ctx context.Context, id string) error {
	// 从数据库删除
	if ae.repository != nil {
		if err := ae.repository.Delete(ctx, id); err != nil {
			return err
		}
	}

	// 从内存缓存删除
	ae.mu.Lock()
	defer ae.mu.Unlock()
	newAlerts := make([]Alert, 0, len(ae.recentAlerts))
	for _, a := range ae.recentAlerts {
		if a.ID != id {
			newAlerts = append(newAlerts, a)
		}
	}
	ae.recentAlerts = newAlerts
	return nil
}

// CreateAlert 手动创建告警
func (ae *AlertEngine) CreateAlert(ctx context.Context, alert Alert) error {
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}
	ae.fireAlert(ctx, &alert)
	return nil
}

// ========== 告警规则实现（使用闭包支持配置化）==========

// makeQueueBacklogCheck 创建队列积压检查函数（动态读取配置）
func (ae *AlertEngine) makeQueueBacklogCheck(defaultThreshold int) func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		// 动态读取阈值配置
		threshold := defaultThreshold
		if ae.configPersistence != nil {
			configs, err := ae.configPersistence.LoadAll(ctx)
			if err == nil {
				if config, ok := configs["queue_backlog"]; ok && config.ConfigJSON != "" {
					var jsonConfig map[string]interface{}
					if json.Unmarshal([]byte(config.ConfigJSON), &jsonConfig) == nil {
						if t, ok := jsonConfig["threshold"].(float64); ok {
							threshold = int(t)
						}
					}
				}
			}
		}

		entries, err := stagingStore.GetPendingEntries(ctx, 1, 0)
		if err != nil {
			return nil
		}

		queueLength := len(entries)
		if queueLength > threshold {
			return &Alert{
				ID:        fmt.Sprintf("queue_backlog_%s", uuid.New().String()[:8]),
				Level:     AlertLevelWarning,
				Rule:      "queue_backlog",
				Message:   "Staging队列积压过多，请检查晋升逻辑",
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"queue_length": queueLength,
					"threshold":    threshold,
				},
			}
		}

		return nil
	}
}

// makeLowSuccessRateCheck 创建成功率检查函数
func (ae *AlertEngine) makeLowSuccessRateCheck(threshold float64) func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		metrics.mu.RLock()
		totalAttempts := metrics.TotalPromotions + metrics.TotalRejections
		promotions := metrics.TotalPromotions
		metrics.mu.RUnlock()

		if totalAttempts < 10 {
			return nil // 样本太少，不告警
		}

		successRate := float64(promotions) / float64(totalAttempts) * 100
		if successRate < threshold {
			return &Alert{
				ID:        fmt.Sprintf("low_success_rate_%s", uuid.New().String()[:8]),
				Level:     AlertLevelWarning,
				Rule:      "low_success_rate",
				Message:   "记忆晋升成功率过低，可能是判定标准过严",
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"success_rate": successRate,
					"threshold":    threshold,
					"attempts":     totalAttempts,
				},
			}
		}

		return nil
	}
}

// makeCacheAnomalyCheckSmart 创建智能缓存异常检查函数
func (ae *AlertEngine) makeCacheAnomalyCheckSmart() func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	// 存储历史命中率（用于趋势检测）
	var historyRates []float64
	var historyMu sync.Mutex

	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		metrics.mu.RLock()
		totalAccess := metrics.CacheHits + metrics.CacheMisses
		hits := metrics.CacheHits
		metrics.mu.RUnlock()

		// 检查1: 最小样本数（避免冷启动和低流量误报）
		if totalAccess < int64(ae.cacheCheckConfig.MinSamples) {
			return nil
		}

		// 计算当前命中率
		currentRate := float64(hits) / float64(totalAccess) * 100

		// 检查2: 分段阈值检测
		var level AlertLevel
		var triggered bool

		if currentRate < ae.cacheCheckConfig.ErrorThreshold {
			level = AlertLevelError
			triggered = true
		} else if currentRate < ae.cacheCheckConfig.WarnThreshold {
			level = AlertLevelWarning
			triggered = true
		}

		// 检查3: 趋势检测（突降检测）
		historyMu.Lock()
		historyRates = append(historyRates, currentRate)
		if len(historyRates) > ae.cacheCheckConfig.TrendPeriods {
			historyRates = historyRates[1:]
		}

		// 如果历史数据足够，检查是否突降
		trendAlert := false
		if len(historyRates) >= ae.cacheCheckConfig.TrendPeriods {
			// 计算历史平均值
			var sum float64
			for i := 0; i < len(historyRates)-1; i++ {
				sum += historyRates[i]
			}
			avgRate := sum / float64(len(historyRates)-1)

			// 如果当前值比平均值低20%以上，视为突降
			if avgRate-currentRate > 20.0 {
				trendAlert = true
				triggered = true
				if level == "" {
					level = AlertLevelWarning
				}
			}
		}
		historyMu.Unlock()

		// 触发告警
		if triggered {
			message := "缓存命中率异常低"
			if trendAlert {
				message = "缓存命中率突降，可能LLM判定逻辑故障"
			}

			return &Alert{
				ID:        fmt.Sprintf("cache_anomaly_%s", uuid.New().String()[:8]),
				Level:     level,
				Rule:      "cache_anomaly",
				Message:   message,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"hit_rate":        currentRate,
					"warn_threshold":  ae.cacheCheckConfig.WarnThreshold,
					"error_threshold": ae.cacheCheckConfig.ErrorThreshold,
					"total_access":    totalAccess,
					"trend_detected":  trendAlert,
				},
			}
		}

		return nil
	}
}

// makeDecaySpikeCheck 创建衰减突增检查函数
func (ae *AlertEngine) makeDecaySpikeCheck(threshold int) func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		metrics.mu.RLock()
		forgotten := metrics.TotalForgotten
		metrics.mu.RUnlock()

		if forgotten > int64(threshold) {
			return &Alert{
				ID:        fmt.Sprintf("decay_spike_%s", uuid.New().String()[:8]),
				Level:     AlertLevelInfo,
				Rule:      "decay_spike",
				Message:   "记忆遗忘数量较高，这可能是正常的衰减",
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"forgotten": forgotten,
					"threshold": threshold,
				},
			}
		}

		return nil
	}
}
