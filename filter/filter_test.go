package filter

import (
	"testing"
	"time"
)

// mockHasher implements the Hasher interface for testing.
type mockHasher struct {
	items map[string]bool
}

func (m *mockHasher) Has(key string) bool { return m.items[key] }
func (m *mockHasher) Mark(key string)     { m.items[key] = true }

func newMockHasher(seen ...string) *mockHasher {
	m := &mockHasher{items: make(map[string]bool)}
	for _, s := range seen {
		m.items[s] = true
	}
	return m
}

func makeArticle(link, title, source string, daysAgo int) Article {
	return Article{
		Link:        link,
		Title:       title,
		Source:      source,
		Description: "Description for " + title,
		Published:   time.Now().AddDate(0, 0, -daysAgo),
	}
}

func TestRemoveSeen(t *testing.T) {
	articles := []Article{
		makeArticle("https://a.com/1", "Article 1", "src", 0),
		makeArticle("https://a.com/2", "Article 2", "src", 0),
		makeArticle("https://a.com/3", "Article 3", "src", 0),
	}
	seen := newMockHasher("https://a.com/2") // article 2 already seen

	result := RemoveSeen(articles, seen)

	if len(result) != 2 {
		t.Fatalf("expected 2 fresh articles, got %d", len(result))
	}
	if result[0].Link != "https://a.com/1" {
		t.Errorf("expected article 1, got %s", result[0].Link)
	}
	if result[1].Link != "https://a.com/3" {
		t.Errorf("expected article 3, got %s", result[1].Link)
	}
}

func TestTopN(t *testing.T) {
	articles := []Article{
		makeArticle("", "Old", "src", 5),
		makeArticle("", "New", "src", 1),
		makeArticle("", "Mid", "src", 3),
	}

	result := TopN(articles, 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(result))
	}
	if result[0].Title != "New" {
		t.Errorf("expected newest first, got %q", result[0].Title)
	}
	if result[1].Title != "Mid" {
		t.Errorf("expected second newest, got %q", result[1].Title)
	}
}

func TestTopNMoreThanAvailable(t *testing.T) {
	articles := []Article{
		makeArticle("", "A", "src", 1),
	}

	result := TopN(articles, 10)
	if len(result) != 1 {
		t.Errorf("expected 1 article when N exceeds available, got %d", len(result))
	}
}


