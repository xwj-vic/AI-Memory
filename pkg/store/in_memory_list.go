package store

import (
	"context"
	"encoding/json"
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
