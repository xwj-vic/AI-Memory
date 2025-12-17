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
