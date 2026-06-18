package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// create a temporary config.yaml for testing
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
sources:
  - name: Hacker News
    url: https://news.ycombinator.com/rss
    max_articles: 5
  - name: Electrek
    url: https://electrek.co/feed

notifications:
  - email

email:
  smtp_host: smtp.gmail.com
  smtp_port: 587
  from: digest@example.com
  to: you@example.com

store:
  path: ./seen.json
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// verify sources
	if len(cfg.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Name != "Hacker News" {
		t.Errorf("expected first source 'Hacker News', got %q", cfg.Sources[0].Name)
	}
	if cfg.Sources[0].MaxArticles != 5 {
		t.Errorf("expected MaxArticles 5, got %d", cfg.Sources[0].MaxArticles)
	}
	// verify default for MaxArticles
	if cfg.Sources[1].MaxArticles != 5 {
		t.Errorf("expected default MaxArticles 5, got %d", cfg.Sources[1].MaxArticles)
	}

	// verify notifications
	if len(cfg.Notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(cfg.Notifications))
	}

	// verify email
	if cfg.Email.SMTPHost != "smtp.gmail.com" {
		t.Errorf("expected smtp_host smtp.gmail.com, got %q", cfg.Email.SMTPHost)
	}

	// verify store
	if cfg.Store.Path != "./seen.json" {
		t.Errorf("expected store path ./seen.json, got %q", cfg.Store.Path)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// minimal config — no store path, no max_articles
	minimal := `
sources:
  - name: Test Feed
    url: https://example.com/rss

notifications:
  - email

email:
  smtp_host: localhost
  smtp_port: 25
  from: test@test.com
  to: test@test.com
`
	if err := os.WriteFile(configPath, []byte(minimal), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.Store.Path != "./seen.json" {
		t.Errorf("expected default store path './seen.json', got %q", cfg.Store.Path)
	}
	if cfg.Sources[0].MaxArticles != 5 {
		t.Errorf("expected default MaxArticles 5, got %d", cfg.Sources[0].MaxArticles)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Sources: []Source{
					{Name: "HN", URL: "https://example.com/rss"},
				},
				Notifications: []string{"email"},
			},
			wantErr: false,
		},
		{
			name: "no sources",
			cfg: &Config{
				Notifications: []string{"email"},
			},
			wantErr: true,
		},
		{
			name: "source missing name",
			cfg: &Config{
				Sources:       []Source{{URL: "https://example.com/rss"}},
				Notifications: []string{"email"},
			},
			wantErr: true,
		},
		{
			name: "source missing url",
			cfg: &Config{
				Sources:       []Source{{Name: "Test"}},
				Notifications: []string{"email"},
			},
			wantErr: true,
		},
		{
			name: "no notifications",
			cfg: &Config{
				Sources: []Source{
					{Name: "HN", URL: "https://example.com/rss"},
				},
			},
			wantErr: true,
		},
		{
			name: "unknown notification channel",
			cfg: &Config{
				Sources: []Source{
					{Name: "HN", URL: "https://example.com/rss"},
				},
				Notifications: []string{"slack"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSecrets(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := `
# secrets for feedforward
SMTP_USER=testuser@gmail.com
SMTP_PASSWORD=app-password-here
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	secrets, err := LoadSecrets(envPath)
	if err != nil {
		t.Fatalf("LoadSecrets() error: %v", err)
	}

	if secrets.SMTPUser != "testuser@gmail.com" {
		t.Errorf("expected SMTP_USER 'testuser@gmail.com', got %q", secrets.SMTPUser)
	}
	if secrets.SMTPPassword != "app-password-here" {
		t.Errorf("expected SMTP_PASSWORD set, got %q", secrets.SMTPPassword)
	}
}

func TestSecretsValidate(t *testing.T) {
	tests := []struct {
		name          string
		secrets       *Secrets
		notifications []string
		wantErr       bool
	}{
		{
			name: "email secrets present",
			secrets: &Secrets{
				SMTPUser:     "user",
				SMTPPassword: "pass",
			},
			notifications: []string{"email"},
			wantErr:       false,
		},
		{
			name: "missing email user",
			secrets: &Secrets{
				SMTPPassword: "pass",
			},
			notifications: []string{"email"},
			wantErr:       true,
		},
		{
			name:    "no notifications — nothing to validate",
			secrets: &Secrets{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.secrets.Validate(tt.notifications)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
