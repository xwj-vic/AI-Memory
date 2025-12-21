package memory

import (
	"sync"
	"time"
)

// StatsCache 统计信息缓存
type StatsCache struct {
	mu          sync.RWMutex
	stats       *AlertEngineStats
	levelCounts map[AlertLevel]int
	cachedAt    time.Time
	ttl         time.Duration
}

// NewStatsCache 创建统计缓存
func NewStatsCache(ttl time.Duration) *StatsCache {
	return &StatsCache{
		ttl: ttl,
	}
}

// GetStats 获取缓存的统计信息
func (sc *StatsCache) GetStats() (*AlertEngineStats, map[AlertLevel]int, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.stats == nil || time.Since(sc.cachedAt) > sc.ttl {
		return nil, nil, false
	}

	return sc.stats, sc.levelCounts, true
}

// SetStats 设置统计缓存
func (sc *StatsCache) SetStats(stats *AlertEngineStats, levelCounts map[AlertLevel]int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats = stats
	sc.levelCounts = levelCounts
	sc.cachedAt = time.Now()
}

// Invalidate 使缓存失效
func (sc *StatsCache) Invalidate() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats = nil
	sc.levelCounts = nil
}
