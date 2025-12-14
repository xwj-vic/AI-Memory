package store

import (
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type InMemoryListStore struct {
	mu    sync.RWMutex
	lists map[string][]string
}

func NewInMemoryListStore() *InMemoryListStore {
	return &InMemoryListStore{
		lists: make(map[string][]string),
	}
}

func (s *InMemoryListStore) RPush(ctx context.Context, key string, values ...interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range values {
		strVal, _ := v.([]byte) // Assuming []byte as per usage in manager
		if strVal == nil {
			// Try marshalling if not byte slice
			b, _ := json.Marshal(v)
			strVal = b
		}
		s.lists[key] = append(s.lists[key], string(strVal))
	}
	return nil
}

func (s *InMemoryListStore) LRange(ctx context.Context, key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list, ok := s.lists[key]
	if !ok {
		return []string{}, nil
	}

	ln := len(list)
	if start < 0 {
		start = ln + start
		if start < 0 {
			start = 0
		}
	}
	if stop < 0 {
		stop = ln + stop
		if stop < 0 {
			stop = -1 // Empty
		}
	}
	if stop >= ln {
		stop = ln - 1
	}

	if start > stop {
		return []string{}, nil
	}

	return list[start : stop+1], nil
}

func (s *InMemoryListStore) Del(ctx context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, k := range keys {
		delete(s.lists, k)
	}
	return nil
}

func (s *InMemoryListStore) Ping(ctx context.Context) error {
	return nil
}

// ScanKeys finds keys matching a pattern.
func (s *InMemoryListStore) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []string
	// Simple glob matching is not implemented here, acting as prefix match or simple contains?
	// Redis SCAN uses glob. For in-memory, we can try to support simple wildcard.
	// NOTE: For now, assuming pattern is "prefix*"
	// In the use case 'memory:stm:<UserID>:*'

	// Very simple glob implementation
	// If pattern ends with '*', we treat it as prefix.
	// Simple wildcard support for test/dev
	// If pattern is "memory:stm:*:*", we match any key starting with "memory:stm:"

	prefix := pattern
	// Remove trailing *
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix = pattern[:len(pattern)-1]
	}

	// Handle the specific middle wildcard case used in Manager "memory:stm:*:*"
	// This becomes "memory:stm:*:" in prefix logic above.
	// We want to match "memory:stm:u1:s1".
	// Hack: if prefix contains "*", truncate at first "*".
	for i, c := range prefix {
		if c == '*' {
			prefix = prefix[:i]
			break
		}
	}

	for k := range s.lists {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			matched = append(matched, k)
		}
	}
	return matched, nil
}

// Update modifies a record.
func (s *InMemoryListStore) Update(ctx context.Context, record types.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, items := range s.lists {
		for i, itemStr := range items {
			var current types.Record
			if err := json.Unmarshal([]byte(itemStr), &current); err == nil {
				if current.ID == record.ID {
					// Update
					data, _ := json.Marshal(record)
					s.lists[key][i] = string(data)
					return nil
				}
			}
		}
	}
	return fmt.Errorf("record not found")
}

// Get retrieves a record.
func (s *InMemoryListStore) Get(ctx context.Context, id string) (*types.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, items := range s.lists {
		for _, itemStr := range items {
			var current types.Record
			if err := json.Unmarshal([]byte(itemStr), &current); err == nil {
				if current.ID == id {
					return &current, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("record not found")
}
