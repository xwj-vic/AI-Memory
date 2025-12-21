package memory

import (
	"context"
	"fmt"
)

// UpdateConfigJSON 更新规则配置JSON
func (p *RuleConfigPersistence) UpdateConfigJSON(ctx context.Context, ruleID string, configJSON string) error {
	query := `UPDATE alert_rule_configs SET config_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := p.db.ExecContext(ctx, query, configJSON, ruleID)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// 检查是否更新了记录
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	return nil
}
