package memory

import (
	"ai-memory/pkg/logger"
	"ai-memory/pkg/store"
	"context"
	"sync"
	"time"
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
	Cooldown    time.Duration // 冷却时间，避免告警风暴
	lastFired   time.Time
}

// AlertEngine 告警引擎
type AlertEngine struct {
	mu               sync.RWMutex
	rules            []*AlertRule
	recentAlerts     []Alert
	maxRecentAlerts  int
	checkInterval    time.Duration
	notifyFunc       func(alert *Alert) // 通知回调函数
	metricsCollector *MetricsCollector
	stagingStore     *store.StagingStore
	stopChan         chan struct{}
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
}

// NewAlertEngine 创建告警引擎
func NewAlertEngine(collector *MetricsCollector, stagingStore *store.StagingStore, config *AlertConfig) *AlertEngine {
	engine := &AlertEngine{
		rules:            make([]*AlertRule, 0),
		recentAlerts:     make([]Alert, 0, config.HistoryMaxSize),
		maxRecentAlerts:  config.HistoryMaxSize,
		checkInterval:    time.Duration(config.CheckIntervalMinutes) * time.Minute,
		metricsCollector: collector,
		stagingStore:     stagingStore,
		stopChan:         make(chan struct{}),
	}

	// 注册默认规则（使用配置参数）
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

	// 规则3: 缓存异常
	ae.AddRule(&AlertRule{
		ID:          "cache_anomaly",
		Name:        "缓存命中率异常",
		Description: "缓存命中率异常低",
		CheckFunc:   ae.makeCacheAnomalyCheck(config.CacheHitRateThreshold),
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
	close(ae.stopChan)
}

// checkAllRules 检查所有规则
func (ae *AlertEngine) checkAllRules(ctx context.Context) {
	ae.mu.RLock()
	rules := make([]*AlertRule, len(ae.rules))
	copy(rules, ae.rules)
	ae.mu.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 检查冷却时间
		if time.Since(rule.lastFired) < rule.Cooldown {
			continue
		}

		// 执行规则检查
		if alert := rule.CheckFunc(ctx, ae.metricsCollector, ae.stagingStore); alert != nil {
			ae.fireAlert(alert)
			rule.lastFired = time.Now()
		}
	}
}

// fireAlert 触发告警
func (ae *AlertEngine) fireAlert(alert *Alert) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	// 添加到历史记录
	ae.recentAlerts = append(ae.recentAlerts, *alert)
	if len(ae.recentAlerts) > ae.maxRecentAlerts {
		ae.recentAlerts = ae.recentAlerts[len(ae.recentAlerts)-ae.maxRecentAlerts:]
	}

	// 日志记录
	logger.System("🚨 ALERT FIRED",
		"level", alert.Level,
		"rule", alert.Rule,
		"message", alert.Message)

	// 调用通知函数
	if ae.notifyFunc != nil {
		ae.notifyFunc(alert)
	}
}

// SetNotifyFunc 设置通知回调函数
func (ae *AlertEngine) SetNotifyFunc(f func(alert *Alert)) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.notifyFunc = f
}

// GetRecentAlerts 获取最近的告警记录
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

// ========== 告警规则实现（使用闭包支持配置化）==========

// makeQueueBacklogCheck 创建队列积压检查函数
func (ae *AlertEngine) makeQueueBacklogCheck(threshold int) func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		entries, err := stagingStore.GetPendingEntries(ctx, 1, 0)
		if err != nil {
			return nil
		}

		queueLength := len(entries)
		if queueLength > threshold {
			return &Alert{
				ID:        "queue_backlog_" + time.Now().Format("20060102150405"),
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

// checkLowSuccessRate 检查晋升成功率
func (ae *AlertEngine) checkLowSuccessRate(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	metrics.mu.RLock()
	totalAttempts := metrics.TotalPromotions + metrics.TotalRejections
	promotions := metrics.TotalPromotions
	metrics.mu.RUnlock()

	if totalAttempts < 10 {
		return nil // 样本太少，不告警
	}

	successRate := float64(promotions) / float64(totalAttempts) * 100
	if successRate < 60 {
		return &Alert{
			ID:        "low_success_rate_" + time.Now().Format("20060102150405"),
			Level:     AlertLevelWarning,
			Rule:      "low_success_rate",
			Message:   "记忆晋升成功率过低，可能是判定标准过严",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"success_rate": successRate,
				"threshold":    60.0,
				"attempts":     totalAttempts,
			},
		}
	}

	return nil
}

// checkCacheAnomaly 检查缓存异常
func (ae *AlertEngine) checkCacheAnomaly(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	metrics.mu.RLock()
	totalAccess := metrics.CacheHits + metrics.CacheMisses
	hits := metrics.CacheHits
	metrics.mu.RUnlock()

	if totalAccess < 50 {
		return nil // 样本太少
	}

	hitRate := float64(hits) / float64(totalAccess) * 100
	if hitRate < 20 {
		return &Alert{
			ID:        "cache_anomaly_" + time.Now().Format("20060102150405"),
			Level:     AlertLevelError,
			Rule:      "cache_anomaly",
			Message:   "缓存命中率异常低，可能是LLM判定逻辑故障",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"hit_rate":  hitRate,
				"threshold": 20.0,
			},
		}
	}

	return nil
}

// checkDecaySpike 检查记忆衰减突增
func (ae *AlertEngine) checkDecaySpike(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	metrics.mu.RLock()
	forgotten := metrics.TotalForgotten
	metrics.mu.RUnlock()

	// 简化版本：如果遗忘数超过1000就告警
	// TODO: 实现基于历史均值的动态阈值
	if forgotten > 1000 {
		return &Alert{
			ID:        "decay_spike_" + time.Now().Format("20060102150405"),
			Level:     AlertLevelInfo,
			Rule:      "decay_spike",
			Message:   "记忆遗忘数量较高，这可能是正常的衰减",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"forgotten": forgotten,
				"threshold": 1000,
			},
		}
	}

	return nil
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
				ID:        "low_success_rate_" + time.Now().Format("20060102150405"),
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

// makeCacheAnomalyCheck 创建缓存异常检查函数
func (ae *AlertEngine) makeCacheAnomalyCheck(threshold float64) func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
	return func(ctx context.Context, metrics *MetricsCollector, stagingStore *store.StagingStore) *Alert {
		metrics.mu.RLock()
		totalAccess := metrics.CacheHits + metrics.CacheMisses
		hits := metrics.CacheHits
		metrics.mu.RUnlock()

		if totalAccess < 50 {
			return nil // 样本太少
		}

		hitRate := float64(hits) / float64(totalAccess) * 100
		if hitRate < threshold {
			return &Alert{
				ID:        "cache_anomaly_" + time.Now().Format("20060102150405"),
				Level:     AlertLevelError,
				Rule:      "cache_anomaly",
				Message:   "缓存命中率异常低，可能是LLM判定逻辑故障",
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"hit_rate":  hitRate,
					"threshold": threshold,
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
				ID:        "decay_spike_" + time.Now().Format("20060102150405"),
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
