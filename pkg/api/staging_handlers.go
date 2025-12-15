package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleGetStagingEntries 获取暂存区记忆列表
func (s *Server) handleGetStagingEntries(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")

	entries, err := s.memory.GetStagingEntries(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get staging entries: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleConfirmStaging 确认并晋升暂存区记忆
func (s *Server) handleConfirmStaging(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("id")
	if entryID == "" {
		http.Error(w, "Missing entry ID", http.StatusBadRequest)
		return
	}

	if err := s.memory.ConfirmStagingEntry(r.Context(), entryID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to confirm: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "confirmed",
		"message": "记忆已晋升到长期记忆",
	})
}

// handleRejectStaging 拒绝暂存区记忆
func (s *Server) handleRejectStaging(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("id")
	if entryID == "" {
		http.Error(w, "Missing entry ID", http.StatusBadRequest)
		return
	}

	if err := s.memory.RejectStagingEntry(r.Context(), entryID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reject: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "rejected",
		"message": "记忆已删除",
	})
}

// handleGetStagingStats 获取暂存区统计信息
func (s *Server) handleGetStagingStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.memory.GetStagingStats(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}
