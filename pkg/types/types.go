package types

import "time"

// MemoryType defines the category of a memory record.
type MemoryType string

const (
	ShortTerm MemoryType = "short_term"
	LongTerm  MemoryType = "long_term"
	Entity    MemoryType = "entity"
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
