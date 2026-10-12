package notify

import (
	"bytes"
	"errors"
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

// SendEmail renders the HTML email template and sends it via SMTP,
// one copy per recipient so nobody sees the other addresses.
func SendEmail(cfg config.EmailConfig, secrets config.Secrets, data DigestData, templateDir string) error {
	body, err := RenderEmail(data, templateDir)
	if err != nil {
		return err
	}

	if err := sendEach(smtp.SendMail, cfg, secrets, "Daily Tech Digest", body); err != nil {
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

	if err := sendEach(smtp.SendMail, cfg, secrets, "FirstByte SMTP Test", body); err != nil {
		return fmt.Errorf("test email: %w", err)
	}
	return nil
}

// sendFunc matches smtp.SendMail so tests can swap in a fake sender.
type sendFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// sendEach sends a separate copy of the message to each recipient.
// A failure for one recipient doesn't stop the others; all failures are returned together.
func sendEach(send sendFunc, cfg config.EmailConfig, secrets config.Secrets, subject string, body []byte) error {
	auth := smtp.PlainAuth("", secrets.SMTPUser, secrets.SMTPPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var errs []error
	for _, rcpt := range cfg.To {
		// only this recipient appears in the To header and the SMTP envelope
		msg := buildEmailMessage(cfg.From, rcpt, subject, body)
		if err := send(addr, auth, cfg.From, []string{rcpt}, msg); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", rcpt, err))
		}
	}
	return errors.Join(errs...)
}

// buildEmailMessage constructs a raw MIME email with HTML body for a single recipient.
func buildEmailMessage(from, to string, subject string, htmlBody []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: FirstByte — %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.Write(htmlBody)

	return buf.Bytes()
}
