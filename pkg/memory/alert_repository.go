package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// AlertRepository 告警存储接口
type AlertRepository interface {
	// Save 保存告警到数据库
	Save(ctx context.Context, alert *Alert) error
	// QueryRecent 查询最近的N条告警
	QueryRecent(ctx context.Context, limit int) ([]Alert, error)
	// QueryFiltered 带过滤条件查询告警
	QueryFiltered(ctx context.Context, level, rule string, limit, offset int) ([]Alert, int, error)
	// Delete 删除告警
	Delete(ctx context.Context, id string) error
	// Count 统计告警总数
	Count(ctx context.Context, level, rule string) (int, error)
}

// MySQLAlertRepository MySQL实现
type MySQLAlertRepository struct {
	db *sql.DB
}

// NewMySQLAlertRepository 创建MySQL存储实现
func NewMySQLAlertRepository(db *sql.DB) *MySQLAlertRepository {
	return &MySQLAlertRepository{db: db}
}

// Save 保存告警
func (r *MySQLAlertRepository) Save(ctx context.Context, alert *Alert) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	metaBytes, _ := json.Marshal(alert.Metadata)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO alerts (id, level, rule, message, timestamp, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		alert.ID, alert.Level, alert.Rule, alert.Message, alert.Timestamp, string(metaBytes))

	if err != nil {
		logger.Error("Failed to save alert to database", err)
		return err
	}
	return nil
}

// QueryRecent 查询最近的告警
func (r *MySQLAlertRepository) QueryRecent(ctx context.Context, limit int) ([]Alert, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, level, rule, message, timestamp, metadata FROM alerts ORDER BY timestamp DESC LIMIT ?",
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// QueryFiltered 带过滤条件查询
func (r *MySQLAlertRepository) QueryFiltered(ctx context.Context, level, rule string, limit, offset int) ([]Alert, int, error) {
	if r.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	// 构建查询条件
	query := "SELECT id, level, rule, message, timestamp, metadata FROM alerts WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM alerts WHERE 1=1"
	var args []interface{}

	if level != "" {
		query += " AND level = ?"
		countQuery += " AND level = ?"
		args = append(args, level)
	}
	if rule != "" {
		query += " AND rule = ?"
		countQuery += " AND rule = ?"
		args = append(args, rule)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"

	// 查询总数
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询数据
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	alerts, err := r.scanAlerts(rows)
	return alerts, total, err
}

// Delete 删除告警
func (r *MySQLAlertRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := r.db.ExecContext(ctx, "DELETE FROM alerts WHERE id = ?", id)
	return err
}

// Count 统计告警数量
func (r *MySQLAlertRepository) Count(ctx context.Context, level, rule string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	query := "SELECT COUNT(*) FROM alerts WHERE 1=1"
	var args []interface{}

	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	if rule != "" {
		query += " AND rule = ?"
		args = append(args, rule)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// scanAlerts 扫描查询结果
func (r *MySQLAlertRepository) scanAlerts(rows *sql.Rows) ([]Alert, error) {
	var alerts []Alert
	for rows.Next() {
		var a Alert
		var metaStr string
		if err := rows.Scan(&a.ID, &a.Level, &a.Rule, &a.Message, &a.Timestamp, &metaStr); err != nil {
			continue
		}
		json.Unmarshal([]byte(metaStr), &a.Metadata)
		alerts = append(alerts, a)
	}
	return alerts, nil
}
