package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"database/sql"
	"sync"
	"time"
)

// MetricsPersistence 监控指标持久化
type MetricsPersistence struct {
	db                *sql.DB
	mu                sync.RWMutex
	persistInterval   time.Duration
	stopChan          chan struct{}
	lastPersistedTime time.Time
	lastQueueLength   float64 // 上次写入的队列长度（只在变化时写入）
}

// NewMetricsPersistence 创建持久化实例
func NewMetricsPersistence(db *sql.DB, persistIntervalMinutes int) *MetricsPersistence {
	return &MetricsPersistence{
		db:              db,
		persistInterval: time.Duration(persistIntervalMinutes) * time.Minute,
		stopChan:        make(chan struct{}),
		lastQueueLength: -1, // 初始化为-1确保首次一定写入
	}
}

// Start 启动定时持久化任务
func (mp *MetricsPersistence) Start(collector *MetricsCollector) {
	ticker := time.NewTicker(mp.persistInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				mp.persistMetrics(collector)
			case <-mp.stopChan:
				ticker.Stop()
				logger.System("Metrics persistence stopped")
				return
			}
		}
	}()

	logger.System("✅ Metrics persistence started", "interval", mp.persistInterval)
}

// Stop 停止持久化任务
func (mp *MetricsPersistence) Stop() {
	close(mp.stopChan)
}

// persistMetrics 持久化指标到数据库
func (mp *MetricsPersistence) persistMetrics(collector *MetricsCollector) {
	ctx := context.Background()
	mp.mu.Lock()
	defer mp.mu.Unlock()

	collector.mu.RLock()
	defer collector.mu.RUnlock()

	// 1. 更新累计统计
	if err := mp.updateCumulativeStats(ctx, collector); err != nil {
		logger.Error("Failed to persist cumulative stats", err)
	}

	// 2. 批量插入时间序列数据
	if err := mp.insertTimeSeriesData(ctx, collector); err != nil {
		logger.Error("Failed to persist timeseries data", err)
	}
}

// updateCumulativeStats 更新累计统计表
func (mp *MetricsPersistence) updateCumulativeStats(ctx context.Context, collector *MetricsCollector) error {
	query := `
		UPDATE metrics_cumulative 
		SET total_promotions = ?, 
		    total_rejections = ?, 
		    total_forgotten = ?, 
		    cache_hits = ?, 
		    cache_misses = ?
		WHERE id = 1
	`

	_, err := mp.db.ExecContext(ctx, query,
		collector.TotalPromotions,
		collector.TotalRejections,
		collector.TotalForgotten,
		collector.CacheHits,
		collector.CacheMisses,
	)

	return err
}

