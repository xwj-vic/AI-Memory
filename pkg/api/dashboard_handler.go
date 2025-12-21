package api

import (
	"encoding/json"
	"net/http"
)

// handleGetDashboardMetrics 获取Dashboard监控指标（包含图表数据）
func (s *Server) handleGetDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	// 解析时间范围参数（支持 1h/24h/7d/30d）
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}
	metrics := s.memory.GetDashboardMetrics(r.Context(), timeRange)
	json.NewEncoder(w).Encode(metrics)
}
