package memory

import (
	"ai-memory/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RuleInfo 规则信息（用于API返回）
type RuleInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Enabled     bool          `json:"enabled"`
	Cooldown    time.Duration `json:"cooldown,omitempty"`
	ConfigJSON  string        `json:"config_json,omitempty"` // 添加配置JSON
	Stats       *RuleStats    `json:"stats,omitempty"`
}

// RuleConfig 规则配置（用于更新）
type RuleConfig struct {
	Threshold int           `json:"threshold,omitempty"`
	Cooldown  time.Duration `json:"cooldown,omitempty"`
}

// GetAllRules 获取所有规则及统计信息（实时从DB读取）
func (ae *AlertEngine) GetAllRules() []RuleInfo {
	// 从数据库加载最新配置
	configs, err := ae.configPersistence.LoadAll(context.Background())
	if err != nil {
		logger.Error("Failed to load rules from DB", err)
		return []RuleInfo{}
	}

	ae.mu.RLock()
	defer ae.mu.RUnlock()

	result := make([]RuleInfo, 0, len(configs))

	// 遍历数据库中的规则配置
	for ruleID, dbConfig := range configs {
		// 查找内存中对应的规则（获取统计信息）
		var stats *RuleStats
		for _, rule := range ae.rules {
			if rule.ID == ruleID {
				stats = ae.getRuleStats(rule.ID)
				break
			}
		}

		if stats == nil {
			stats = &RuleStats{}
		}

		result = append(result, RuleInfo{
			ID:          dbConfig.ID,
			Name:        dbConfig.Name,
			Description: dbConfig.Description,
			Enabled:     dbConfig.Enabled,
			Cooldown:    time.Duration(dbConfig.CooldownSeconds) * time.Second,
			ConfigJSON:  dbConfig.ConfigJSON, // 添加配置JSON
			Stats:       stats,
		})
	}

	return result
}

// GetRuleByID 根据ID获取规则
func (ae *AlertEngine) GetRuleByID(ruleID string) *AlertRule {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	for _, rule := range ae.rules {
		if rule.ID == ruleID {
			return rule
		}
	}
	return nil
}

// ToggleRule 启用/禁用规则
func (ae *AlertEngine) ToggleRule(ruleID string, enabled bool) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	for _, rule := range ae.rules {
		if rule.ID == ruleID {
			rule.Enabled = enabled

			// 持久化到数据库
			if ae.configPersistence != nil {
				ctx := context.Background()
				if err := ae.configPersistence.UpdateEnabled(ctx, ruleID, enabled); err != nil {
					logger.Error("Failed to persist rule enabled status", err)
					// 不返回错误，允许内存修改成功
				}
			}

			return nil
		}
	}
	return fmt.Errorf("rule not found: %s", ruleID)
}

// UpdateRuleCooldown 更新规则冷却时间
func (ae *AlertEngine) UpdateRuleCooldown(ruleID string, cooldown time.Duration) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	for _, rule := range ae.rules {
		if rule.ID == ruleID {
			rule.Cooldown = cooldown

			// 持久化到数据库
			if ae.configPersistence != nil {
				ctx := context.Background()
				cooldownSeconds := int(cooldown.Seconds())
				if err := ae.configPersistence.UpdateCooldown(ctx, ruleID, cooldownSeconds); err != nil {
					logger.Error("Failed to persist rule cooldown", err)
					// 不返回错误，允许内存修改成功
				}
			}

			return nil
		}
	}
	return fmt.Errorf("rule not found: %s", ruleID)
}

// GetStats 获取引擎统计信息
func (ae *AlertEngine) GetStats() *AlertEngineStats {
	ae.stats.mu.RLock()
	defer ae.stats.mu.RUnlock()

	// 复制统计信息
	stats := &AlertEngineStats{
		TotalChecks:   ae.stats.TotalChecks,
		TotalFired:    ae.stats.TotalFired,
		NotifySuccess: ae.stats.NotifySuccess,
		NotifyFailed:  ae.stats.NotifyFailed,
		RuleStats:     make(map[string]*RuleStats),
	}

	for id, rs := range ae.stats.RuleStats {
		rs.mu.RLock()
		stats.RuleStats[id] = &RuleStats{
			TotalFired:       rs.TotalFired,
			LastFiredAt:      rs.LastFiredAt,
			TotalChecks:      rs.TotalChecks,
			AvgCheckDuration: rs.AvgCheckDuration,
		}
		rs.mu.RUnlock()
	}

	return stats
}

