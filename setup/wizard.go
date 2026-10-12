// Package setup implements the interactive --init wizard that writes config.yaml.
package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"firstbyte/config"
)

// ErrCancelled is returned when the user aborts the wizard (e.g. Ctrl+C).
var ErrCancelled = huh.ErrUserAborted

// Run launches the interactive wizard and writes the result to configPath.
// It starts from the existing config, falling back to the example config.
func Run(configPath, examplePath string) error {
	// load starting values
	cfg, existing, err := loadStartingConfig(configPath, examplePath)
	if err != nil {
		return err
	}

	// confirm before overwriting an existing config
	if existing {
		ok, err := confirm(fmt.Sprintf("%s already exists. Edit and overwrite it?", configPath))
		if err != nil || !ok {
			return err
		}
	}

	// walk through each step in order
	steps := []func(*config.Config) error{chooseSources, addCustomSources, editFilter, editEmail}
	for _, step := range steps {
		if err := step(cfg); err != nil {
			return err
		}
	}

	// validate and write the file
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := config.Save(cfg, configPath); err != nil {
		return err
	}

	fmt.Printf("Saved %s. Set SMTP_USER and SMTP_PASSWORD in .env, then run ./firstbyte --test\n", configPath)
	return nil
}

// loadStartingConfig reads configPath, or examplePath if it doesn't exist.
// The bool reports whether configPath already existed.
func loadStartingConfig(configPath, examplePath string) (*config.Config, bool, error) {
	// prefer the user's current config
	cfg, err := config.LoadRaw(configPath)
	if err == nil {
		return cfg, true, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}

	// fall back to the example, or start empty
	cfg, err = config.LoadRaw(examplePath)
	if errors.Is(err, fs.ErrNotExist) {
		cfg, err = &config.Config{}, nil
	}
	if err != nil {
		return nil, false, err
	}

	// email is the only channel today
	if len(cfg.Notifications) == 0 {
		cfg.Notifications = []string{"email"}
	}
	return cfg, false, nil
}

// confirm asks a yes/no question.
func confirm(question string) (bool, error) {
	var ok bool
	err := huh.NewConfirm().Title(question).Value(&ok).Run()
	return ok, err
}

// chooseSources shows a checklist of sources and marks unchecked ones disabled.
func chooseSources(cfg *config.Config) error {
	if len(cfg.Sources) == 0 {
		return nil
	}

	// build options, pre-checking enabled sources
	var options []huh.Option[int]
	for i, s := range cfg.Sources {
		options = append(options, huh.NewOption(s.Name, i).Selected(s.IsEnabled()))
	}

	// show the checklist
	var picked []int
	err := huh.NewMultiSelect[int]().
		Title("Which sources should be included?").
		Description("Space to toggle, Enter to confirm. Unchecked sources are kept but disabled.").
		Options(options...).
		Value(&picked).
		Validate(func(v []int) error {
			if len(v) == 0 {
				return errors.New("pick at least one source")
			}
			return nil
		}).
		Run()
	if err != nil {
		return err
	}

	// apply choices: nil means enabled (keeps YAML tidy)
	disabled := false
	for i := range cfg.Sources {
		cfg.Sources[i].Enabled = &disabled
	}
	for _, i := range picked {
		cfg.Sources[i].Enabled = nil
	}
	return nil
}

// addCustomSources lets the user append extra feeds one at a time.
func addCustomSources(cfg *config.Config) error {
	for {
		// ask whether to add another
		more, err := confirm("Add a custom source?")
		if err != nil || !more {
			return err
		}

		// collect the new source's fields
		var name, url, maxStr string
		maxStr = "5"
		err = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Name").Value(&name).Validate(required),
			huh.NewInput().Title("Feed URL").Value(&url).Validate(validURL),
			huh.NewInput().Title("Max articles").Value(&maxStr).Validate(positiveInt),
		)).Run()
		if err != nil {
			return err
		}

		n, _ := strconv.Atoi(maxStr)
		cfg.Sources = append(cfg.Sources, config.Source{Name: name, URL: url, MaxArticles: n})
	}
}

// editFilter edits the age window and keyword lists.
func editFilter(cfg *config.Config) error {
	f := &cfg.Filter

	// prefill with the window currently in effect
	maxAge := initialMaxAge(*f)
	allow := strings.Join(f.AllowList, ", ")
	block := strings.Join(f.BlockList, ", ")

	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Max article age").
			Description("Hours or minutes, e.g. 24h or 90m; only keep articles newer than this (0 = no limit).").
			Value(&maxAge).Validate(validDuration),
		huh.NewInput().Title("Allowlist (optional)").
			Description("Comma-separated; only keep articles matching these.").Value(&allow),
		huh.NewInput().Title("Blocklist (optional)").
			Description("Comma-separated; drop articles matching these.").Value(&block),
	).Title("Filters")).Run()
	if err != nil {
		return err
	}

	// store results, replacing the deprecated day count
	f.MaxAge = strings.TrimSpace(maxAge)
	f.MaxAgeDays = 0
	f.AllowList = splitList(allow)
	f.BlockList = splitList(block)
	return nil
}

// initialMaxAge is the age window to prefill: max_age, else the legacy
// max_age_days converted to hours, else the 24h default.
func initialMaxAge(f config.FilterConfig) string {
	switch {
	case f.MaxAge != "":
		return f.MaxAge
	case f.MaxAgeDays > 0:
		return fmt.Sprintf("%dh", f.MaxAgeDays*24)
	default:
		return "24h"
	}
}

// editEmail edits SMTP and recipient settings (secrets stay in .env).
func editEmail(cfg *config.Config) error {
	e := &cfg.Email
	port := ""
	if e.SMTPPort > 0 {
		port = strconv.Itoa(e.SMTPPort)
	}
	to := strings.Join(e.To, ", ")

	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("SMTP host").Value(&e.SMTPHost).Validate(required),
		huh.NewInput().Title("SMTP port").Value(&port).Validate(positiveInt),
		huh.NewInput().Title("From address").Value(&e.From).Validate(required),
		huh.NewInput().Title("To address(es)").Description("Comma-separated for multiple recipients.").
			Value(&to).Validate(required),
	).Title("Email")).Run()
	if err != nil {
		return err
	}

	e.SMTPPort, _ = strconv.Atoi(port)
	e.To = splitList(to)
	return nil
}

// input validators

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("required")
	}
	return nil
}

func validURL(s string) error {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return errors.New("must start with http:// or https://")
	}
	return nil
}

func positiveInt(s string) error {
	if n, err := strconv.Atoi(s); err != nil || n <= 0 {
		return errors.New("must be a positive number")
	}
	return nil
}

func validDuration(s string) error {
	s = strings.TrimSpace(s)

	// give a specific hint for the most common mistakes
	switch {
	case s == "":
		return errors.New("enter a time window, e.g. 24h, or 0 for no limit")
	case strings.HasSuffix(s, "d") || strings.HasSuffix(s, "w"):
		return errors.New("days/weeks aren't supported; use hours, e.g. 72h for 3 days")
	}
	if _, err := strconv.Atoi(s); err == nil && s != "0" {
		return fmt.Errorf("add a unit, e.g. %sh for %s hours", s, s)
	}

	// anything else must parse as a Go duration
	if _, err := time.ParseDuration(s); err != nil {
		return errors.New("not a valid time; use e.g. 24h, 12h, or 90m")
	}
	return nil
}

// splitList turns "a, b, ,c" into ["a", "b", "c"].
func splitList(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
