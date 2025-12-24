package types

import "time"

// MemoryType defines the category of a memory record.
type MemoryType string

const (
	ShortTerm MemoryType = "short_term"
	LongTerm  MemoryType = "long_term"
	Entity    MemoryType = "entity"
	Staging   MemoryType = "staging" // 暂存区
)

// MemoryCategory defines the semantic category of a memory (for LLM judgment).
type MemoryCategory string

const (
	CategoryFact       MemoryCategory = "fact"       // 事实性信息
	CategoryPreference MemoryCategory = "preference" // 用户偏好
	CategoryGoal       MemoryCategory = "goal"       // 长期目标
	CategoryNoise      MemoryCategory = "noise"      // 无价值信息
)

// StagingStatus defines the state of a staging entry.
type StagingStatus string

const (
	StagingPending   StagingStatus = "pending"   // 等待晋升
	StagingConfirmed StagingStatus = "confirmed" // 用户确认
	StagingRejected  StagingStatus = "rejected"  // 用户拒绝
)

// Record represents a single unit of memory.
type Record struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Embedding []float32              `json:"-"` // Stored in vector DB, not always needed in JSON
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
	Type      MemoryType             `json:"type"`
}

// LTMMetadata 长期记忆的增强元数据结构
type LTMMetadata struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// 结构化标签（强制提取）
	Tags     []string          `json:"tags"`     // ["golang", "偏好", "技术栈"]
	Entities map[string]string `json:"entities"` // {"语言": "Go", "框架": "Gin"}
	Category MemoryCategory    `json:"category"` // fact/preference/goal

	// 生命周期管理
	LastAccessAt time.Time `json:"last_access_at"` // 最后访问时间
	AccessCount  int       `json:"access_count"`   // 访问频次
	DecayScore   float64   `json:"decay_score"`    // 衰减分数 (1.0→0)

	// 来源追踪
	SourceType       string  `json:"source_type"`       // staging/manual/legacy
	ConfidenceOrigin float64 `json:"confidence_origin"` // 写入时的信心分数
}

// StagingEntry 暂存区条目（候选记忆）
type StagingEntry struct {
	ID                string            `json:"id"`
	Content           string            `json:"content"`
	Embedding         []float32         `json:"embedding,omitempty"` // 新增：用于语义去重
	UserID            string            `json:"user_id"`
	FirstSeenAt       time.Time         `json:"first_seen_at"`
	LastSeenAt        time.Time         `json:"last_seen_at"`
	OccurrenceCount   int               `json:"occurrence_count"`
	ValueScore        float64           `json:"value_score"`
	ConfidenceScore   float64           `json:"confidence_score"`
	Category          MemoryCategory    `json:"category"`
	ExtractedTags     []string          `json:"extracted_tags"`
	ExtractedEntities map[string]string `json:"extracted_entities"`
	Status            StagingStatus     `json:"status"`
	ConfirmedBy       string            `json:"confirmed_by"` // auto/user
	SessionIDs        []string          `json:"session_ids"`  // 记录所有触达过该事实的会话
}

// JudgeResult LLM判定模型的输出
type JudgeResult struct {
	ValueScore      float64           `json:"value_score"`      // 综合价值分数 (0-1)
	ConfidenceScore float64           `json:"confidence_score"` // 判定信心 (0-1)
	Category        MemoryCategory    `json:"category"`
	Reason          string            `json:"reason"`       // 判定理由
	Tags            []string          `json:"tags"`         // 提取的标签
	Entities        map[string]string `json:"entities"`     // 实体映射
	ShouldStage     bool              `json:"should_stage"` // 是否应进入暂存区
	IsCritical      bool              `json:"is_critical"`  // 是否属于关键事实/强烈意图（可直接晋升LTM）
}

// RecallOptions 增强的召回查询选项
type RecallOptions struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`

	// Metadata过滤
	RequiredTags   []string       `json:"required_tags,omitempty"`
	ExcludedTags   []string       `json:"excluded_tags,omitempty"`
	CategoryFilter MemoryCategory `json:"category_filter,omitempty"`
	MinDecayScore  float64        `json:"min_decay_score,omitempty"`

	// 时间范围
	TimeRangeStart *time.Time `json:"time_range_start,omitempty"`
	TimeRangeEnd   *time.Time `json:"time_range_end,omitempty"`
}

// EndUser represents a user interacting with the AI.
type EndUser struct {
	ID             int       `json:"id"`
	UserIdentifier string    `json:"user_identifier"`
	LastActive     time.Time `json:"last_active"`
	CreatedAt      time.Time `json:"created_at"`
	// Stats (not in DB)
	SessionCount int `json:"session_count"`
	LTMCount     int `json:"ltm_count"`
}
