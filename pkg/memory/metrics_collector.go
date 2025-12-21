package memory

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// 包级别的数据库连接（用于时间序列查询）
var metricsDB *sql.DB

// SetMetricsDB 设置监控指标数据库连接（在 main.go 初始化时调用）
func SetMetricsDB(db *sql.DB) {
	metricsDB = db
}

// Dashboard 缓存（30秒过期，按时间范围独立缓存）
const dashboardCacheTTL = 30 * time.Second

type cacheEntry struct {
	data     map[string]interface{}
	expireAt time.Time
}

type dashboardCache struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry // key: timeRange (1h/24h/7d/30d)
}

var dbCache = &dashboardCache{
	cache: make(map[string]*cacheEntry),
}

// 分类分布独立缓存（30秒过期，不随时间范围变化）
type categoryCache struct {
	mu       sync.RWMutex
	data     map[string]int
	expireAt time.Time
}

var catCache = &categoryCache{}

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

// GetGlobalMetrics 获取全局指标收集器实例（供main.go等外部使用）
func GetGlobalMetrics() *MetricsCollector {
	return globalMetrics
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

	// 找到第一个满足条件的索引
	startIdx := -1
	for i, point := range *history {
		if point.Timestamp.After(cutoff) {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		*history = make([]TimeSeriesPoint, 0, 144)
	} else if startIdx > 0 {
		*history = (*history)[startIdx:]
	}
}

// GetDashboardMetrics 获取Dashboard所需的所有指标
// timeRange 支持: 1h, 24h, 7d, 30d
// 使用 30 秒本地缓存减少数据库查询
func (m *Manager) GetDashboardMetrics(ctx context.Context, timeRange string) map[string]interface{} {
	// 检查缓存是否命中（按时间范围独立缓存）
	dbCache.mu.RLock()
	if entry, ok := dbCache.cache[timeRange]; ok && time.Now().Before(entry.expireAt) {
		cached := entry.data
		dbCache.mu.RUnlock()
		return cached
	}
	dbCache.mu.RUnlock()

	// 缓存未命中，重新查询
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	// 解析时间范围
	hours := parseTimeRangeToHours(timeRange)

	// 获取当前Staging队列长度
	currentQueueLength := m.getStagingQueueLength(ctx)

	// 更新队列长度历史
	go globalMetrics.RecordQueueLength(currentQueueLength)

	// 1. 获取DB中的原始数据
	dbPromotions, dbQueues := m.queryRawMetricsFromDB(ctx, hours)

	// 找出DB中最新的时间点
	var maxDBTime time.Time
	for _, p := range dbPromotions {
		if p.Timestamp.After(maxDBTime) {
			maxDBTime = p.Timestamp
		}
	}
	for _, p := range dbQueues {
		if p.Timestamp.After(maxDBTime) {
			maxDBTime = p.Timestamp
		}
	}

	// 2. 获取内存中的数据（副本）
	memPromoHistory := make([]TimeSeriesPoint, len(globalMetrics.PromotionHistory))
	copy(memPromoHistory, globalMetrics.PromotionHistory)
	memQueueHistory := make([]TimeSeriesPoint, len(globalMetrics.QueueLengthHistory))
	copy(memQueueHistory, globalMetrics.QueueLengthHistory)

	// 3. 合并数据（排除DB中已有的时间段，避免重复）
	allPromotions := make([]TimeSeriesPoint, 0, len(dbPromotions)+len(memPromoHistory))
	allPromotions = append(allPromotions, dbPromotions...)

	for _, p := range memPromoHistory {
		if p.Timestamp.After(maxDBTime) {
			allPromotions = append(allPromotions, p)
		}
	}

	allQueues := make([]TimeSeriesPoint, 0, len(dbQueues)+len(memQueueHistory))
	allQueues = append(allQueues, dbQueues...)

	for _, p := range memQueueHistory {
		if p.Timestamp.After(maxDBTime) {
			allQueues = append(allQueues, p)
		}
	}

	// 排除超出时间范围的数据
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	allPromotions = filterPointsAfter(allPromotions, cutoff)
	allQueues = filterPointsAfter(allQueues, cutoff)

	// 4. 执行聚合
	var promotionTrend, queueTrend []TimeSeriesPoint
	if hours <= 1 {
		promotionTrend = aggregateByMinute(allPromotions, false, hours*60) // sum
		queueTrend = aggregateByMinute(allQueues, true, hours*60)          // avg
	} else if hours <= 24 {
		promotionTrend = aggregateByHour(allPromotions, false, hours) // sum
		queueTrend = aggregateByHour(allQueues, true, hours)          // avg
	} else {
		days := hours / 24
		promotionTrend = aggregateByDay(allPromotions, false, days) // sum
		queueTrend = aggregateByDay(allQueues, true, days)          // avg
	}

	// 长期记忆分布：使用独立缓存，30秒过期后再查DB
	var categoryMap map[string]int
	catCache.mu.RLock()
	if catCache.data != nil && time.Now().Before(catCache.expireAt) {
		categoryMap = catCache.data
		catCache.mu.RUnlock()
	} else {
		catCache.mu.RUnlock()
		// 缓存过期，查询数据库
		categoryMap = m.queryCategoryDistributionFromDB(ctx, 24*30)
		// 更新缓存
		catCache.mu.Lock()
		catCache.data = categoryMap
		catCache.expireAt = time.Now().Add(dashboardCacheTTL)
		catCache.mu.Unlock()
	}

	// 更新全局分类分布缓存
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

	result := map[string]interface{}{
		// 实时统计
		"current_queue_length": currentQueueLength,

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

		// 分类分布（直接使用查询结果，转换为 CategoryCount 格式）
		"category_distribution": convertToCategoryHistory(categoryMap),

		// 元信息
		"timestamp":        time.Now().Format(time.RFC3339),
		"data_range_hours": hours,
	}

	// 更新缓存（按时间范围独立存储）
	dbCache.mu.Lock()
	dbCache.cache[timeRange] = &cacheEntry{
		data:     result,
		expireAt: time.Now().Add(dashboardCacheTTL),
	}
	dbCache.mu.Unlock()

	return result
}

// convertToCategoryHistory 将 map[string]int 转换为 []CategoryCount 格式
func convertToCategoryHistory(categoryMap map[string]int) []CategoryCount {
	if len(categoryMap) == 0 {
		return []CategoryCount{}
	}

	total := 0
	for _, count := range categoryMap {
		total += count
	}

	result := make([]CategoryCount, 0, len(categoryMap))
	for category, count := range categoryMap {
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		result = append(result, CategoryCount{
			Category: category,
			Count:    count,
			Percent:  percent,
		})
	}
	return result
}

// getStagingQueueLength 获取 Staging 队列长度
func (m *Manager) getStagingQueueLength(ctx context.Context) int {
	entries, _ := m.stagingStore.GetPendingEntries(ctx, 1, 0)
	return len(entries)
}

// parseTimeRangeToHours 解析时间范围字符串为小时数
func parseTimeRangeToHours(timeRange string) int {
	switch timeRange {
	case "1h":
		return 1
	case "24h":
		return 24
	case "7d":
		return 24 * 7
	case "30d":
		return 24 * 30
	default:
		return 24 // 默认24小时
	}
}

// queryRawMetricsFromDB 获取原始时间序列数据，不进行SQL聚合
func (m *Manager) queryRawMetricsFromDB(ctx context.Context, hours int) (promotions, queues []TimeSeriesPoint) {
	if metricsDB == nil {
		return nil, nil
	}

	query := `
		SELECT metric_type, value, timestamp
		FROM metrics_timeseries 
		WHERE timestamp >= DATE_SUB(NOW(), INTERVAL ? HOUR)
		ORDER BY timestamp ASC
	`

	rows, err := metricsDB.QueryContext(ctx, query, hours)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	promotions = make([]TimeSeriesPoint, 0)
	queues = make([]TimeSeriesPoint, 0)

	for rows.Next() {
		var metricType string
		var value float64
		var timestamp time.Time

		if err := rows.Scan(&metricType, &value, &timestamp); err != nil {
			continue
		}

		point := TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     value,
		}

		switch metricType {
		case "promotion":
			promotions = append(promotions, point)
		case "queue_length":
			queues = append(queues, point)
		}
	}

	return promotions, queues
}

