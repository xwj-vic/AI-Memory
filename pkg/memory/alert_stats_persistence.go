package memory

import (
	"context"
	"database/sql"
	"fmt"
)

// AlertStatsPersistence 告警统计持久化
type AlertStatsPersistence struct {
	db *sql.DB
}

// NewAlertStatsPersistence 创建统计持久化实例
func NewAlertStatsPersistence(db *sql.DB) *AlertStatsPersistence {
	return &AlertStatsPersistence{db: db}
}

// Load 加载统计数据
func (p *AlertStatsPersistence) Load(ctx context.Context) (totalChecks, notifySuccess, notifyFailed int64, err error) {
	query := `SELECT total_checks, notify_success, notify_failed FROM alert_stats WHERE id = 1`
	err = p.db.QueryRowContext(ctx, query).Scan(&totalChecks, &notifySuccess, &notifyFailed)
	if err == sql.ErrNoRows {
		// 如果没有记录，返回0值
		return 0, 0, 0, nil
	}
	return
}

// Update 增量更新统计（批量）
func (p *AlertStatsPersistence) Update(ctx context.Context, deltaChecks, deltaSuccess, deltaFailed int64) error {
	if deltaChecks == 0 && deltaSuccess == 0 && deltaFailed == 0 {
		return nil // 无变化，跳过
	}

	query := `
		UPDATE alert_stats 
		SET 
			total_checks = total_checks + ?,
			notify_success = notify_success + ?,
			notify_failed = notify_failed + ?
		WHERE id = 1
	`
	result, err := p.db.ExecContext(ctx, query, deltaChecks, deltaSuccess, deltaFailed)
	if err != nil {
		return fmt.Errorf("failed to update alert stats: %w", err)
	}

	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		// 如果没有更新到行，可能是表为空，尝试插入
		insertQuery := `INSERT INTO alert_stats (id, total_checks, notify_success, notify_failed) VALUES (1, ?, ?, ?)`
		_, err = p.db.ExecContext(ctx, insertQuery, deltaChecks, deltaSuccess, deltaFailed)
		if err != nil {
			return fmt.Errorf("failed to insert alert stats: %w", err)
		}
	}

	return nil
}
