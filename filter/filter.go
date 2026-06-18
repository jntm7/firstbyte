package filter

import (
	"sort"
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
