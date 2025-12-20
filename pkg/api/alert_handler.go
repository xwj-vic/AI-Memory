package api

import (
	"ai-memory/pkg/memory"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// handleGetAlerts 获取告警记录（支持过滤）
func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	// Parse Query Params
	query := r.URL.Query()
	level := query.Get("level")
	rule := query.Get("rule")

	limit := 20
	if lStr := query.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	page := 1
	if pStr := query.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	offset := (page - 1) * limit

	// Call Manager Query
	alerts, total, err := s.memory.QueryAlerts(level, rule, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query alerts: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// handleDeleteAlert 删除告警
func (s *Server) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing alert ID", http.StatusBadRequest)
		return
	}

	if err := s.memory.DeleteAlert(id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}

// handleCreateAlert 手动创建告警
func (s *Server) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var alert memory.Alert
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("manual_%d", time.Now().UnixNano())
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	if err := s.memory.CreateAlert(alert); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create alert: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "id": alert.ID})
}
