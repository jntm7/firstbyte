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

func TestFilterByKeywords(t *testing.T) {
	articles := []Article{
		makeArticle("https://a.com/1", "Apple launches new MacBook", "src", 0),
		makeArticle("https://a.com/2", "Microsoft announces Copilot update", "src", 0),
		makeArticle("https://a.com/3", "Google unveils Gemini Pro", "src", 0),
		makeArticle("https://a.com/4", "Sponsored: Best VPN deals", "src", 0),
	}

	t.Run("allowlist", func(t *testing.T) {
		result := FilterByKeywords(articles, []string{"Apple", "Google"}, nil)
		if len(result) != 2 {
			t.Fatalf("expected 2 articles, got %d", len(result))
		}
		if result[0].Title != "Apple launches new MacBook" {
			t.Errorf("expected first Apple article, got %q", result[0].Title)
		}
	})

	t.Run("blocklist", func(t *testing.T) {
		result := FilterByKeywords(articles, nil, []string{"Sponsored"})
		if len(result) != 3 {
			t.Fatalf("expected 3 articles, got %d", len(result))
		}
	})

	t.Run("allowlist and blocklist", func(t *testing.T) {
		result := FilterByKeywords(articles, []string{"Apple", "Microsoft"}, []string{"Mac"})
		if len(result) != 1 {
			t.Fatalf("expected 1 article (Microsoft), got %d", len(result))
		}
		if result[0].Title != "Microsoft announces Copilot update" {
			t.Errorf("expected Microsoft article, got %q", result[0].Title)
		}
	})

	t.Run("no filters", func(t *testing.T) {
		result := FilterByKeywords(articles, nil, nil)
		if len(result) != 4 {
			t.Fatalf("expected 4 articles, got %d", len(result))
		}
	})

	t.Run("empty allowlist", func(t *testing.T) {
		result := FilterByKeywords(articles, []string{}, nil)
		if len(result) != 4 {
			t.Fatalf("expected 4 articles with empty allowlist, got %d", len(result))
		}
	})
}

func TestDeduplicate(t *testing.T) {
	articles := []Article{
		makeArticle("https://a.com/1", "Apple launches new MacBook", "Hacker News", 0),
		makeArticle("https://b.com/1", "Microsoft announces Copilot", "Lobsters", 0),
		makeArticle("https://a.com/2", "Apple launches new MacBook", "Lobsters", 0),
		makeArticle("https://c.com/1", "Google unveils Gemini Pro", "Hacker News", 0),
		makeArticle("https://c.com/2", "apple launches new macbook", "TechCrunch", 0),
	}

	result := Deduplicate(articles)

	if len(result) != 3 {
		t.Fatalf("expected 3 unique articles, got %d", len(result))
	}
	// first occurrence of duplicate should be kept (Hacker News version)
	if result[0].Link != "https://a.com/1" {
		t.Errorf("expected first duplicate kept HN link, got %q", result[0].Link)
	}
	if result[1].Title != "Microsoft announces Copilot" {
		t.Errorf("expected Microsoft article second, got %q", result[1].Title)
	}
	if result[2].Title != "Google unveils Gemini Pro" {
		t.Errorf("expected Google article third, got %q", result[2].Title)
	}
}

func TestDeduplicateNoDupes(t *testing.T) {
	articles := []Article{
		makeArticle("https://a.com/1", "Apple launches new MacBook", "HN", 0),
		makeArticle("https://b.com/1", "Google unveils Gemini Pro", "HN", 0),
	}

	result := Deduplicate(articles)
	if len(result) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(result))
	}
}



func TestFilterByMaxAge(t *testing.T) {
	articles := []Article{
		makeArticle("https://a.com/1", "Today's article", "src", 0),
		makeArticle("https://a.com/2", "Two days old", "src", 2),
		makeArticle("https://a.com/3", "Five days old", "src", 5),
		makeArticle("https://a.com/4", "One day old", "src", 1),
	}

	t.Run("keep last 3 days", func(t *testing.T) {
		result := FilterByMaxAge(articles, 3)
		if len(result) != 3 {
			t.Fatalf("expected 3 articles, got %d", len(result))
		}
		if result[0].Title != "Today's article" {
			t.Errorf("expected newest first, got %q", result[0].Title)
		}
	})

	t.Run("no limit", func(t *testing.T) {
		result := FilterByMaxAge(articles, 0)
		if len(result) != 4 {
			t.Fatalf("expected 4 articles with no limit, got %d", len(result))
		}
	})

	t.Run("negative value", func(t *testing.T) {
		result := FilterByMaxAge(articles, -1)
		if len(result) != 4 {
			t.Fatalf("expected 4 articles with negative limit, got %d", len(result))
		}
	})

	t.Run("all filtered out", func(t *testing.T) {
		result := FilterByMaxAge(articles, 0)
		if len(result) != 4 {
			t.Fatalf("expected 4 articles with limit 0, got %d", len(result))
		}
	})
}