// insertTimeSeriesData 批量插入时间序列数据
func (mp *MetricsPersistence) insertTimeSeriesData(ctx context.Context, collector *MetricsCollector) error {
	tx, err := mp.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO metrics_timeseries (metric_type, value, category, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// 插入晋升历史
	maxTime := mp.lastPersistedTime
	for _, point := range collector.PromotionHistory {
		if point.Timestamp.After(mp.lastPersistedTime) {
			if _, err := stmt.ExecContext(ctx, "promotion", point.Value, point.Label, point.Timestamp); err != nil {
				logger.Error("Failed to insert promotion metric", err)
			}
			if point.Timestamp.After(maxTime) {
				maxTime = point.Timestamp
			}
		}
	}

	// 插入队列长度历史（只在值变化时写入，减少数据量）
	for _, point := range collector.QueueLengthHistory {
		if point.Timestamp.After(mp.lastPersistedTime) {
			// 只在队列长度变化时才写入数据库
			if point.Value != mp.lastQueueLength {
				if _, err := stmt.ExecContext(ctx, "queue_length", point.Value, nil, point.Timestamp); err != nil {
					logger.Error("Failed to insert queue_length metric", err)
				}
				mp.lastQueueLength = point.Value
			}
			if point.Timestamp.After(maxTime) {
				maxTime = point.Timestamp
			}
		}
	}

	mp.lastPersistedTime = maxTime

	// 清空内存中的历史数据（已持久化）
	// 注意：为了保证前端图表连续性，保留最近2小时的数据在内存
	cutoff := time.Now().Add(-2 * time.Hour)
	collector.PromotionHistory = filterRecentPoints(collector.PromotionHistory, cutoff)
	collector.QueueLengthHistory = filterRecentPoints(collector.QueueLengthHistory, cutoff)

	return tx.Commit()
}

// filterRecentPoints 过滤保留最近的点
func filterRecentPoints(points []TimeSeriesPoint, cutoff time.Time) []TimeSeriesPoint {
	result := make([]TimeSeriesPoint, 0, len(points))
	for _, p := range points {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	return result
}

// LoadCumulativeStats 从数据库加载累计统计（启动时调用）
func (mp *MetricsPersistence) LoadCumulativeStats(ctx context.Context, collector *MetricsCollector) error {
	query := `
		SELECT total_promotions, total_rejections, total_forgotten, cache_hits, cache_misses 
		FROM metrics_cumulative 
		WHERE id = 1
	`

	var stats struct {
		Promotions int64
		Rejections int64
		Forgotten  int64
		Hits       int64
		Misses     int64
	}

	err := mp.db.QueryRowContext(ctx, query).Scan(
		&stats.Promotions,
		&stats.Rejections,
		&stats.Forgotten,
		&stats.Hits,
		&stats.Misses,
	)

	if err != nil {
		return err
	}

	// 恢复到全局指标收集器
	collector.mu.Lock()
	collector.TotalPromotions = stats.Promotions
	collector.TotalRejections = stats.Rejections
	collector.TotalForgotten = stats.Forgotten
	collector.CacheHits = stats.Hits
	collector.CacheMisses = stats.Misses
	collector.mu.Unlock()

	logger.System("✅ Loaded cumulative stats from DB",
		"promotions", stats.Promotions,
		"rejections", stats.Rejections,
		"forgotten", stats.Forgotten)

	return nil
}

// LoadRecentTimeSeries 加载最近N小时的时间序列数据
func (mp *MetricsPersistence) LoadRecentTimeSeries(ctx context.Context, collector *MetricsCollector, hours int) error {
	query := `
		SELECT metric_type, value, category, timestamp 
		FROM metrics_timeseries 
		WHERE timestamp >= DATE_SUB(NOW(), INTERVAL ? HOUR)
		ORDER BY timestamp ASC
	`

	rows, err := mp.db.QueryContext(ctx, query, hours)
	if err != nil {
		return err
	}
	defer rows.Close()

	collector.mu.Lock()
	defer collector.mu.Unlock()

	// 清空现有数据
	collector.PromotionHistory = make([]TimeSeriesPoint, 0, 144)
	collector.QueueLengthHistory = make([]TimeSeriesPoint, 0, 144)

	for rows.Next() {
		var metricType string
		var value float64
		var category sql.NullString
		var timestamp time.Time

		if err := rows.Scan(&metricType, &value, &category, &timestamp); err != nil {
			continue
		}

		point := TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     value,
		}

		if category.Valid {
			point.Label = category.String
		}

		switch metricType {
		case "promotion":
			collector.PromotionHistory = append(collector.PromotionHistory, point)
		case "queue_length":
			collector.QueueLengthHistory = append(collector.QueueLengthHistory, point)
		}
	}

	logger.System("✅ Loaded timeseries data from DB",
		"promotions", len(collector.PromotionHistory),
		"queue_points", len(collector.QueueLengthHistory),
		"hours", hours)

	// 更新最后持久化时间，避免重启后通过 LoadRecentTimeSeries 加载的数据被重复持久化
	maxTime := time.Time{}
	for _, p := range collector.PromotionHistory {
		if p.Timestamp.After(maxTime) {
			maxTime = p.Timestamp
		}
	}
	for _, p := range collector.QueueLengthHistory {
		if p.Timestamp.After(maxTime) {
			maxTime = p.Timestamp
		}
	}
	mp.lastPersistedTime = maxTime

	return nil
}

// CleanupOldData 清理超过 retentionDays 天的历史数据
func (mp *MetricsPersistence) CleanupOldData(ctx context.Context, retentionDays int) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	query := `DELETE FROM metrics_timeseries WHERE timestamp < DATE_SUB(NOW(), INTERVAL ? DAY)`
	result, err := mp.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return err
	}

	rowsDeleted, _ := result.RowsAffected()
	if rowsDeleted > 0 {
		logger.System("✅ 监控数据清理完成", "deleted_rows", rowsDeleted, "retention_days", retentionDays)
	}

	return nil
}

// StartWithCleanup 启动定时持久化和清理任务
func (mp *MetricsPersistence) StartWithCleanup(collector *MetricsCollector, retentionDays int) {
	// 启动原有的持久化定时器
	persistTicker := time.NewTicker(mp.persistInterval)

	// 启动每日清理定时器（每24小时执行一次）
	cleanupTicker := time.NewTicker(24 * time.Hour)

	go func() {
		// 启动时立即执行一次清理
		if err := mp.CleanupOldData(context.Background(), retentionDays); err != nil {
			logger.Error("启动时清理历史数据失败", err)
		}

		for {
			select {
			case <-persistTicker.C:
				mp.persistMetrics(collector)
			case <-cleanupTicker.C:
				if err := mp.CleanupOldData(context.Background(), retentionDays); err != nil {
					logger.Error("定时清理历史数据失败", err)
				}
			case <-mp.stopChan:
				persistTicker.Stop()
				cleanupTicker.Stop()
				logger.System("Metrics persistence stopped")
				return
			}
		}
	}()

	logger.System("✅ Metrics persistence started (with cleanup)", "persist_interval", mp.persistInterval, "retention_days", retentionDays)
}
