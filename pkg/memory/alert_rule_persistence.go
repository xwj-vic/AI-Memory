package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// RuleConfigPersistence 规则配置持久化层
type RuleConfigPersistence struct {
	db *sql.DB
}

// NewRuleConfigPersistence 创建规则配置持久化实例
func NewRuleConfigPersistence(db *sql.DB) *RuleConfigPersistence {
	return &RuleConfigPersistence{db: db}
}

// RuleConfigDB 数据库中的规则配置
type RuleConfigDB struct {
	ID              string
	Name            string
	Description     string
	Enabled         bool
	CooldownSeconds int
	ConfigJSON      string
	UpdatedAt       time.Time
}

// LoadAll 加载所有规则配置
func (p *RuleConfigPersistence) LoadAll(ctx context.Context) (map[string]*RuleConfigDB, error) {
	query := `
		SELECT id, name, description, enabled, cooldown_seconds, config_json, updated_at
		FROM alert_rule_configs
	`

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load rule configs: %w", err)
	}
	defer rows.Close()

	configs := make(map[string]*RuleConfigDB)
	for rows.Next() {
		var cfg RuleConfigDB
		if err := rows.Scan(
			&cfg.ID,
			&cfg.Name,
			&cfg.Description,
			&cfg.Enabled,
			&cfg.CooldownSeconds,
			&cfg.ConfigJSON,
			&cfg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan rule config: %w", err)
		}
		configs[cfg.ID] = &cfg
	}

	return configs, rows.Err()
}

// Save 保存单个规则配置
func (p *RuleConfigPersistence) Save(ctx context.Context, ruleID string, enabled bool, cooldownSeconds int, configJSON string) error {
	query := `
		INSERT INTO alert_rule_configs (id, name, description, enabled, cooldown_seconds, config_json)
		VALUES (?, '', '', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			enabled = VALUES(enabled),
			cooldown_seconds = VALUES(cooldown_seconds),
			config_json = VALUES(config_json),
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := p.db.ExecContext(ctx, query, ruleID, enabled, cooldownSeconds, configJSON)
	if err != nil {
		return fmt.Errorf("failed to save rule config: %w", err)
	}

	return nil
}

// UpdateEnabled 更新规则启用状态
func (p *RuleConfigPersistence) UpdateEnabled(ctx context.Context, ruleID string, enabled bool) error {
	query := `UPDATE alert_rule_configs SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := p.db.ExecContext(ctx, query, enabled, ruleID)
	if err != nil {
		return fmt.Errorf("failed to update enabled status: %w", err)
	}

	// 检查是否更新了记录
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	return nil
}

// UpdateCooldown 更新规则冷却时间
func (p *RuleConfigPersistence) UpdateCooldown(ctx context.Context, ruleID string, cooldownSeconds int) error {
	query := `UPDATE alert_rule_configs SET cooldown_seconds = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := p.db.ExecContext(ctx, query, cooldownSeconds, ruleID)
	if err != nil {
		return fmt.Errorf("failed to update cooldown: %w", err)
	}

	// 检查是否更新了记录
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	return nil
}

// SeedDefaultConfigs 插入默认规则配置（如果不存在）
func (p *RuleConfigPersistence) SeedDefaultConfigs(ctx context.Context) error {
	defaults := []struct {
		ID              string
		Name            string
		Description     string
		Enabled         bool
		CooldownSeconds int
		ConfigJSON      string
	}{
		{
			ID:              "queue_backlog",
			Name:            "队列积压告警",
			Description:     "Staging队列长度超过阈值",
			Enabled:         true,
			CooldownSeconds: 600,
			ConfigJSON:      `{"threshold": 100}`,
		},
		{
			ID:              "low_success_rate",
			Name:            "晋升成功率过低",
			Description:     "记忆晋升成功率低于阈值",
			Enabled:         true,
			CooldownSeconds: 1800,
			ConfigJSON:      `{"threshold": 60}`,
		},
		{
			ID:              "cache_anomaly",
			Name:            "缓存命中率异常",
			Description:     "判定缓存命中率异常（智能检测）",
			Enabled:         true,
			CooldownSeconds: 900,
			ConfigJSON:      `{"window_minutes": 5, "min_samples": 500, "warn_threshold": 30, "error_threshold": 15}`,
		},
		{
			ID:              "decay_spike",
			Name:            "记忆衰减突增",
			Description:     "遗忘的记忆数量突然增加",
			Enabled:         true,
			CooldownSeconds: 3600,
			ConfigJSON:      `{"threshold": 1000}`,
		},
	}

	query := `
		INSERT INTO alert_rule_configs (id, name, description, enabled, cooldown_seconds, config_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE id = id
	`

	for _, cfg := range defaults {
		_, err := p.db.ExecContext(ctx, query,
			cfg.ID, cfg.Name, cfg.Description, cfg.Enabled, cfg.CooldownSeconds, cfg.ConfigJSON)
		if err != nil {
			return fmt.Errorf("failed to seed default config for %s: %w", cfg.ID, err)
		}
	}

	logger.System("✅ Alert rule default configs seeded")
	return nil
}

// ApplyConfigToRule 将数据库配置应用到规则对象
func ApplyConfigToRule(rule *AlertRule, dbConfig *RuleConfigDB) {
	if dbConfig == nil {
		return
	}

	rule.Name = dbConfig.Name
	rule.Description = dbConfig.Description
	rule.Enabled = dbConfig.Enabled
	rule.Cooldown = time.Duration(dbConfig.CooldownSeconds) * time.Second

	// 可选：解析config_json应用特定配置
	// 这里暂时不实现，因为阈值在config.go中管理
}

// SerializeRuleConfig 序列化规则配置为JSON
func SerializeRuleConfig(config interface{}) string {
	data, err := json.Marshal(config)
	if err != nil {
		return "{}"
	}
	return string(data)
}
