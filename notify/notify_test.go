package notify

import (
	"strings"
	"testing"

	"firstbyte/filter"
)

func TestGroupArticles(t *testing.T) {
	// articles arrive newest-first across sources, not in config order
	articles := []filter.Article{
		{Title: "G1", Source: "GitHub"},
		{Title: "H1", Source: "Hacker News"},
		{Title: "X1", Source: "Unlisted"},
		{Title: "H2", Source: "Hacker News"},
		{Title: "L1", Source: "Lobsters"},
	}
	order := []string{"Hacker News", "Ars Technica", "Lobsters", "GitHub"}

	tests := []struct {
		name  string
		order []string
		want  []string // expected source order
	}{
		{"config order, empty sources skipped, unlisted last", order, []string{"Hacker News", "Lobsters", "GitHub", "Unlisted"}},
		{"no order falls back to first appearance", nil, []string{"GitHub", "Hacker News", "Unlisted", "Lobsters"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := GroupArticles(articles, tt.order)

			var got []string
			for _, g := range groups {
				got = append(got, g.Source)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("source order = %v, want %v", got, tt.want)
			}
		})
	}

	// articles keep their original order within a group
	groups := GroupArticles(articles, order)
	if hn := groups[0].Items; len(hn) != 2 || hn[0].Title != "H1" || hn[1].Title != "H2" {
		t.Errorf("Hacker News items = %+v, want H1 then H2", hn)
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
