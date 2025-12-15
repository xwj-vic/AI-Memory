package memory

import (
	"context"
	"sync"
	"time"
)

// MetricsCollector 监控指标收集器
type MetricsCollector struct {
	mu sync.RWMutex

	// 时间序列数据（最近24小时）
	PromotionHistory   []TimeSeriesPoint // 晋升记录
	QueueLengthHistory []TimeSeriesPoint // 队列长度
	CategoryHistory    []CategoryCount   // 分类统计

	// 实时统计
	TotalPromotions int64
	TotalRejections int64
	TotalForgotten  int64
	CacheHits       int64
	CacheMisses     int64
}

type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}

type CategoryCount struct {
	Category string  `json:"category"`
	Count    int     `json:"count"`
	Percent  float64 `json:"percent"`
}

var globalMetrics = &MetricsCollector{
	PromotionHistory:   make([]TimeSeriesPoint, 0, 144), // 24小时，每10分钟一个点
	QueueLengthHistory: make([]TimeSeriesPoint, 0, 144),
	CategoryHistory:    make([]CategoryCount, 0),
}

// RecordPromotion 记录晋升事件
func (mc *MetricsCollector) RecordPromotion(category string, success bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	if success {
		mc.TotalPromotions++
		mc.PromotionHistory = append(mc.PromotionHistory, TimeSeriesPoint{
			Timestamp: now,
			Value:     1,
			Label:     category,
		})
	} else {
		mc.TotalRejections++
	}

	// 保留最近24小时
	mc.trimHistory(&mc.PromotionHistory, 24*time.Hour)
}

// RecordQueueLength 记录队列长度
func (mc *MetricsCollector) RecordQueueLength(length int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.QueueLengthHistory = append(mc.QueueLengthHistory, TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     float64(length),
	})

	mc.trimHistory(&mc.QueueLengthHistory, 24*time.Hour)
}

// UpdateCategoryDistribution 更新分类分布
func (mc *MetricsCollector) UpdateCategoryDistribution(categories map[string]int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	total := 0
	for _, count := range categories {
		total += count
	}

	mc.CategoryHistory = make([]CategoryCount, 0, len(categories))
	for cat, count := range categories {
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		mc.CategoryHistory = append(mc.CategoryHistory, CategoryCount{
			Category: cat,
			Count:    count,
			Percent:  percent,
		})
	}
}

// trimHistory 保留指定时间范围内的数据
func (mc *MetricsCollector) trimHistory(history *[]TimeSeriesPoint, duration time.Duration) {
	cutoff := time.Now().Add(-duration)
	newHistory := make([]TimeSeriesPoint, 0, len(*history))

	for _, point := range *history {
		if point.Timestamp.After(cutoff) {
			newHistory = append(newHistory, point)
		}
	}

	*history = newHistory
}

// GetDashboardMetrics 获取Dashboard所需的所有指标
func (m *Manager) GetDashboardMetrics(ctx context.Context) map[string]interface{} {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	// 获取当前Staging统计
	stagingEntries, _ := m.stagingStore.GetPendingEntries(ctx, 1, 0)
	currentQueueLength := len(stagingEntries)

	// 更新队列长度历史
	go globalMetrics.RecordQueueLength(currentQueueLength)

	// 分类统计
	categoryMap := make(map[string]int)
	highConfCount := 0
	mediumConfCount := 0
	lowConfCount := 0

	for _, entry := range stagingEntries {
		categoryMap[string(entry.Category)]++

		if entry.ConfidenceScore >= m.cfg.StagingConfidenceHigh {
			highConfCount++
		} else if entry.ConfidenceScore >= m.cfg.StagingConfidenceLow {
			mediumConfCount++
		} else {
			lowConfCount++
		}
	}

	// 更新分类分布
	go globalMetrics.UpdateCategoryDistribution(categoryMap)

	// 计算成功率
	totalAttempts := globalMetrics.TotalPromotions + globalMetrics.TotalRejections
	successRate := 0.0
	if totalAttempts > 0 {
		successRate = float64(globalMetrics.TotalPromotions) / float64(totalAttempts) * 100
	}

	// 缓存命中率
	totalCacheAccess := globalMetrics.CacheHits + globalMetrics.CacheMisses
	cacheHitRate := 0.0
	if totalCacheAccess > 0 {
		cacheHitRate = float64(globalMetrics.CacheHits) / float64(totalCacheAccess) * 100
	}

	// 聚合晋升趋势（每小时）
	promotionTrend := aggregateByHour(globalMetrics.PromotionHistory)
	queueTrend := aggregateByHour(globalMetrics.QueueLengthHistory)

	return map[string]interface{}{
		// 实时统计
		"current_queue_length":    currentQueueLength,
		"high_confidence_count":   highConfCount,
		"medium_confidence_count": mediumConfCount,
		"low_confidence_count":    lowConfCount,

		// 累计统计
		"total_promotions":       globalMetrics.TotalPromotions,
		"total_rejections":       globalMetrics.TotalRejections,
		"total_forgotten":        globalMetrics.TotalForgotten,
		"promotion_success_rate": successRate,

		// 缓存统计
		"cache_hit_rate": cacheHitRate,
		"cache_hits":     globalMetrics.CacheHits,
		"cache_misses":   globalMetrics.CacheMisses,

		// 时间序列
		"promotion_trend":    promotionTrend,
		"queue_length_trend": queueTrend,

		// 分类分布
		"category_distribution": globalMetrics.CategoryHistory,

		// 元信息
		"timestamp":        time.Now().Format(time.RFC3339),
		"data_range_hours": 24,
	}
}

// aggregateByHour 将时间序列数据按小时聚合
func aggregateByHour(points []TimeSeriesPoint) []TimeSeriesPoint {
	if len(points) == 0 {
		return []TimeSeriesPoint{}
	}

	hourly := make(map[string]float64)
	counts := make(map[string]int)

	for _, point := range points {
		hourKey := point.Timestamp.Format("2006-01-02 15:00")
		hourly[hourKey] += point.Value
		counts[hourKey]++
	}

	result := make([]TimeSeriesPoint, 0, len(hourly))
	for hourKey, sum := range hourly {
		timestamp, _ := time.Parse("2006-01-02 15:00", hourKey)
		avg := sum / float64(counts[hourKey])

		result = append(result, TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     avg,
		})
	}

	return result
}
