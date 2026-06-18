package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Store tracks seen articles to prevent duplicates across runs.
// It is safe for concurrent use.
type Store struct {
	mu    sync.Mutex
	path  string
	items map[string]bool // keyed by article link or GUID
}

// New creates a Store backed by the file at path.
// If the file already exists, it loads previously seen items.
func New(path string) (*Store, error) {
	s := &Store{
		path:  path,
		items: make(map[string]bool),
	}

	// load existing store file if present
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil // no existing store, start fresh
		}
		return nil, fmt.Errorf("read store file: %w", err)
	}

	if err := json.Unmarshal(data, &s.items); err != nil {
		return nil, fmt.Errorf("parse store file: %w", err)
	}

	return s, nil
}

// Has returns true if the article with the given key has already been seen.
func (s *Store) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.items[key]
}

// Mark records an article as seen. Call Save to persist to disk.
func (s *Store) Mark(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = true
}

// Save persists the current set of seen items to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(s.items)
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write store file: %w", err)
	}

	return nil
}


