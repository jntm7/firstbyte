package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"

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

// GroupArticles groups a flat list of articles by their Source field.
func GroupArticles(articles []filter.Article) []ArticleGroup {
	groups := make(map[string][]filter.Article)
	for _, a := range articles {
		groups[a.Source] = append(groups[a.Source], a)
	}

	// preserve source order from first appearance
	seen := make(map[string]bool)
	var result []ArticleGroup
	for _, a := range articles {
		if seen[a.Source] {
			continue
		}
		seen[a.Source] = true
		result = append(result, ArticleGroup{
			Source: a.Source,
			Items:  groups[a.Source],
		})
	}
	return result
}

// SendEmail renders the HTML email template and sends it via SMTP.
func SendEmail(cfg config.EmailConfig, secrets config.Secrets, data DigestData, templateDir string) error {
	// parse the email template
	tmplPath := filepath.Join(templateDir, "email.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("parse email template: %w", err)
	}

	// render template into a buffer
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("render email: %w", err)
	}

	// build the email message
	msg := buildEmailMessage(cfg.From, cfg.To, data.Date, body.Bytes())

	// connect and send
	auth := smtp.PlainAuth("", secrets.SMTPUser, secrets.SMTPPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	if err := smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

// buildEmailMessage constructs a raw MIME email with HTML body.
func buildEmailMessage(from, to, date string, htmlBody []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: FirstByte Digest — %s\r\n", date))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.Write(htmlBody)

	return buf.Bytes()
}
