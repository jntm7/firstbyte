package notify

import (
	"errors"
	"mime"
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
	msg := buildEmailMessage("FirstByte", "from@test.com", "to@test.com", "Tech News — Tue, Oct 6, 2026", html)

	s := string(msg)

	if !strings.Contains(s, "From: \"FirstByte\" <from@test.com>") {
		t.Errorf("missing From header with display name, got %q", extractHeader(s, "From:"))
	}
	if !strings.Contains(s, "To: to@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(s, "Content-Type: text/html") {
		t.Error("missing content type")
	}
	if !strings.Contains(s, "<h1>Hello</h1>") {
		t.Error("missing HTML body")
	}

	// subject is RFC 2047 encoded because of the em dash; decode to check the text
	raw := strings.TrimPrefix(extractHeader(s, "Subject:"), "Subject: ")
	decoded, err := new(mime.WordDecoder).DecodeHeader(raw)
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if decoded != "Tech News — Tue, Oct 6, 2026" {
		t.Errorf("subject = %q, want %q", decoded, "Tech News — Tue, Oct 6, 2026")
	}
}

func TestBuildEmailMessageWithoutFromName(t *testing.T) {
	msg := string(buildEmailMessage("", "from@test.com", "to@test.com", "Plain subject", nil))

	if !strings.Contains(msg, "From: <from@test.com>") {
		t.Errorf("unexpected From header %q", extractHeader(msg, "From:"))
	}
	if !strings.Contains(msg, "Subject: Plain subject") {
		t.Errorf("ASCII subject should be left as-is, got %q", extractHeader(msg, "Subject:"))
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

	sent, err := sendEach(fake, cfg, config.Secrets{}, "Subject", []byte("<p>hi</p>"))
	if err != nil {
		t.Fatalf("sendEach() error: %v", err)
	}
	if sent != 3 {
		t.Errorf("sent = %d, want 3", sent)
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

	sent, err := sendEach(fake, cfg, config.Secrets{}, "Subject", []byte("<p>hi</p>"))
	if err == nil || !strings.Contains(err.Error(), "bad@test.com") {
		t.Fatalf("expected error naming bad@test.com, got %v", err)
	}
	// the caller uses this count to decide whether the digest was delivered at all
	if sent != 1 {
		t.Errorf("sent = %d, want 1 (only good@test.com)", sent)
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

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{name: "short text untouched", in: "Short summary.", limit: 50, want: "Short summary."},
		{name: "exactly at limit", in: "abcde", limit: 5, want: "abcde"},
		{name: "breaks on word boundary", in: "The quick brown fox jumps", limit: 12, want: "The quick…"},
		{name: "trims trailing punctuation", in: "Hello, world and more", limit: 7, want: "Hello…"},
		{name: "no spaces hard cut", in: "abcdefghij", limit: 4, want: "abcd…"},
		{name: "counts runes not bytes", in: "héllo wörld ünïcode", limit: 11, want: "héllo wörld…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateText(tt.in, tt.limit); got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}

func TestTruncateDescriptionsDoesNotMutateInput(t *testing.T) {
	// build one group with a long description
	long := strings.Repeat("word ", 100)
	groups := []ArticleGroup{{Source: "A", Items: []filter.Article{{Title: "T", Description: long}}}}

	got := TruncateDescriptions(groups, 20)
	if n := len([]rune(got[0].Items[0].Description)); n > 21 {
		t.Errorf("truncated description has %d runes, want <= 21", n)
	}
	if groups[0].Items[0].Description != long {
		t.Error("input description was modified")
	}

	// a non-positive limit leaves descriptions alone
	if got := TruncateDescriptions(groups, -1); got[0].Items[0].Description != long {
		t.Error("limit -1 should not truncate")
	}
}

func TestDropDescriptions(t *testing.T) {
	groups := []ArticleGroup{{Source: "A", Items: []filter.Article{{Title: "T", Description: "A real summary here."}}}}

	got := DropDescriptions(groups)
	if got[0].Items[0].Description != "" {
		t.Errorf("description = %q, want empty", got[0].Items[0].Description)
	}
	if got[0].Items[0].Title != "T" {
		t.Errorf("title = %q, want %q", got[0].Items[0].Title, "T")
	}
	if groups[0].Items[0].Description == "" {
		t.Error("input description was modified")
	}
}
