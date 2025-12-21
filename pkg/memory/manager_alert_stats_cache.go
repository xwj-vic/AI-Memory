package memory

import (
	"context"
	"fmt"
)

// GetAlertStatsWithCache 获取告警统计（带缓存，Manager代理方法）
func (m *Manager) GetAlertStatsWithCache(ctx context.Context) (*AlertEngineStats, map[AlertLevel]int, error) {
	if m.alertEngine == nil {
		return nil, nil, fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.GetStatsWithCache(ctx)
}
