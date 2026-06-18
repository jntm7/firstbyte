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

// TopNPerSource returns the most recent N articles for each source.
func TopNPerSource(articles []Article, n int) []Article {
	// group articles by source
	groups := make(map[string][]Article)
	for _, a := range articles {
		groups[a.Source] = append(groups[a.Source], a)
	}

	// collect top N from each group
	var result []Article
	for _, group := range groups {
		top := TopN(group, n)
		result = append(result, top...)
	}

	// sort the combined result by publish date
	sort.Slice(result, func(i, j int) bool {
		return result[i].Published.After(result[j].Published)
	})
	return result
}

// ByKeyword filters articles by keywords in their title or description.
// If allowlist is non-empty, only articles matching at least one allowed keyword are kept.
// Articles matching any blocked keyword are always removed (even if they matched allowlist).
// Matching is case-insensitive.
func ByKeyword(articles []Article, allowlist, blocklist []string) []Article {
	var filtered []Article
	for _, a := range articles {
		// if allowlist is set, the article must match at least one keyword
		if len(allowlist) > 0 && !matchesAny(a, allowlist) {
			continue
		}
		// remove if it matches any blocked keyword
		if matchesAny(a, blocklist) {
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered
}

// matchesAny returns true if the article's title or description contains
// any of the given keywords (case-insensitive substring match).
func matchesAny(a Article, keywords []string) bool {
	title := strings.ToLower(a.Title)
	desc := strings.ToLower(a.Description)
	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		if strings.Contains(title, lower) || strings.Contains(desc, lower) {
			return true
		}
	}
	return false
}

// Digest is a convenience method that runs the full pipeline:
// remove seen → filter by keywords → pick top N per source.
// It marks kept articles as seen in the store.
func Digest(articles []Article, seen Hasher, allowlist, blocklist []string, maxPerSource int) []Article {
	// step 1: remove already-seen articles
	fresh := RemoveSeen(articles, seen)

	// step 2: apply keyword filters
	filtered := ByKeyword(fresh, allowlist, blocklist)

	// step 3: pick top N per source
	result := TopNPerSource(filtered, maxPerSource)

	// step 4: mark kept articles as seen
	for _, a := range result {
		seen.Mark(a.Link)
	}

	return result
}
