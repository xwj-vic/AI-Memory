package memory

import (
	"context"
)

// GetStatsWithCache 获取统计信息（带缓存）
func (ae *AlertEngine) GetStatsWithCache(ctx context.Context) (*AlertEngineStats, map[AlertLevel]int, error) {
	// 尝试从缓存获取
	if stats, levelCounts, ok := ae.statsCache.GetStats(); ok {
		return stats, levelCounts, nil
	}

	// 缓存未命中，重新计算
	stats := ae.GetStats()
	levelCounts, err := ae.GetAlertsByLevel(ctx)
	if err != nil {
		return stats, nil, err
	}

	// 更新缓存
	ae.statsCache.SetStats(stats, levelCounts)

	return stats, levelCounts, nil
}

// InvalidateStatsCache 使统计缓存失效（在告警触发时调用）
func (ae *AlertEngine) InvalidateStatsCache() {
	if ae.statsCache != nil {
		ae.statsCache.Invalidate()
	}
}
