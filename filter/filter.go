package filter

import (
	"sort"
	"strings"
	"time"
)

// Article represents a single item from an RSS feed.
type Article struct {
	Title       string
	Link        string
	Published   time.Time
	Description string
	Source      string // which feed this came from (e.g. "Hacker News")
}

// RemoveSeen returns only articles whose Link is NOT already in the store.
// This prevents the same article from appearing in multiple digests.
func RemoveSeen(articles []Article, seen Hasher) []Article {
	var fresh []Article
	for _, a := range articles {
		if !seen.Has(a.Link) {
			fresh = append(fresh, a)
		}
	}
	return fresh
}

// Hasher is the interface the store must satisfy for duplicate detection.
// Using an interface instead of importing store keeps the packages loosely coupled.
type Hasher interface {
	Has(key string) bool
	Mark(key string)
}

// TopN returns the most recent N articles, sorted by publish date (newest first).
// If there are fewer than N articles, all are returned.
func TopN(articles []Article, n int) []Article {
	// sort by Published descending (newest first)
	sorted := make([]Article, len(articles))
	copy(sorted, articles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Published.After(sorted[j].Published)
	})

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// TopNPerSourceMap returns the most recent articles per source,
// using per-source limits from a map (source name → max articles).
// Sources not in the map default to 5.
func TopNPerSourceMap(articles []Article, limits map[string]int) []Article {
	groups := make(map[string][]Article)
	for _, a := range articles {
		groups[a.Source] = append(groups[a.Source], a)
	}

	var result []Article
	for source, group := range groups {
		n := limits[source]
		if n <= 0 {
			n = 5
		}
		top := TopN(group, n)
		result = append(result, top...)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Published.After(result[j].Published)
	})
	return result
}

// FilterByKeywords removes articles based on global allowlist and blocklist.
// If allowlist is non-empty, an article is kept only if its title or description
// contains at least one allowlist term (case-insensitive).
// If blocklist is non-empty, an article is dropped if its title or description
// contains any blocklist term (case-insensitive).
// Filtering order: allowlist first, then blocklist.
func FilterByKeywords(articles []Article, allowlist, blocklist []string) []Article {
	if len(allowlist) == 0 && len(blocklist) == 0 {
		return articles
	}

	var result []Article
	for _, a := range articles {
		if len(allowlist) > 0 && !matchesAny(a.Title, a.Description, allowlist) {
			continue
		}
		if len(blocklist) > 0 && matchesAny(a.Title, a.Description, blocklist) {
			continue
		}
		result = append(result, a)
	}
	return result
}

// Deduplicate removes articles with duplicate titles across sources.
// The first occurrence of a given title is kept; subsequent duplicates are dropped.
// Comparison is case-insensitive with trimmed whitespace.
func Deduplicate(articles []Article) []Article {
	seen := make(map[string]bool)
	var result []Article
	for _, a := range articles {
		key := normalizeTitle(a.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, a)
	}
	return result
}

// matchesAny checks if any keyword appears in the combined title and description.
func matchesAny(title, desc string, keywords []string) bool {
	text := strings.ToLower(title + " " + desc)
	for _, k := range keywords {
		if strings.Contains(text, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

// normalizeTitle lowercases and trims whitespace for dedup comparison.
func normalizeTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

// FilterByMaxAge drops articles older than maxAgeDays days.
// A value of 0 or less means no filtering.
func FilterByMaxAge(articles []Article, maxAgeDays int) []Article {
	if maxAgeDays <= 0 {
		return articles
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	var result []Article
	for _, a := range articles {
		if a.Published.After(cutoff) {
			result = append(result, a)
		}
	}
	return result
}
