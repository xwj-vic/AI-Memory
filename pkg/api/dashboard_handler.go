package api

import (
	"encoding/json"
	"net/http"
)

// handleGetDashboardMetrics 获取Dashboard监控指标（包含图表数据）
func (s *Server) handleGetDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.memory.GetDashboardMetrics(r.Context())
	json.NewEncoder(w).Encode(metrics)
}
