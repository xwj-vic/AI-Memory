package store

import (
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"math"
	"os"
	"sort"
	"sync"
)

// InMemoryVectorStore is a simple, thread-safe in-memory vector database.
type InMemoryVectorStore struct {
	mu      sync.RWMutex
	records []types.Record
}

func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		records: make([]types.Record, 0),
	}
}

// Add stores records in memory.
func (s *InMemoryVectorStore) Add(ctx context.Context, records []types.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, records...)
	return nil
}

// Save persists the records to a file.
func (s *InMemoryVectorStore) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load reads records from a file.
func (s *InMemoryVectorStore) Load(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist, start with empty
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.records)
}

// Delete removes records by ID.
func (s *InMemoryVectorStore) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	newRecords := make([]types.Record, 0, len(s.records))
	for _, rec := range s.records {
		if !idMap[rec.ID] {
			newRecords = append(newRecords, rec)
		}
	}
	s.records = newRecords
	return nil
}

// List retrieves all records.
func (s *InMemoryVectorStore) List(ctx context.Context) ([]types.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	results := make([]types.Record, len(s.records))
	copy(results, s.records)
	return results, nil
}

// Search finds the nearest neighbors using cosine similarity.
func (s *InMemoryVectorStore) Search(ctx context.Context, vector []float32, limit int, scoreThreshold float32, filters map[string]interface{}) ([]types.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type result struct {
		record types.Record
		score  float32
	}

	var results []result

	for _, rec := range s.records {
		if rec.Embedding == nil {
			continue
		}

		// Apply filters
		match := true
		for k, v := range filters {
			// Check metadata
			if val, ok := rec.Metadata[k]; !ok || val != v {
				// Special check for top-level fields if needed, but here assuming metadata filtering
				// If we want to filter by "user_id" which might be in Metadata
				match = false
				break
			}
		}
		if !match {
			continue
		}

		score := cosineSimilarity(vector, rec.Embedding)
		if score >= scoreThreshold {
			results = append(results, result{record: rec, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	finalRecords := make([]types.Record, len(results))
	for i, r := range results {
		finalRecords[i] = r.record
	}

	return finalRecords, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