// getRuleStats 获取规则统计（内部使用，调用者需持有锁）
func (ae *AlertEngine) getRuleStats(ruleID string) *RuleStats {
	ae.stats.mu.RLock()
	defer ae.stats.mu.RUnlock()

	if stats, ok := ae.stats.RuleStats[ruleID]; ok {
		stats.mu.RLock()
		defer stats.mu.RUnlock()
		return &RuleStats{
			TotalFired:       stats.TotalFired,
			LastFiredAt:      stats.LastFiredAt,
			TotalChecks:      stats.TotalChecks,
			AvgCheckDuration: stats.AvgCheckDuration,
		}
	}
	return &RuleStats{}
}

// recordRuleCheck 记录规则检查（内部使用）
func (ae *AlertEngine) recordRuleCheck(ruleID string, duration time.Duration) {
	ae.stats.mu.Lock()
	defer ae.stats.mu.Unlock()

	if _, ok := ae.stats.RuleStats[ruleID]; !ok {
		ae.stats.RuleStats[ruleID] = &RuleStats{}
	}

	stats := ae.stats.RuleStats[ruleID]
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.TotalChecks++
	// 简单移动平均
	if stats.AvgCheckDuration == 0 {
		stats.AvgCheckDuration = duration
	} else {
		stats.AvgCheckDuration = (stats.AvgCheckDuration + duration) / 2
	}

	// 累积到同步队列
	if ae.statsSync != nil {
		ae.statsSync.RecordCheck()
	}
}

// recordRuleFire 记录规则触发（内部使用）
func (ae *AlertEngine) recordRuleFire(ruleID string) {
	ae.stats.mu.Lock()
	defer ae.stats.mu.Unlock()

	if _, ok := ae.stats.RuleStats[ruleID]; !ok {
		ae.stats.RuleStats[ruleID] = &RuleStats{}
	}

	stats := ae.stats.RuleStats[ruleID]
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.TotalFired++
	stats.LastFiredAt = time.Now()

	ae.stats.TotalFired++
}

// recordNotifyResult 记录通知结果
func (ae *AlertEngine) recordNotifyResult(success bool) {
	ae.stats.mu.Lock()
	defer ae.stats.mu.Unlock()

	if success {
		ae.stats.NotifySuccess++
		if ae.statsSync != nil {
			ae.statsSync.RecordNotifySuccess()
		}
	} else {
		ae.stats.NotifyFailed++
		if ae.statsSync != nil {
			ae.statsSync.RecordNotifyFailed()
		}
	}
}

// GetAlertTrend 获取告警趋势数据
func (ae *AlertEngine) GetAlertTrend(ctx context.Context, hours int) (map[string]interface{}, error) {
	if ae.repository == nil {
		return nil, fmt.Errorf("repository not initialized")
	}

	// 从数据库查询最近N小时的告警
	alerts, err := ae.repository.QueryRecent(ctx, hours*100) // 粗略估算
	if err != nil {
		return nil, err
	}

	// 按小时分组统计
	now := time.Now()
	startTime := now.Add(-time.Duration(hours) * time.Hour).Truncate(time.Hour)

	// 创建时间桶（从startTime到now，每小时一个桶，整点对齐）
	buckets := make(map[string]map[AlertLevel]int)
	timestamps := []string{}

	for i := 0; i <= hours; i++ {
		t := startTime.Add(time.Duration(i) * time.Hour)
		key := t.Format("2006-01-02 15:00") // 整点格式
		buckets[key] = map[AlertLevel]int{
			AlertLevelError:   0,
			AlertLevelWarning: 0,
			AlertLevelInfo:    0,
		}
		timestamps = append(timestamps, key)
	}

	// 统计告警数量
	for _, alert := range alerts {
		// 将UTC时间转换为本地时间
		localTime := alert.Timestamp.Local()

		if localTime.Before(startTime) {
			continue
		}
		if localTime.After(now) {
			continue
		}
		// 将告警时间对齐到整点（本地时区）
		key := localTime.Truncate(time.Hour).Format("2006-01-02 15:00")
		if counts, ok := buckets[key]; ok {
			counts[alert.Level]++
		}
	}

	// 转换为数组格式
	errorCounts := []int{}
	warningCounts := []int{}
	infoCounts := []int{}

	for _, ts := range timestamps {
		counts := buckets[ts]
		errorCounts = append(errorCounts, counts[AlertLevelError])
		warningCounts = append(warningCounts, counts[AlertLevelWarning])
		infoCounts = append(infoCounts, counts[AlertLevelInfo])
	}

	return map[string]interface{}{
		"timestamps": timestamps,
		"error":      errorCounts,
		"warning":    warningCounts,
		"info":       infoCounts,
	}, nil
}

