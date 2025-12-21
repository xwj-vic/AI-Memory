package memory

import (
	"fmt"
)

// UpdateAlertRuleConfigJSON 更新规则配置JSON（Manager代理）
func (m *Manager) UpdateAlertRuleConfigJSON(ruleID string, configJSON string) error {
	if m.alertEngine == nil {
		return fmt.Errorf("alert engine not initialized")
	}
	return m.alertEngine.UpdateRuleConfigJSON(ruleID, configJSON)
}
