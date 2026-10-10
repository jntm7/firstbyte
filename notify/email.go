package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"
	"strings"

	"firstbyte/config"
	"firstbyte/filter"
)

// DigestData is the data passed to email templates.
type DigestData struct {
	Date     string
	Articles []ArticleGroup
}

// ArticleGroup represents a set of articles from one source.
type ArticleGroup struct {
	Source string
	Items  []filter.Article
}

// GroupArticles groups articles by Source, ordered by sourceOrder (the config order).
// Sources with no articles are left out; any source not in sourceOrder goes last,
// in order of first appearance. Articles keep their order within each group.
func GroupArticles(articles []filter.Article, sourceOrder []string) []ArticleGroup {
	// bucket articles by source, remembering first-appearance order
	groups := make(map[string][]filter.Article)
	var appearance []string
	for _, a := range articles {
		if _, ok := groups[a.Source]; !ok {
			appearance = append(appearance, a.Source)
		}
		groups[a.Source] = append(groups[a.Source], a)
	}

	// emit configured sources first, then anything unlisted
	var result []ArticleGroup
	added := make(map[string]bool)
	for _, name := range append(append([]string{}, sourceOrder...), appearance...) {
		if items, ok := groups[name]; ok && !added[name] {
			added[name] = true
			result = append(result, ArticleGroup{Source: name, Items: items})
		}
	}
	return result
}

// RenderEmail renders the HTML email template and returns the rendered body.
func RenderEmail(data DigestData, templateDir string) ([]byte, error) {
	tmplPath := filepath.Join(templateDir, "email.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("parse email template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return nil, fmt.Errorf("render email: %w", err)
	}

	return body.Bytes(), nil
}

// SendEmail renders the HTML email template and sends it via SMTP.
func SendEmail(cfg config.EmailConfig, secrets config.Secrets, data DigestData, templateDir string) error {
	body, err := RenderEmail(data, templateDir)
	if err != nil {
		return err
	}

	msg := buildEmailMessage(cfg.From, cfg.To, "Daily Tech Digest", body)

	auth := smtp.PlainAuth("", secrets.SMTPUser, secrets.SMTPPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	if err := smtp.SendMail(addr, auth, cfg.From, []string(cfg.To), msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

// SendTestEmail sends a simple test message to verify SMTP credentials.
func SendTestEmail(cfg config.EmailConfig, secrets config.Secrets) error {
	body := []byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;padding:24px;">
<h1 style="color:#1a1a2e;">FirstByte Test</h1>
<p style="color:#495057;font-size:15px;">If you're reading this, SMTP is configured correctly.</p>
</body></html>`)

	msg := buildEmailMessage(cfg.From, cfg.To, "FirstByte SMTP Test", body)

	auth := smtp.PlainAuth("", secrets.SMTPUser, secrets.SMTPPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	if err := smtp.SendMail(addr, auth, cfg.From, []string(cfg.To), msg); err != nil {
		return fmt.Errorf("test email: %w", err)
	}

	return nil
}

// buildEmailMessage constructs a raw MIME email with HTML body.
func buildEmailMessage(from string, to []string, subject string, htmlBody []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: FirstByte — %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.Write(htmlBody)

	return buf.Bytes()
}
