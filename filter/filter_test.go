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

func TestTopNPerSource(t *testing.T) {
	articles := []Article{
		makeArticle("", "HN-New", "Hacker News", 1),
		makeArticle("", "HN-Old", "Hacker News", 5),
		makeArticle("", "EL-New", "Electrek", 0),
		makeArticle("", "EL-Mid", "Electrek", 2),
		makeArticle("", "EL-Old", "Electrek", 4),
	}

	result := TopNPerSource(articles, 2)

	// should get top 2 from Hacker News + top 2 from Electrek = 4 total
	if len(result) != 4 {
		t.Fatalf("expected 4 articles (2 per source), got %d", len(result))
	}

	// sources should be sorted by date (Electrek had a day-0 article, so it should be first overall)
	if result[0].Source != "Electrek" {
		t.Errorf("expected Electrek first (newest), got %s", result[0].Source)
	}
}

func TestByKeywordAllowlist(t *testing.T) {
	articles := []Article{
		{Title: "Apple releases new iPhone"},
		{Title: "Google updates Chrome"},
		{Title: "Microsoft patches Windows"},
	}

	result := ByKeyword(articles, []string{"Apple", "Microsoft"}, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result))
	}
}

func TestByKeywordBlocklist(t *testing.T) {
	articles := []Article{
		{Title: "Apple releases new iPhone"},
		{Title: "Sponsored: Buy this product"},
		{Title: "Microsoft patches Windows"},
	}

	result := ByKeyword(articles, nil, []string{"Sponsored"})

	if len(result) != 2 {
		t.Fatalf("expected 2 after blocklist, got %d", len(result))
	}
}

func TestByKeywordCaseInsensitive(t *testing.T) {
	articles := []Article{
		{Title: "APPLE RELEASES NEW IPHONE"},
	}

	result := ByKeyword(articles, []string{"apple"}, nil)
	if len(result) != 1 {
		t.Error("expected case-insensitive match")
	}
}

func TestByKeywordInDescription(t *testing.T) {
	articles := []Article{
		{Title: "Daily Roundup", Description: "Covering apple news and more"},
	}

	result := ByKeyword(articles, []string{"apple"}, nil)
	if len(result) != 1 {
		t.Error("expected match in description")
	}
}

func TestDigest(t *testing.T) {
	articles := []Article{
		makeArticle("https://hn.com/1", "HN Article 1", "Hacker News", 1),
		makeArticle("https://hn.com/2", "HN Article 2", "Hacker News", 2),
		makeArticle("https://hn.com/3", "Sponsored Post", "Hacker News", 3),
	}

	seen := newMockHasher()

	result := Digest(articles, seen, nil, []string{"Sponsored"}, 2)

	if len(result) != 2 {
		t.Fatalf("expected 2 in digest, got %d", len(result))
	}

	// verify articles were marked as seen
	if !seen.Has("https://hn.com/1") {
		t.Error("expected article 1 to be marked as seen")
	}
	if !seen.Has("https://hn.com/2") {
		t.Error("expected article 2 to be marked as seen")
	}
	if seen.Has("https://hn.com/3") {
		t.Error("sponsored article should not be marked as seen (it was filtered out)")
	}
}

func TestDigestDeduplicatesSeen(t *testing.T) {
	articles := []Article{
		makeArticle("https://hn.com/1", "HN Article 1", "Hacker News", 1),
		makeArticle("https://hn.com/2", "HN Article 2", "Hacker News", 2),
	}

	// article 1 was already sent in a previous run
	seen := newMockHasher("https://hn.com/1")

	result := Digest(articles, seen, nil, nil, 5)

	if len(result) != 1 {
		t.Fatalf("expected 1 new article, got %d", len(result))
	}
	if result[0].Link != "https://hn.com/2" {
		t.Errorf("expected article 2, got %s", result[0].Link)
	}
}
