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
