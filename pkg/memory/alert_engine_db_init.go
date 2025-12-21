package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"database/sql"
	"time"
)

// InitWithDB 使用数据库初始化（在Main中调用）
func (ae *AlertEngine) InitWithDB(ctx context.Context, db *sql.DB) error {
	if db == nil {
		logger.System("⚠️ Alert engine running without rule config persistence")
		return nil
	}

	// 创建持久化层
	ae.configPersistence = NewRuleConfigPersistence(db)
	ae.statsPersistence = NewAlertStatsPersistence(db)

	// 插入默认规则配置（如果不存在，使用INSERT ... ON DUPLICATE KEY UPDATE）
	if err := ae.configPersistence.SeedDefaultConfigs(ctx); err != nil {
		logger.Error("Failed to seed default configs", err)
		// 继续执行，不阻断启动
	}

	// 从数据库加载配置（表需要预先通过schema.sql创建）
	if err := ae.loadRuleConfigsFromDB(ctx); err != nil {
		logger.Error("Failed to load rule configs from DB", err)
		return err
	}

	// 从数据库加载历史统计数据
	if err := ae.loadStatsFromDB(ctx); err != nil {
		logger.Error("Failed to load stats from DB", err)
		// 不阻断启动，使用默认值0
	}

	// 启动统计同步（每10秒刷新一次）
	ae.statsSync = NewAlertEngineStatsSync(ae, 10*time.Second)
	ae.statsSync.Start()

	logger.System("✅ Alert engine initialized with DB persistence")
	return nil
}

// loadRuleConfigsFromDB 从数据库加载规则配置
func (ae *AlertEngine) loadRuleConfigsFromDB(ctx context.Context) error {
	configs, err := ae.configPersistence.LoadAll(ctx)
	if err != nil {
		return err
	}

	ae.mu.Lock()
	defer ae.mu.Unlock()

	appliedCount := 0
	for _, rule := range ae.rules {
		if dbConfig, ok := configs[rule.ID]; ok {
			ApplyConfigToRule(rule, dbConfig)
			appliedCount++
		}
	}

	logger.System("✅ Applied rule configs from DB", "count", appliedCount)
	return nil
}

// loadStatsFromDB 从数据库加载统计数据
func (ae *AlertEngine) loadStatsFromDB(ctx context.Context) error {
	if ae.statsPersistence == nil {
		return nil
	}

	checks, success, failed, err := ae.statsPersistence.Load(ctx)
	if err != nil {
		return err
	}

	// 加载到内存统计
	ae.stats.mu.Lock()
	ae.stats.TotalChecks = checks
	ae.stats.NotifySuccess = success
	ae.stats.NotifyFailed = failed
	ae.stats.mu.Unlock()

	logger.System("✅ Loaded alert stats from DB", "checks", checks, "success", success, "failed", failed)
	return nil
}
