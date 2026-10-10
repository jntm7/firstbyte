package notify

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"

	"firstbyte/config"
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
	msg := buildEmailMessage("from@test.com", "to@test.com", "June 17, 2026", html)

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

func TestSendEachRecipientGetsOwnCopy(t *testing.T) {
	cfg := config.EmailConfig{
		SMTPHost: "smtp.test", SMTPPort: 587, From: "from@test.com",
		To: config.StringSlice{"a@test.com", "b@test.com", "c@test.com"},
	}

	// fake sender records each call instead of talking to a server
	type call struct {
		to  []string
		msg string
	}
	var calls []call
	fake := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		calls = append(calls, call{to, string(msg)})
		return nil
	}

	if err := sendEach(fake, cfg, config.Secrets{}, "Subject", []byte("<p>hi</p>")); err != nil {
		t.Fatalf("sendEach() error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 sends, got %d", len(calls))
	}
	for i, want := range cfg.To {
		c := calls[i]
		// envelope and To header must contain only this recipient
		if len(c.to) != 1 || c.to[0] != want {
			t.Errorf("send %d: envelope recipients = %v, want [%s]", i, c.to, want)
		}
		if got := extractHeader(c.msg, "To:"); got != "To: "+want {
			t.Errorf("send %d: header %q, want %q", i, got, "To: "+want)
		}
		for _, other := range cfg.To {
			if other != want && strings.Contains(c.msg, other) {
				t.Errorf("send %d: message for %s leaks %s", i, want, other)
			}
		}
	}
}

func TestSendEachContinuesAfterFailure(t *testing.T) {
	cfg := config.EmailConfig{
		SMTPHost: "smtp.test", SMTPPort: 587, From: "from@test.com",
		To: config.StringSlice{"bad@test.com", "good@test.com"},
	}

	// first recipient fails, second succeeds
	var sentTo []string
	fake := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		sentTo = append(sentTo, to[0])
		if to[0] == "bad@test.com" {
			return errors.New("mailbox unavailable")
		}
		return nil
	}

	err := sendEach(fake, cfg, config.Secrets{}, "Subject", []byte("<p>hi</p>"))
	if err == nil || !strings.Contains(err.Error(), "bad@test.com") {
		t.Fatalf("expected error naming bad@test.com, got %v", err)
	}
	if len(sentTo) != 2 || sentTo[1] != "good@test.com" {
		t.Errorf("expected delivery to continue to good@test.com, sent to %v", sentTo)
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
