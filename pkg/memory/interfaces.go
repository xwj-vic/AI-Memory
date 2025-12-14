package memory

import (
	"ai-memory/pkg/types"
	"context"
)

// Memory is the high-level interface for interacting with the agent's memory.
type Memory interface {
	// Add stores a new interaction or observation.
	Add(ctx context.Context, input string, output string, metadata map[string]interface{}) error

	// Retrieve finds relevant memories based on a query.
	Retrieve(ctx context.Context, query string, limit int) ([]types.Record, error)

	// Summarize triggers a consolidation of short-term memories into long-term storage.
	Summarize(ctx context.Context) error

	// Clear resets the memory storage.
	Clear(ctx context.Context) error
}

// VectorStore abstracts the underlying vector database.
type VectorStore interface {
	// Search finds the nearest neighbors to the given vector.
	Search(ctx context.Context, vector []float32, limit int, scoreThreshold float32) ([]types.Record, error)

	// Add stores records with their embeddings.
	Add(ctx context.Context, records []types.Record) error

	// Delete removes records by ID.
	Delete(ctx context.Context, ids []string) error

	// List retrieves all records (for summarization/cleanup).
	List(ctx context.Context) ([]types.Record, error)
}

// KVStore abstracts key-value storage for metadata or raw logs.
type KVStore interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}) error
}

// ListStore abstracts list-based storage for Short-Term Memory (Session).
type ListStore interface {
	RPush(ctx context.Context, key string, values ...interface{}) error
	LRange(ctx context.Context, key string, start, stop int) ([]string, error)
	Del(ctx context.Context, keys ...string) error
}

// Embedder abstracts the text embedding model provider.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}
