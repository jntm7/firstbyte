package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source represents a single RSS feed to fetch.
type Source struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	MaxArticles int    `yaml:"max_articles"`
}

// EmailConfig holds SMTP settings for email delivery.
type EmailConfig struct {
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
}

// StoreConfig holds settings for the seen-article store.
type StoreConfig struct {
	Path string `yaml:"path"`
}

// Config is the top-level configuration loaded from config.yaml.
type Config struct {
	Sources       []Source    `yaml:"sources"`
	Schedule      string      `yaml:"schedule"`
	Notifications []string    `yaml:"notifications"`
	Email         EmailConfig `yaml:"email"`
	Store         StoreConfig `yaml:"store"`
}

// Secrets holds credentials loaded from environment variables.
type Secrets struct {
	SMTPUser          string
	SMTPPassword      string
	DiscordWebhookURL string
}

// LoadConfig reads and parses the YAML config file at the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// set defaults
	for i := range cfg.Sources {
		if cfg.Sources[i].MaxArticles <= 0 {
			cfg.Sources[i].MaxArticles = 5
		}
	}
	if cfg.Store.Path == "" {
		cfg.Store.Path = "./seen.json"
	}

	return &cfg, nil
}

// LoadSecrets reads secrets from environment variables.
// If envPath is provided, it loads a .env file first (non-fatal if missing).
func LoadSecrets(envPath string) (*Secrets, error) {
	// load .env file into environment if path provided
	if envPath != "" {
		if err := loadEnvFile(envPath); err != nil {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	return &Secrets{
		SMTPUser:          os.Getenv("SMTP_USER"),
		SMTPPassword:      os.Getenv("SMTP_PASSWORD"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
	}, nil
}

// loadEnvFile reads a .env file and sets each KEY=VALUE pair in the environment.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// strip surrounding quotes if present
		value = strings.Trim(value, `"'`)
		os.Setenv(key, value)
	}
	return scanner.Err()
}

// Validate checks that all required config fields are present.
func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}
	for i, s := range c.Sources {
		if s.Name == "" {
			return fmt.Errorf("source %d: name is required", i)
		}
		if s.URL == "" {
			return fmt.Errorf("source %d (%s): url is required", i, s.Name)
		}
	}

	if len(c.Notifications) == 0 {
		return fmt.Errorf("at least one notification channel is required")
	}
	for _, n := range c.Notifications {
		if n != "email" && n != "discord" {
			return fmt.Errorf("unknown notification channel: %s (use email or discord)", n)
		}
	}

	return nil
}

// Validate checks that secrets required by the given notification channels are present.
func (s *Secrets) Validate(notifications []string) error {
	for _, n := range notifications {
		switch n {
		case "email":
			if s.SMTPUser == "" {
				return fmt.Errorf("SMTP_USER is required for email notifications")
			}
			if s.SMTPPassword == "" {
				return fmt.Errorf("SMTP_PASSWORD is required for email notifications")
			}
		case "discord":
			if s.DiscordWebhookURL == "" {
				return fmt.Errorf("DISCORD_WEBHOOK_URL is required for discord notifications")
			}
		}
	}
	return nil
}
