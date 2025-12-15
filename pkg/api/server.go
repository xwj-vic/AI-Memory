package api

import (
	"ai-memory/pkg/auth"
	"ai-memory/pkg/logger"
	"ai-memory/pkg/memory"
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	auth   *auth.Service
	memory *memory.Manager
	mux    *http.ServeMux
}

func NewServer(auth *auth.Service, mem *memory.Manager) *Server {
	s := &Server{
		auth:   auth,
		memory: mem,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// API Group
	s.mux.HandleFunc("/api/login", s.handleLogin)

	// Protected Routes (TODO: Add Middleware)
	// Protected Routes (TODO: Add Middleware)
	s.mux.HandleFunc("GET /api/memories", s.handleListMemories)
	s.mux.HandleFunc("POST /api/memories", s.handleAddMemory)
	s.mux.HandleFunc("PUT /api/memories/{id}", s.handleUpdateMemory)
	s.mux.HandleFunc("POST /api/retrieve", s.handleRetrieveMemory)
	s.mux.HandleFunc("DELETE /api/memories/{id}", s.handleDeleteMemory)

	// Admin Endpoints
	s.mux.HandleFunc("GET /api/users", s.handleGetUsers)
	s.mux.HandleFunc("GET /api/status", s.handleGetStatus)

	// Staging审核API
	s.mux.HandleFunc("GET /api/staging", s.handleGetStagingEntries)
	s.mux.HandleFunc("POST /api/staging/{id}/confirm", s.handleConfirmStaging)
	s.mux.HandleFunc("POST /api/staging/{id}/reject", s.handleRejectStaging)
	s.mux.HandleFunc("GET /api/staging/stats", s.handleGetStagingStats)

	// 监控指标API
	s.mux.HandleFunc("GET /api/metrics", s.handleGetMetrics)
	s.mux.HandleFunc("GET /api/dashboard/metrics", s.handleGetDashboardMetrics)

	// 管理触发器API（手动触发漏斗流程）
	s.mux.HandleFunc("POST /api/admin/trigger-judge", s.handleTriggerJudge)
	s.mux.HandleFunc("POST /api/admin/trigger-promotion", s.handleTriggerPromotion)
	s.mux.HandleFunc("POST /api/admin/trigger-decay", s.handleTriggerDecay)

	// Static Files (Frontend) - Must be last to avoid catching API routes if not specific
	fs := http.FileServer(http.Dir("./frontend/dist"))
	s.mux.Handle("/", fs)
}

func (s *Server) handleUpdateMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing memory ID", http.StatusBadRequest)
		return
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.memory.Update(r.Context(), id, payload.Content); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.memory.GetUsers(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch users: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := s.memory.GetSystemStatus(r.Context())
	json.NewEncoder(w).Encode(status)
}

func (s *Server) Start(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	logger.System("Starting Admin API", "addr", addr) // Changed from log.Printf
	return server.ListenAndServe()
}

// Handlers

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, err := s.auth.Authenticate(creds.Username, creds.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// TODO: Issue JWT or Session. For now, just return OK + User info
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login successful",
		"user":    user,
	})
}

func (s *Server) handleListMemories(w http.ResponseWriter, r *http.Request) {
	// Parse Query Params
	query := r.URL.Query()
	userID := query.Get("user_id")
	memType := query.Get("type")

	page := 1
	if pStr := query.Get("page"); pStr != "" {
		fmt.Sscanf(pStr, "%d", &page)
	}

	limit := 50
	if lStr := query.Get("limit"); lStr != "" {
		fmt.Sscanf(lStr, "%d", &limit)
	}

	filter := memory.Filter{
		UserID: userID,
		Type:   memType,
		Page:   page,
		Limit:  limit,
	}

	memories, err := s.memory.List(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list memories: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"memories": memories,
		"page":     page,
		"limit":    limit,
	})
}

func (s *Server) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing memory ID", http.StatusBadRequest)
		return
	}

	if err := s.memory.Delete(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete memory: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleAddMemory(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
		Input     string `json:"input"`
		Output    string `json:"output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.memory.Add(r.Context(), payload.UserID, payload.SessionID, payload.Input, payload.Output, nil); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add memory: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleRetrieveMemory(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
		Query     string `json:"query"`
		Limit     int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	results, err := s.memory.Retrieve(r.Context(), payload.UserID, payload.SessionID, payload.Query, payload.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}
