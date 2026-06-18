package feed

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"firstbyte/config"
)

func TestFetchOneRSS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <item>
      <title>First Article</title>
      <link>https://example.com/1</link>
      <description>Description of first</description>
      <pubDate>Mon, 15 Jun 2026 10:00:00 -0700</pubDate>
      <guid>https://example.com/1</guid>
    </item>
    <item>
      <title>Second Article</title>
      <link>https://example.com/2</link>
      <description>Description of second</description>
      <pubDate>Tue, 16 Jun 2026 10:00:00 -0700</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer ts.Close()

	articles, err := FetchOne(config.Source{Name: "Test", URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	a := articles[0]
	if a.Title != "First Article" {
		t.Errorf("expected 'First Article', got %q", a.Title)
	}
	if a.Link != "https://example.com/1" {
		t.Errorf("expected link, got %q", a.Link)
	}
	if a.Source != "Test" {
		t.Errorf("expected source 'Test', got %q", a.Source)
	}
	if a.Description != "Description of first" {
		t.Errorf("expected description, got %q", a.Description)
	}
	year, month, day := a.Published.Date()
	if year != 2026 || month != 6 || day != 15 {
		t.Errorf("expected date 2026-06-15, got %d-%02d-%02d", year, month, day)
	}
}

func TestFetchOneAtom(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <entry>
    <title>Atom Article</title>
    <link rel="alternate" href="https://example.com/atom/1"/>
    <published>2026-06-16T08:00:00Z</published>
    <summary>A detailed article about the latest Atom feed developments</summary>
    <id>tag:example.com,2026:1</id>
  </entry>
</feed>`))
	}))
	defer ts.Close()

	articles, err := FetchOne(config.Source{Name: "AtomTest", URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}

	a := articles[0]
	if a.Title != "Atom Article" {
		t.Errorf("expected 'Atom Article', got %q", a.Title)
	}
	if a.Link != "https://example.com/atom/1" {
		t.Errorf("expected atom link, got %q", a.Link)
	}
	if a.Description != "A detailed article about the latest Atom feed developments" {
		t.Errorf("expected summary, got %q", a.Description)
	}
	if a.Source != "AtomTest" {
		t.Errorf("expected source 'AtomTest', got %q", a.Source)
	}
}

func TestFetchOneAtomLinkFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>No Link Rel</title>
    <link href="https://example.com/no-rel"/>
    <id>tag:example.com:2</id>
  </entry>
</feed>`))
	}))
	defer ts.Close()

	articles, err := FetchOne(config.Source{Name: "Src", URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) == 0 {
		t.Fatal("expected at least 1 article")
	}
	if articles[0].Link != "https://example.com/no-rel" {
		t.Errorf("expected link without rel, got %q", articles[0].Link)
	}
}

func TestFetchOneAtomUsesIDFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>ID Only</title>
    <id>https://example.com/id-only</id>
  </entry>
</feed>`))
	}))
	defer ts.Close()

	articles, _ := FetchOne(config.Source{Name: "Src", URL: ts.URL})
	if len(articles) == 0 {
		t.Fatal("expected at least 1 article")
	}
	if articles[0].Link != "https://example.com/id-only" {
		t.Errorf("expected ID as fallback link, got %q", articles[0].Link)
	}
}

func TestFetchOneHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := FetchOne(config.Source{Name: "Bad", URL: ts.URL})
	if err == nil {
		t.Error("expected error for 500 status, got nil")
	}
}

func TestFetchOneBadURL(t *testing.T) {
	_, err := FetchOne(config.Source{Name: "Bad", URL: "http://127.0.0.1:1/nothing"})
	if err == nil {
		t.Error("expected error for bad URL, got nil")
	}
}

func TestFetchAllMixedSuccess(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
  <item><title>Good</title><link>https://example.com/good</link></item>
</channel></rss>`))
	}))
	defer goodServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer badServer.Close()

	sources := []config.Source{
		{Name: "GoodFeed", URL: goodServer.URL},
		{Name: "BadFeed", URL: badServer.URL},
	}

	articles, errs := FetchAll(sources)

	// should have 1 article from the good feed
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Source != "GoodFeed" {
		t.Errorf("expected source 'GoodFeed', got %q", articles[0].Source)
	}

	// should have 1 error from the bad feed
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestFetchAllConcurrent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
  <item><title>Article</title><link>https://example.com/item</link></item>
</channel></rss>`))
	}))
	defer ts.Close()

	// 10 identical sources to stress concurrency
	sources := make([]config.Source, 10)
	for i := range sources {
		sources[i] = config.Source{
			Name: "TestFeed",
			URL:  ts.URL,
		}
	}

	articles, errs := FetchAll(sources)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(articles) != 10 {
		t.Errorf("expected 10 articles from 10 sources, got %d", len(articles))
	}
}

func TestFetchAllEmptySources(t *testing.T) {
	articles, errs := FetchAll(nil)
	if len(articles) != 0 || len(errs) != 0 {
		t.Errorf("expected no results for empty sources")
	}
}

func TestCleanDesc(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"<p>Real content about something interesting</p>", "Real content about something interesting"},
		{"<a href='https://example.com'>Comments</a>", ""},
		{"<img src='x.jpg'/>", ""},
		{"Short", ""},
	}

	for _, tt := range tests {
		got := cleanDesc(tt.in)
		if got != tt.want {
			t.Errorf("cleanDesc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
