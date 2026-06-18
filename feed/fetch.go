package feed

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"firstbyte/config"
	"firstbyte/filter"
)

// httpClient is reused across all requests with a sensible timeout.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// RSS XML structures

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// Atom XML structures

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Summary   string     `xml:"summary"`
	ID        string     `xml:"id"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// FetchAll fetches every source concurrently and returns all parsed articles.
// Per-source errors are collected but do not abort the overall run.
func FetchAll(sources []config.Source) ([]filter.Article, []error) {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		articles []filter.Article
		errs     []error
	)

	for _, src := range sources {
		wg.Add(1)
		go func(s config.Source) {
			defer wg.Done()

			items, err := FetchOne(s)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
				return
			}
			articles = append(articles, items...)
		}(src)
	}

	wg.Wait()
	return articles, errs
}

// FetchOne downloads and parses a single RSS or Atom feed.
// It auto-detects the format and converts the items to filter.Article.
func FetchOne(src config.Source) ([]filter.Article, error) {
	resp, err := httpClient.Get(src.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// try RSS first, then Atom
	if items := parseRSS(body, src.Name); len(items) > 0 {
		return items, nil
	}
	if items := parseAtom(body, src.Name); len(items) > 0 {
		return items, nil
	}

	return nil, fmt.Errorf("unrecognized feed format")
}

// RSS parsing

func parseRSS(body []byte, source string) []filter.Article {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}
	if feed.Channel.Items == nil {
		return nil
	}

	var articles []filter.Article
	for _, item := range feed.Channel.Items {
		link := item.Link
		if link == "" {
			link = item.GUID // fallback for feeds that put the permalink in GUID
		}
		if link == "" {
			continue
		}
		articles = append(articles, filter.Article{
			Title:       html.UnescapeString(item.Title),
			Link:        link,
			Published:   parseDate(item.PubDate),
			Description: cleanDesc(item.Description),
			Source:      source,
		})
	}
	return articles
}

// Atom parsing

func parseAtom(body []byte, source string) []filter.Article {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}
	if feed.Entries == nil {
		return nil
	}

	var articles []filter.Article
	for _, entry := range feed.Entries {
		link := bestAtomLink(entry.Links)
		if link == "" {
			link = entry.ID // fallback
		}
		if link == "" {
			continue
		}

		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}

		articles = append(articles, filter.Article{
			Title:       html.UnescapeString(entry.Title),
			Link:        link,
			Published:   parseDate(dateStr),
			Description: cleanDesc(entry.Summary),
			Source:      source,
		})
	}
	return articles
}

// bestAtomLink returns the best link from an Atom entry,
// preferring alternate or the first available.
func bestAtomLink(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "" || l.Rel == "alternate" {
			return l.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

// Date parsing

// known date formats we try in order (most common first).
var dateFormats = []string{
	time.RFC1123Z,    // Mon, 02 Jan 2006 15:04:05 -0700
	time.RFC1123,     // Mon, 02 Jan 2006 15:04:05 MST
	time.RFC3339,     // 2006-01-02T15:04:05Z07:00
	time.RFC3339Nano, // 2006-01-02T15:04:05.999999999Z07:00
	time.RFC822Z,     // 02 Jan 06 15:04 -0700
	time.RFC822,      // 02 Jan 06 15:04 MST
	"2006-01-02 15:04:05",
}

// parseDate tries to parse a date string using common feed formats.
// Falls back to time.Now() on failure so articles without dates
// still appear in the digest (sorted to the top as "fresh").
func parseDate(raw string) time.Time {
	raw = stripCDATA(raw)
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Now()
}

// stripCDATA removes CDATA wrappers if present.
func stripCDATA(s string) string {
	if len(s) > 9 && s[:9] == "<![CDATA[" {
		s = s[9:]
		if len(s) > 3 && s[len(s)-3:] == "]]>" {
			s = s[:len(s)-3]
		}
	}
	return s
}

// cleanDesc strips HTML tags and entities from a description.
// Returns an empty string if the remaining text is just boilerplate (< 20 chars).
func cleanDesc(raw string) string {
	if raw == "" {
		return ""
	}

	// decode HTML entities first
	text := html.UnescapeString(raw)

	// strip HTML tags
	var buf strings.Builder
	inTag := false
	for _, r := range text {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			buf.WriteRune(r)
		}
	}
	text = strings.TrimSpace(buf.String())

	// drop descriptions that are just boilerplate (Comments, Read more, etc.)
	if len([]rune(text)) < 20 {
		return ""
	}

	return text
}
