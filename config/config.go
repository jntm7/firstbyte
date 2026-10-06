package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// StringSlice accepts a single string or a list of strings in YAML.
type StringSlice []string

// UnmarshalYAML supports both "to: user@example.com" and "to: [a, b]" formats.
func (s *StringSlice) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		*s = StringSlice{single}
		return nil
	}
	return value.Decode((*[]string)(s))
}

// Source represents a single RSS feed to fetch.
type Source struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	MaxArticles int    `yaml:"max_articles"`
	Enabled     *bool  `yaml:"enabled,omitempty"` // nil means enabled
}

// IsEnabled reports whether the source should be fetched (default true).
func (s Source) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// EnabledSources returns only the sources that are switched on.
func (c *Config) EnabledSources() []Source {
	var result []Source
	for _, s := range c.Sources {
		if s.IsEnabled() {
			result = append(result, s)
		}
	}
	return result
}

// EmailConfig holds SMTP settings for email delivery.
type EmailConfig struct {
	SMTPHost string      `yaml:"smtp_host"`
	SMTPPort int         `yaml:"smtp_port"`
	From     string      `yaml:"from"`
	To       StringSlice `yaml:"to"`
}

// StoreConfig holds settings for the seen-article store.
type StoreConfig struct {
	Path string `yaml:"path"`
}

// defaultMaxAge is the article age window used when no max_age is configured.
const defaultMaxAge = 24 * time.Hour

// FilterConfig holds global keyword and age-based filter settings.
type FilterConfig struct {
	AllowList      []string      `yaml:"allowlist"`
	BlockList      []string      `yaml:"blocklist"`
	MaxAge         string        `yaml:"max_age"`      // Go duration, e.g. "24h"; "0" disables
	MaxAgeDays     int           `yaml:"max_age_days"` // deprecated: use max_age
	MaxAgeDuration time.Duration `yaml:"-"`            // resolved from MaxAge / MaxAgeDays
}

// Config is the top-level configuration loaded from config.yaml.
type Config struct {
	Sources       []Source     `yaml:"sources"`
	Filter        FilterConfig `yaml:"filter"`
	Notifications []string     `yaml:"notifications"`
	Email         EmailConfig  `yaml:"email"`
	Store         StoreConfig  `yaml:"store"`
}

// Secrets holds credentials loaded from environment variables.
type Secrets struct {
	SMTPUser     string
	SMTPPassword string
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

	// apply environment variable overrides
	overrideFromEnv(&cfg)

	// resolve the article age window
	maxAge, err := resolveMaxAge(cfg.Filter)
	if err != nil {
		return nil, err
	}
	cfg.Filter.MaxAgeDuration = maxAge

	return &cfg, nil
}

// resolveMaxAge picks the age window: max_age wins, then the deprecated
// max_age_days, then the 24h default. Zero or negative means no limit.
func resolveMaxAge(f FilterConfig) (time.Duration, error) {
	// prefer the duration string when set
	if f.MaxAge != "" {
		d, err := time.ParseDuration(f.MaxAge)
		if err != nil {
			return 0, fmt.Errorf("parse filter.max_age %q: %w", f.MaxAge, err)
		}
		return d, nil
	}

	// fall back to legacy whole-day setting
	if f.MaxAgeDays > 0 {
		return time.Duration(f.MaxAgeDays) * 24 * time.Hour, nil
	}

	return defaultMaxAge, nil
}

// LoadSecrets reads secrets from environment variables.
// If envPath is provided, it loads a .env file first (skips silently if file doesn't exist).
func LoadSecrets(envPath string) (*Secrets, error) {
	if envPath != "" {
		if err := loadEnvFile(envPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load .env file: %w", err)
		}
	}

	return &Secrets{
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
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

// overrideFromEnv applies environment variable overrides to the config.
// Env vars take precedence over values in the YAML file.
// Supported vars: MAX_AGE, MAX_AGE_DAYS (deprecated), SMTP_HOST, SMTP_PORT, EMAIL_FROM, EMAIL_TO,
// STORE_PATH, FILTER_ALLOWLIST, FILTER_BLOCKLIST.
func overrideFromEnv(cfg *Config) {
	// legacy day count; clears max_age so the env value takes precedence
	if v := os.Getenv("MAX_AGE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Filter.MaxAgeDays = n
			cfg.Filter.MaxAge = ""
		}
	}
	if v := os.Getenv("MAX_AGE"); v != "" {
		cfg.Filter.MaxAge = v
	}
	if v := os.Getenv("SMTP_HOST"); v != "" {
		cfg.Email.SMTPHost = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Email.SMTPPort = n
		}
	}
	if v := os.Getenv("EMAIL_FROM"); v != "" {
		cfg.Email.From = v
	}
	if v := os.Getenv("EMAIL_TO"); v != "" {
		cfg.Email.To = splitEnvList(v)
	}
	if v := os.Getenv("STORE_PATH"); v != "" {
		cfg.Store.Path = v
	}
	if v := os.Getenv("FILTER_ALLOWLIST"); v != "" {
		cfg.Filter.AllowList = splitEnvList(v)
	}
	if v := os.Getenv("FILTER_BLOCKLIST"); v != "" {
		cfg.Filter.BlockList = splitEnvList(v)
	}
}

// splitEnvList splits a comma-separated env var value into a string slice.
// Trims whitespace from each element. Returns nil for empty strings.
func splitEnvList(v string) []string {
	var result []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
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
	if len(c.EnabledSources()) == 0 {
		return fmt.Errorf("at least one source must be enabled")
	}

	if len(c.Notifications) == 0 {
		return fmt.Errorf("at least one notification channel is required")
	}
	for _, n := range c.Notifications {
		if n != "email" {
			return fmt.Errorf("unknown notification channel: %s (use email)", n)
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
		}
	}
	return nil
}
