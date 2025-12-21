package api

import (
	"ai-memory/pkg/memory"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// handleGetAlertRules 获取所有规则
func (s *Server) handleGetAlertRules(w http.ResponseWriter, r *http.Request) {
	rules := s.memory.GetAllAlertRules()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
	})
}

// handleToggleAlertRule 启用/禁用规则
func (s *Server) handleToggleAlertRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if ruleID == "" {
		http.Error(w, "Missing rule ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.memory.ToggleAlertRule(ruleID, req.Enabled); err != nil {
		http.Error(w, fmt.Sprintf("Failed to toggle rule: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleUpdateAlertRuleConfig 更新规则配置
func (s *Server) handleUpdateAlertRuleConfig(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if ruleID == "" {
		http.Error(w, "Missing rule ID", http.StatusBadRequest)
		return
	}

	var req struct {
		CooldownMinutes int `json:"cooldown_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cooldown := time.Duration(req.CooldownMinutes) * time.Minute
	if err := s.memory.UpdateAlertRuleCooldown(ruleID, cooldown); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update rule: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleUpdateAlertRuleConfigJSON 更新规则配置JSON
func (s *Server) handleUpdateAlertRuleConfigJSON(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if ruleID == "" {
		http.Error(w, "Missing rule ID", http.StatusBadRequest)
		return
	}

	var req struct {
		ConfigJSON string `json:"config_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.memory.UpdateAlertRuleConfigJSON(ruleID, req.ConfigJSON); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGetAlertStats 获取告警统计信息（带缓存）
func (s *Server) handleGetAlertStats(w http.ResponseWriter, r *http.Request) {
	stats, levelCounts, err := s.memory.GetAlertStatsWithCache(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	// 计算通知成功率
	notifySuccessRate := 0.0
	if stats.NotifySuccess+stats.NotifyFailed > 0 {
		notifySuccessRate = float64(stats.NotifySuccess) / float64(stats.NotifySuccess+stats.NotifyFailed)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_checks":        stats.TotalChecks,
		"total_fired":         stats.TotalFired,
		"notify_success_rate": notifySuccessRate,
		"by_level":            levelCounts,
		"rule_stats":          stats.RuleStats,
	})
}

// handleGetAlertTrend 获取告警趋势
func (s *Server) handleGetAlertTrend(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24 // 默认24小时
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	trend, err := s.memory.GetAlertTrend(r.Context(), hours)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get trend: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(trend)
}

// handleGetAggregatedAlerts 获取聚合告警
func (s *Server) handleGetAggregatedAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := memory.GetAggregatedAlerts()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
	})
}