// filterPointsAfter 过滤出指定时间之后的数据点
func filterPointsAfter(points []TimeSeriesPoint, cutoff time.Time) []TimeSeriesPoint {
	result := make([]TimeSeriesPoint, 0, len(points))
	for _, p := range points {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	return result
}

// queryCategoryDistributionFromDB 从数据库直接统计分类分布
func (m *Manager) queryCategoryDistributionFromDB(ctx context.Context, hours int) map[string]int {
	categoryMap := make(map[string]int)
	if metricsDB == nil {
		return categoryMap
	}

	query := `
		SELECT category, COUNT(*) as cnt
		FROM metrics_timeseries FORCE INDEX (idx_type_time)
		WHERE metric_type = 'promotion' 
		  AND category IS NOT NULL 
		  AND timestamp >= DATE_SUB(NOW(), INTERVAL ? HOUR)
		GROUP BY category
	`

	rows, err := metricsDB.QueryContext(ctx, query, hours)
	if err != nil {
		return categoryMap
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var cnt int
		if err := rows.Scan(&category, &cnt); err != nil {
			continue
		}
		categoryMap[category] = cnt
	}

	return categoryMap
}

// aggregateByHour 将时间序列数据按小时聚合
// isAverage: true则计算平均值（如队列长度），false则计算累计值（如晋升数）
func aggregateByHour(points []TimeSeriesPoint, isAverage bool, hours int) []TimeSeriesPoint {
	if hours <= 0 {
		hours = 24
	}

	hourlyProgress := make(map[string]float64)
	hourlyCount := make(map[string]int)

	// 初始化时间槽 (使用本地时间)
	now := time.Now()
	for i := 0; i < hours; i++ {
		t := now.Add(-time.Duration(i) * time.Hour)
		key := t.Format("2006-01-02 15:00") // Local Time Format
		hourlyProgress[key] = 0
		hourlyCount[key] = 0
	}

	for _, point := range points {
		// 转换到本地时间进行 key 生成
		localTime := point.Timestamp.Local()
		hourKey := localTime.Format("2006-01-02 15:00")

		// 只有在初始化的时间槽内才统计（避免统计范围外的数据）
		if _, exists := hourlyProgress[hourKey]; exists {
			hourlyProgress[hourKey] += point.Value
			hourlyCount[hourKey]++
		}
	}

	result := make([]TimeSeriesPoint, 0, hours)
	for i := hours - 1; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Hour)
		key := t.Format("2006-01-02 15:00")

		val := hourlyProgress[key]
		count := hourlyCount[key]

		if isAverage && count > 0 {
			val = val / float64(count)
		}

		timestamp, _ := time.ParseInLocation("2006-01-02 15:00", key, time.Local)
		result = append(result, TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     val,
		})
	}

	return result
}

