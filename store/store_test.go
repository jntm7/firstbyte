package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if len(s.items) != 0 {
		t.Errorf("expected empty store, got %d items", len(s.items))
	}
}

func TestMarkAndHas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// initially, nothing is seen
	if s.Has("https://example.com/article-1") {
		t.Error("expected Has() to return false for new item")
	}

	// mark an article as seen
	s.Mark("https://example.com/article-1")

	if !s.Has("https://example.com/article-1") {
		t.Error("expected Has() to return true after Mark()")
	}
	if s.Has("https://example.com/article-2") {
		t.Error("expected Has() to return false for unmarked item")
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	// create a store, mark some items, save
	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	s.Mark("https://example.com/a")
	s.Mark("https://example.com/b")
	s.Mark("https://example.com/c")

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// verify the file exists on disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected store file to exist after Save()")
	}

	// reload the store from disk
	s2, err := New(path)
	if err != nil {
		t.Fatalf("New() reload error: %v", err)
	}

	if len(s2.items) != 3 {
		t.Errorf("expected 3 items after reload, got %d", len(s2.items))
	}
	if !s2.Has("https://example.com/a") {
		t.Error("expected item 'a' to persist across reload")
	}
	if !s2.Has("https://example.com/b") {
		t.Error("expected item 'b' to persist across reload")
	}
	if !s2.Has("https://example.com/c") {
		t.Error("expected item 'c' to persist across reload")
	}
}

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// mark from multiple goroutines simultaneously
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			s.Mark("https://example.com/item")
			done <- true
		}(i)
	}

	// wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// should have exactly 1 item (same key marked 10 times)
	if len(s.items) != 1 {
		t.Errorf("expected 1 item after concurrent marks, got %d", len(s.items))
	}
}