// GetAlertsByLevel 按级别统计告警数量（从DB读取）
func (ae *AlertEngine) GetAlertsByLevel(ctx context.Context) (map[AlertLevel]int, error) {
	if ae.repository == nil {
		return nil, fmt.Errorf("repository not initialized")
	}

	counts := map[AlertLevel]int{
		AlertLevelError:   0,
		AlertLevelWarning: 0,
		AlertLevelInfo:    0,
	}

	// 分别统计各级别的告警数量
	for level := range counts {
		count, err := ae.repository.Count(ctx, string(level), "")
		if err != nil {
			logger.Error("Failed to count alerts", err)
			continue
		}
		counts[level] = count
	}

	return counts, nil
}

// AggregatedAlert 聚合告警
type AggregatedAlert struct {
	Alert
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// aggregationMap 用于告警聚合的临时存储
var (
	aggregationMap   = make(map[string]*AggregatedAlert)
	aggregationMutex sync.RWMutex
)

// getAggregationKey 生成聚合键
func getAggregationKey(alert *Alert) string {
	return fmt.Sprintf("%s:%s", alert.Rule, alert.Level)
}

// aggregateAlert 聚合告警（在冷却期内的相同规则告警会被聚合）
func aggregateAlert(alert *Alert) *AggregatedAlert {
	aggregationMutex.Lock()
	defer aggregationMutex.Unlock()

	key := getAggregationKey(alert)
	if existing, ok := aggregationMap[key]; ok {
		// 更新已有聚合
		existing.Count++
		existing.LastSeen = alert.Timestamp
		existing.Message = alert.Message // 更新为最新消息
		return existing
	}

	// 创建新聚合
	agg := &AggregatedAlert{
		Alert:     *alert,
		Count:     1,
		FirstSeen: alert.Timestamp,
		LastSeen:  alert.Timestamp,
	}
	aggregationMap[key] = agg
	return agg
}

// cleanOldAggregations 清理旧的聚合数据（超过1小时）
func cleanOldAggregations() {
	aggregationMutex.Lock()
	defer aggregationMutex.Unlock()

	now := time.Now()
	for key, agg := range aggregationMap {
		if now.Sub(agg.LastSeen) > time.Hour {
			delete(aggregationMap, key)
		}
	}
}

// GetAggregatedAlerts 获取聚合后的告警
func GetAggregatedAlerts() []*AggregatedAlert {
	aggregationMutex.RLock()
	defer aggregationMutex.RUnlock()

	result := make([]*AggregatedAlert, 0, len(aggregationMap))
	for _, agg := range aggregationMap {
		// 深拷贝
		copy := &AggregatedAlert{
			Alert:     agg.Alert,
			Count:     agg.Count,
			FirstSeen: agg.FirstSeen,
			LastSeen:  agg.LastSeen,
		}
		result = append(result, copy)
	}
	return result
}

// MarshalJSON 自定义JSON序列化（处理Duration）
func (r *RuleStats) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"total_fired":        r.TotalFired,
		"last_fired_at":      r.LastFiredAt,
		"total_checks":       r.TotalChecks,
		"avg_check_duration": r.AvgCheckDuration.String(),
	})
}
