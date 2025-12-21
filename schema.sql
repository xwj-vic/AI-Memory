-- 1. Create Database (Must be done manually or ensure it exists)
CREATE DATABASE IF NOT EXISTS ai_memory;

-- 2. Use the Database
USE ai_memory;

-- 3. Create Users Table (handled by app auto-migration, but here for reference)
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Initial Admin User (Optional, app creates this on startup if missing)
-- Password: admin123
INSERT INTO users (username, password_hash) VALUES ('admin', '$2a$10$FrMJpuNsOfEY.5edOFDoSOWsswfLVPG.MutG8xNcXtcZrc75YyYxu') ON DUPLICATE KEY UPDATE id=id;

-- 5. End Users Table (Tracks users interacting with the AI)
CREATE TABLE IF NOT EXISTS end_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_identifier VARCHAR(255) NOT NULL UNIQUE,
    last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 6. 监控指标时间序列表
CREATE TABLE IF NOT EXISTS metrics_timeseries (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    metric_type VARCHAR(50) NOT NULL COMMENT '指标类型: promotion, queue_length, cache_hit_rate',
    value FLOAT NOT NULL COMMENT '指标值',
    category VARCHAR(50) DEFAULT NULL COMMENT '分类标签（如记忆分类）',
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '记录时间',
    INDEX idx_type_time (metric_type, timestamp),
    INDEX idx_timestamp (timestamp)
) COMMENT='监控指标时间序列数据（支持24小时+长期趋势分析）';

-- 7. 监控指标累计统计表
CREATE TABLE IF NOT EXISTS metrics_cumulative (
    id INT AUTO_INCREMENT PRIMARY KEY,
    total_promotions BIGINT DEFAULT 0 COMMENT '总晋升次数',
    total_rejections BIGINT DEFAULT 0 COMMENT '总拒绝次数',
    total_forgotten BIGINT DEFAULT 0 COMMENT '总遗忘数量',
    cache_hits BIGINT DEFAULT 0 COMMENT '缓存命中次数',
    cache_misses BIGINT DEFAULT 0 COMMENT '缓存未命中次数',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间'
) COMMENT='监控指标累计统计（用于服务重启后恢复数据）';

-- 初始化累计统计表（确保有一条记录）
INSERT INTO metrics_cumulative (id, total_promotions, total_rejections, total_forgotten, cache_hits, cache_misses)
VALUES (1, 0, 0, 0, 0, 0)
ON DUPLICATE KEY UPDATE id=id;

-- 8. 告警记录表
CREATE TABLE IF NOT EXISTS alerts (
    id VARCHAR(64) PRIMARY KEY COMMENT '告警唯一ID',
    level VARCHAR(32) COMMENT '告警级别: INFO, WARNING, ERROR',
    rule VARCHAR(64) COMMENT '触发规则ID',
    message TEXT COMMENT '告警消息内容',
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '告警时间',
    metadata TEXT COMMENT '元数据(JSON格式)',
    INDEX idx_timestamp (timestamp)
) COMMENT='告警历史记录';

-- 9. 告警规则配置表
CREATE TABLE IF NOT EXISTS alert_rule_configs (
    id VARCHAR(64) PRIMARY KEY COMMENT '规则ID（如queue_backlog）',
    name VARCHAR(255) NOT NULL COMMENT '规则名称',
    description TEXT COMMENT '规则描述',
    enabled BOOLEAN DEFAULT TRUE COMMENT '是否启用',
    cooldown_seconds INT DEFAULT 600 COMMENT '冷却时间（秒）',
    config_json TEXT COMMENT '规则特定配置（JSON格式，如阈值）',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后修改时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间'
) COMMENT='告警规则配置持久化（支持动态修改后重启保留）';

-- 插入默认规则配置
INSERT INTO alert_rule_configs (id, name, description, enabled, cooldown_seconds, config_json) VALUES
('queue_backlog', '队列积压告警', 'Staging队列长度超过阈值', TRUE, 600, '{"threshold": 100}'),
('low_success_rate', '晋升成功率过低', '记忆晋升成功率低于阈值', TRUE, 1800, '{"threshold": 60}'),
('cache_anomaly', '缓存命中率异常', '判定缓存命中率异常（智能检测）', TRUE, 900, '{"window_minutes": 5, "min_samples": 500, "warn_threshold": 30, "error_threshold": 15}'),
('decay_spike', '记忆衰减突增', '遗忘的记忆数量突然增加', TRUE, 3600, '{"threshold": 1000}')
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- 10. 告警统计数据表
CREATE TABLE IF NOT EXISTS alert_stats (
    id INT PRIMARY KEY DEFAULT 1 COMMENT '唯一ID（单行表）',
    total_checks BIGINT DEFAULT 0 COMMENT '总规则检查次数',
    notify_success BIGINT DEFAULT 0 COMMENT '通知成功次数',
    notify_failed BIGINT DEFAULT 0 COMMENT '通知失败次数',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
    CONSTRAINT chk_single_row CHECK (id = 1)
) COMMENT='告警引擎统计数据（支持重启后恢复）';

-- 初始化统计表（确保有一条记录）
INSERT INTO alert_stats (id, total_checks, notify_success, notify_failed)
VALUES (1, 0, 0, 0)
ON DUPLICATE KEY UPDATE id=id;
