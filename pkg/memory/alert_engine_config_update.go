package memory

import (
	"context"
	"fmt"
)

// UpdateRuleConfigJSON 更新规则配置JSON
func (ae *AlertEngine) UpdateRuleConfigJSON(ruleID string, configJSON string) error {
	if ae.configPersistence == nil {
		return fmt.Errorf("config persistence not initialized")
	}

	// 更新数据库
	ctx := context.Background()
	if err := ae.configPersistence.UpdateConfigJSON(ctx, ruleID, configJSON); err != nil {
		return err
	}

	return nil
}
