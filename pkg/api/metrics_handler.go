package api

import (
	"encoding/json"
	"net/http"
)

// handleGetMetrics 获取性能监控指标
func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.memory.GetPerformanceMetrics(r.Context())

	// 添加系统状态
	status := s.memory.GetSystemStatus(r.Context())

	combined := map[string]interface{}{
		"performance": metrics,
		"system":      status,
	}

	json.NewEncoder(w).Encode(combined)
}
