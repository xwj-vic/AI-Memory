package store

import (
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
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
// List retrieves all records with filtering and pagination.
func (s *InMemoryVectorStore) List(ctx context.Context, filter map[string]interface{}, limit int, offset int) ([]types.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []types.Record
	for _, rec := range s.records {
		match := true
		for k, v := range filter {
			// Check specific fields first (type, user_id in metadata)
			if k == "type" {
				if string(rec.Type) != v.(string) {
					match = false
					break
				}
				continue
			}
			// Check metadata
			if val, ok := rec.Metadata[k]; !ok || val != v {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, rec)
		}
	}

	// Apply Offset and Limit
	total := len(filtered)
	if offset >= total {
		return []types.Record{}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// Return a copy
	results := make([]types.Record, end-offset)
	copy(results, filtered[offset:end])
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

// Update modifies an existing record.
func (s *InMemoryVectorStore) Update(ctx context.Context, record types.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.records {
		if r.ID == record.ID {
			// Update fields (Content, Metadata)
			// Preserve Embedding if not provided? Or assume full replacement?
			// Usually assume full replacement or specific merge. For now: Full replacement of fields provided.
			// The interface takes Record.
			s.records[i] = record
			return nil
		}
	}
	return fmt.Errorf("record not found")
}

// Get retrieves a record by ID.
func (s *InMemoryVectorStore) Get(ctx context.Context, id string) (*types.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.records {
		if r.ID == id {
			// Return copy
			c := r
			return &c, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *InMemoryVectorStore) Count(ctx context.Context, filters map[string]interface{}) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, rec := range s.records {
		match := true
		for k, v := range filters {
			// Handle user_id special case if needed, but in-memory usually matches flat metadata or similar
			// Assuming metadata check:
			if k == "user_id" {
				if val, ok := rec.Metadata["user_id"]; !ok || val != v {
					match = false
					break
				}
			} else if k == "type" {
				if string(rec.Type) != v { // Assuming v is string
					match = false
					break
				}
			} else {
				// Genetic metadata match
				if val, ok := rec.Metadata[k]; !ok || val != v {
					match = false
					break
				}
			}
		}
		if match {
			count++
		}
	}
	return count, nil
}
