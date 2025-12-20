package memory

import (
	"ai-memory/pkg/types"
	"context"
	"sync"
	"time"
)

// PerformanceMonitor 性能监控
type PerformanceMonitor struct {
	mu sync.RWMutex

	// 统计指标
	StagingQueueLength     int
	PromotionSuccessCount  int64
	PromotionFailCount     int64
	ForgottenMemoriesCount int64
	JudgmentCacheHits      int64
	JudgmentCacheMisses    int64

	// 缓存
	judgeCache      map[string]*types.JudgeResult
	cacheExpiry     time.Duration
	cacheTimestamps map[string]time.Time
}

// NewPerformanceMonitor 创建监控实例
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		judgeCache:      make(map[string]*types.JudgeResult),
		cacheTimestamps: make(map[string]time.Time),
		cacheExpiry:     time.Hour * 24, // 缓存24小时
	}
}

// GetJudgeResultFromCache 从缓存获取判定结果
func (pm *PerformanceMonitor) GetJudgeResultFromCache(content string) (*types.JudgeResult, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result, exists := pm.judgeCache[content]
	if !exists {
		pm.mu.RUnlock()
		pm.mu.Lock()
		pm.JudgmentCacheMisses++
		GetGlobalMetrics().mu.Lock()
		GetGlobalMetrics().CacheMisses++
		GetGlobalMetrics().mu.Unlock()
		pm.mu.Unlock()
		pm.mu.RLock()
		return nil, false
	}

	// 检查过期
	if time.Since(pm.cacheTimestamps[content]) > pm.cacheExpiry {
		pm.mu.RUnlock()
		pm.mu.Lock()
		delete(pm.judgeCache, content)
		delete(pm.cacheTimestamps, content)
		pm.JudgmentCacheMisses++
		GetGlobalMetrics().mu.Lock()
		GetGlobalMetrics().CacheMisses++
		GetGlobalMetrics().mu.Unlock()
		pm.mu.Unlock()
		pm.mu.RLock()
		return nil, false
	}

	pm.mu.RUnlock()
	pm.mu.Lock()
	pm.JudgmentCacheHits++
	GetGlobalMetrics().mu.Lock()
	GetGlobalMetrics().CacheHits++
	GetGlobalMetrics().mu.Unlock()
	pm.mu.Unlock()
	pm.mu.RLock()

	return result, true
}

// SetJudgeResultCache 设置判定结果缓存
func (pm *PerformanceMonitor) SetJudgeResultCache(content string, result *types.JudgeResult) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.judgeCache[content] = result
	pm.cacheTimestamps[content] = time.Now()
}

// RecordPromotion 记录晋升结果
func (pm *PerformanceMonitor) RecordPromotion(success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if success {
		pm.PromotionSuccessCount++
	} else {
		pm.PromotionFailCount++
	}
}

// RecordForgotten 记录遗忘
func (pm *PerformanceMonitor) RecordForgotten(count int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.ForgottenMemoriesCount += int64(count)
}

// UpdateStagingQueueLength 更新暂存区队列长度
func (pm *PerformanceMonitor) UpdateStagingQueueLength(length int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.StagingQueueLength = length
}

// GetMetrics 获取所有监控指标
func (pm *PerformanceMonitor) GetMetrics() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalPromotions := pm.PromotionSuccessCount + pm.PromotionFailCount
	successRate := float64(0)
	if totalPromotions > 0 {
		successRate = float64(pm.PromotionSuccessCount) / float64(totalPromotions) * 100
	}

	totalCacheAccess := pm.JudgmentCacheHits + pm.JudgmentCacheMisses
	cacheHitRate := float64(0)
	if totalCacheAccess > 0 {
		cacheHitRate = float64(pm.JudgmentCacheHits) / float64(totalCacheAccess) * 100
	}

	return map[string]interface{}{
		"staging_queue_length":     pm.StagingQueueLength,
		"promotion_success_count":  pm.PromotionSuccessCount,
		"promotion_fail_count":     pm.PromotionFailCount,
		"promotion_success_rate":   successRate,
		"forgotten_memories_count": pm.ForgottenMemoriesCount,
		"judgment_cache_hits":      pm.JudgmentCacheHits,
		"judgment_cache_misses":    pm.JudgmentCacheMisses,
		"judgment_cache_hit_rate":  cacheHitRate,
		"judgment_cache_size":      len(pm.judgeCache),
	}
}

// ========== Manager 扩展 ==========

// 在Manager结构中添加监控器
func (m *Manager) initPerformanceMonitor() {
	if m.monitor == nil {
		m.monitor = NewPerformanceMonitor()
	}
}

// GetPerformanceMetrics 获取性能指标（供API调用）
func (m *Manager) GetPerformanceMetrics(ctx context.Context) map[string]interface{} {
	// 获取暂存区长度
	entries, _ := m.stagingStore.GetPendingEntries(ctx, 1, 0)

	metrics := map[string]interface{}{
		"staging_queue_length": len(entries),
		"timestamp":            time.Now().Format(time.RFC3339),
	}

	return metrics
}
