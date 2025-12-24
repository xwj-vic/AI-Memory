package memory

import (
	"ai-memory/pkg/types"
	"context"
	"time"
)

// Memory is the high-level interface for interacting with the agent's memory.
type Memory interface {
	// Add stores a new interaction or observation.
	Add(ctx context.Context, userID string, sessionID string, input string, output string, metadata map[string]interface{}) error

	// Retrieve finds relevant memories based on a query.
	Retrieve(ctx context.Context, userID string, sessionID string, query string, limit int) ([]types.Record, error)

	// List retrieves all long-term memories (for admin).
	List(ctx context.Context, filter map[string]interface{}, limit int, offset int) ([]types.Record, error)

	// Delete removes a memory record by ID.
	Delete(ctx context.Context, id string) error

	// Clear resets the memory storage.
	Clear(ctx context.Context, userID string, sessionID string) error
}

// VectorStore abstracts the underlying vector database.
type VectorStore interface {
	// Search finds the nearest neighbors to the given vector.
	Search(ctx context.Context, vector []float32, limit int, scoreThreshold float32, filters map[string]interface{}) ([]types.Record, error)

	// Add stores records with their embeddings.
	Add(ctx context.Context, records []types.Record) error

	// Delete removes records by ID.
	Delete(ctx context.Context, ids []string) error

	// List retrieves all records (for summarization/cleanup).
	List(ctx context.Context, filter map[string]interface{}, limit int, offset int) ([]types.Record, error)

	// Update modifies an existing record.
	Update(ctx context.Context, record types.Record) error

	// Get retrieves a record by ID.
	Get(ctx context.Context, id string) (*types.Record, error)
	// Count returns the number of records matching a filter.
	Count(ctx context.Context, filter map[string]interface{}) (int64, error)
}

// KVStore abstracts key-value storage for metadata or raw logs.
type KVStore interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}) error
}

// ListStore abstracts list-based storage for Short-Term Memory (Session).
type ListStore interface {
	RPush(ctx context.Context, key string, values ...interface{}) error
	RPushWithExpire(ctx context.Context, key string, expirationDays int, values ...interface{}) error
	LRange(ctx context.Context, key string, start, stop int) ([]string, error)
	LRem(ctx context.Context, key string, count int64, value interface{}) error
	Del(ctx context.Context, keys ...string) error
	// ScanKeys returns keys matching a pattern (e.g. for finding user sessions).
	ScanKeys(ctx context.Context, pattern string) ([]string, error)

	// Update modifies a record in the list by ID.
	Update(ctx context.Context, record types.Record) error

	// Get searches for a record by ID across all lists.
	Get(ctx context.Context, id string) (*types.Record, error)

	// Set operations for judged record tracking
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
	SAdd(ctx context.Context, key string, members ...interface{}) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// EndUserStore 持久化层接口（for end_users table）
type EndUserStore interface {
	UpsertUser(ctx context.Context, identifier string) error
	ListUsers(ctx context.Context) ([]types.EndUser, error)
}

// Embedder abstracts the text embedding model provider.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}
