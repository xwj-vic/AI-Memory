package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// AlertEngineStatsSync 统计同步管理器
type AlertEngineStatsSync struct {
	engine *AlertEngine

	// 待同步的增量
	pendingChecks  int64
	pendingSuccess int64
	pendingFailed  int64
	mu             sync.Mutex

	// 同步控制
	syncInterval time.Duration
	stopChan     chan struct{}
}

// NewAlertEngineStatsSync 创建统计同步管理器
func NewAlertEngineStatsSync(engine *AlertEngine, syncInterval time.Duration) *AlertEngineStatsSync {
	return &AlertEngineStatsSync{
		engine:       engine,
		syncInterval: syncInterval,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动定期同步
func (s *AlertEngineStatsSync) Start() {
	ticker := time.NewTicker(s.syncInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.FlushToDB()
			case <-s.stopChan:
				ticker.Stop()
				// 停止前最后一次刷新
				s.FlushToDB()
				logger.System("Alert stats sync stopped")
				return
			}
		}
	}()
	logger.System("Alert stats sync started", "interval", s.syncInterval)
}

// Stop 停止同步
func (s *AlertEngineStatsSync) Stop() {
	close(s.stopChan)
}

// RecordCheck 记录检查（累积）
func (s *AlertEngineStatsSync) RecordCheck() {
	atomic.AddInt64(&s.pendingChecks, 1)
}

// RecordNotifySuccess 记录通知成功（累积）
func (s *AlertEngineStatsSync) RecordNotifySuccess() {
	atomic.AddInt64(&s.pendingSuccess, 1)
}

// RecordNotifyFailed 记录通知失败（累积）
func (s *AlertEngineStatsSync) RecordNotifyFailed() {
	atomic.AddInt64(&s.pendingFailed, 1)
}

// FlushToDB 刷新到数据库
func (s *AlertEngineStatsSync) FlushToDB() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取并重置累积值
	checks := atomic.SwapInt64(&s.pendingChecks, 0)
	success := atomic.SwapInt64(&s.pendingSuccess, 0)
	failed := atomic.SwapInt64(&s.pendingFailed, 0)

	if checks == 0 && success == 0 && failed == 0 {
		return // 无变化，跳过
	}

	// 写入数据库
	if s.engine.statsPersistence != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.engine.statsPersistence.Update(ctx, checks, success, failed); err != nil {
			logger.Error("Failed to sync alert stats to DB", err)
			// 失败时恢复累积值（避免丢失）
			atomic.AddInt64(&s.pendingChecks, checks)
			atomic.AddInt64(&s.pendingSuccess, success)
			atomic.AddInt64(&s.pendingFailed, failed)
		}
	}
}
