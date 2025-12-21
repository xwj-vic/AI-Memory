package memory

import (
	"context"
	"fmt"
	"time"
)

// GetAllAlertRules 获取所有告警规则
func (m *Manager) GetAllAlertRules() []RuleInfo {
	if m.alertEngine == nil {
		return []RuleInfo{}
	}
	return m.alertEngine.GetAllRules()
}

// ToggleAlertRule 启用/禁用告警规则
func (m *Manager) ToggleAlertRule(ruleID string, enabled bool) error {
	if m.alertEngine == nil {
		return fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.ToggleRule(ruleID, enabled)
}

// UpdateAlertRuleCooldown 更新规则冷却时间
func (m *Manager) UpdateAlertRuleCooldown(ruleID string, cooldown time.Duration) error {
	if m.alertEngine == nil {
		return fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.UpdateRuleCooldown(ruleID, cooldown)
}

// GetAlertStats 获取告警统计信息
func (m *Manager) GetAlertStats() *AlertEngineStats {
	if m.alertEngine == nil {
		return &AlertEngineStats{}
	}
	return m.alertEngine.GetStats()
}

// GetAlertTrend 获取告警趋势
func (m *Manager) GetAlertTrend(ctx context.Context, hours int) (map[string]interface{}, error) {
	if m.alertEngine == nil {
		return nil, fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.GetAlertTrend(ctx, hours)
}

// GetAlertsByLevel 按级别统计告警
func (m *Manager) GetAlertsByLevel(ctx context.Context) (map[AlertLevel]int, error) {
	if m.alertEngine == nil {
		return nil, fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.GetAlertsByLevel(ctx)
}
