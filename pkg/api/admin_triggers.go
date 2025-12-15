package api

import (
	"encoding/json"
	"net/http"
)

// handleTriggerJudge 手动触发STM判定流程
func (s *Server) handleTriggerJudge(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if payload.UserID == "" || payload.SessionID == "" {
		http.Error(w, "user_id and session_id are required", http.StatusBadRequest)
		return
	}

	// 触发判定流程
	if err := s.memory.JudgeAndStageFromSTM(r.Context(), payload.UserID, payload.SessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "STM判定流程已触发",
	})
}

// handleTriggerPromotion 手动触发Staging晋升流程
func (s *Server) handleTriggerPromotion(w http.ResponseWriter, r *http.Request) {
	if err := s.memory.PromoteStagingToLTM(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Staging晋升流程已触发",
	})
}

// handleTriggerDecay 手动触发遗忘扫描
func (s *Server) handleTriggerDecay(w http.ResponseWriter, r *http.Request) {
	if err := s.memory.ScanAndEvictDecayedMemories(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "遗忘扫描已触发",
	})
}
