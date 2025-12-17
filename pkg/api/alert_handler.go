package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleGetAlerts 获取最近的告警记录
func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	// 解析limit参数
	limit := 20 // 默认20条
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	alerts := s.memory.GetRecentAlerts(limit)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}
