package notify

import (
	"strings"
	"testing"

	"firstbyte/filter"
)

func TestGroupArticles(t *testing.T) {
	articles := []filter.Article{
		{Title: "A1", Source: "Hacker News", Link: "https://h.com/1"},
		{Title: "B1", Source: "Lobsters", Link: "https://l.com/1"},
		{Title: "A2", Source: "Hacker News", Link: "https://h.com/2"},
		{Title: "C1", Source: "GitHub", Link: "https://g.com/1"},
	}

	groups := GroupArticles(articles)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// verify Hacker News is first (first appearance)
	if groups[0].Source != "Hacker News" {
		t.Errorf("expected Hacker News first, got %q", groups[0].Source)
	}
	if len(groups[0].Items) != 2 {
		t.Errorf("expected 2 HN articles, got %d", len(groups[0].Items))
	}

	// Lobsters second
	if groups[1].Source != "Lobsters" {
		t.Errorf("expected Lobsters second, got %q", groups[1].Source)
	}
	if len(groups[1].Items) != 1 {
		t.Errorf("expected 1 Lobsters article, got %d", len(groups[1].Items))
	}

	// GitHub third
	if groups[2].Source != "GitHub" {
		t.Errorf("expected GitHub third, got %q", groups[2].Source)
	}
}

func TestBuildEmailMessage(t *testing.T) {
	html := []byte("<h1>Hello</h1>")
	msg := buildEmailMessage("from@test.com", []string{"to@test.com"}, "June 17, 2026", html)

	s := string(msg)

	if !strings.Contains(s, "From: from@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(s, "To: to@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(s, "Subject: FirstByte — June 17, 2026") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(s, "Content-Type: text/html") {
		t.Error("missing content type")
	}
	if !strings.Contains(s, "<h1>Hello</h1>") {
		t.Error("missing HTML body")
	}
}

func TestBuildEmailMessageMultipleRecipients(t *testing.T) {
	html := []byte("<p>test</p>")
	msg := buildEmailMessage("from@test.com", []string{"a@test.com", "b@test.com"}, "Subject", html)

	s := string(msg)

	if !strings.Contains(s, "To: a@test.com, b@test.com") {
		t.Errorf("expected both recipients in To header, got %q", extractHeader(s, "To:"))
	}
}

func extractHeader(msg, header string) string {
	lines := strings.Split(msg, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, header) {
			return line
		}
	}
	return ""
}