// aggregateByDay 将时间序列数据按天聚合（用于7d、30d视图）
func aggregateByDay(points []TimeSeriesPoint, isAverage bool, days int) []TimeSeriesPoint {
	if days <= 0 {
		days = 7
	}

	dailyProgress := make(map[string]float64)
	dailyCount := make(map[string]int)

	// 初始化时间槽
	now := time.Now()
	for i := 0; i < days; i++ {
		t := now.AddDate(0, 0, -i)
		key := t.Format("2006-01-02")
		dailyProgress[key] = 0
		dailyCount[key] = 0
	}

	for _, point := range points {
		dayKey := point.Timestamp.Local().Format("2006-01-02")
		if _, exists := dailyProgress[dayKey]; exists {
			dailyProgress[dayKey] += point.Value
			dailyCount[dayKey]++
		}
	}

	result := make([]TimeSeriesPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		t := now.AddDate(0, 0, -i)
		key := t.Format("2006-01-02")

		val := dailyProgress[key]

		count := dailyCount[key]

		if isAverage && count > 0 {
			val = val / float64(count)
		}

		timestamp, _ := time.ParseInLocation("2006-01-02", key, time.Local)
		result = append(result, TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     val,
		})
	}

	return result
}

// aggregateByMinute 将时间序列数据按分钟聚合
// minutes: 聚合的总分钟数
func aggregateByMinute(points []TimeSeriesPoint, isAverage bool, minutes int) []TimeSeriesPoint {
	if minutes <= 0 {
		minutes = 60
	}

	minuteProgress := make(map[string]float64)
	minuteCount := make(map[string]int)

	// 初始化时间槽 (使用本地时间)
	now := time.Now()
	for i := 0; i < minutes; i++ {
		t := now.Add(-time.Duration(i) * time.Minute)
		key := t.Format("2006-01-02 15:04") // Local Time Format
		minuteProgress[key] = 0
		minuteCount[key] = 0
	}

	for _, point := range points {
		// 转换到本地时间进行 key 生成
		localTime := point.Timestamp.Local()
		minuteKey := localTime.Format("2006-01-02 15:04")

		// 只有在初始化的时间槽内才统计
		if _, exists := minuteProgress[minuteKey]; exists {
			minuteProgress[minuteKey] += point.Value
			minuteCount[minuteKey]++
		}
	}

	result := make([]TimeSeriesPoint, 0, minutes)
	for i := minutes - 1; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Minute)
		key := t.Format("2006-01-02 15:04")

		val := minuteProgress[key]
		count := minuteCount[key]

		if isAverage && count > 0 {
			val = val / float64(count)
		}

		timestamp, _ := time.ParseInLocation("2006-01-02 15:04", key, time.Local)
		result = append(result, TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     val,
		})
	}

	return result
}
