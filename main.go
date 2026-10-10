package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"firstbyte/config"
	"firstbyte/feed"
	"firstbyte/filter"
	"firstbyte/logger"
	"firstbyte/notify"
	"firstbyte/setup"
	"firstbyte/store"
)

func main() {
	// parse flags
	dryRun := flag.Bool("dry-run", false, "Render the digest to stdout without sending")
	testMode := flag.Bool("test", false, "Send a test message to verify credentials")
	initMode := flag.Bool("init", false, "Launch the interactive setup wizard to create or edit config.yaml")
	verbose := flag.Bool("verbose", false, "Enable debug-level logging")
	logFormat := flag.String("log-format", "text", "Log format: text or json")

	// custom --help output: short intro, then the standard flag list
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "FirstByte: fetch your RSS feeds and email a daily digest.")
		fmt.Fprintln(out, "\nUsage: firstbyte [flags]")
		fmt.Fprintln(out, "\nReads config.yaml and .env from the current directory. Run with --init to create config.yaml.")
		fmt.Fprintln(out, "\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	// initialize logger
	level := logger.LevelInfo
	if *verbose {
		level = logger.LevelDebug
	}
	log := logger.New(level, strings.ToLower(*logFormat) == "json")
	logger.Default = log

	// --init mode: run the setup wizard and exit (before loading config, which may not exist yet)
	if *initMode {
		err := setup.Run("config.yaml", "config.example.yaml")
		if errors.Is(err, setup.ErrCancelled) {
			log.Info("Setup cancelled; config.yaml unchanged.")
			return
		}
		if err != nil {
			log.Fatal("init: %v", err)
		}
		return
	}

	// load and validate configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal("config: %v", err)
	}

	// load secrets from .env or environment
	secrets, err := config.LoadSecrets(".env")
	if err != nil {
		log.Fatal("secrets: %v", err)
	}
	if err := secrets.Validate(cfg.Notifications); err != nil {
		log.Fatal("secrets: %v", err)
	}

	// --test mode: send test message and exit
	if *testMode {
		log.Info("Sending test message...")
		if err := notify.SendTestEmail(cfg.Email, *secrets); err != nil {
			log.Fatal("test: %v", err)
		}
		log.Info("Test message sent.")
		return
	}

	// open the seen-article store (skip in --dry-run)
	var seen *store.Store
	if !*dryRun {
		s, err := store.New(cfg.Store.Path)
		if err != nil {
			log.Fatal("store: %v", err)
		}
		seen = s
		defer func() {
			if err := seen.Save(); err != nil {
				log.Warn("store save: %v", err)
			}
		}()
	}

	// fetch all enabled feeds concurrently
	sources := cfg.EnabledSources()
	log.Info("Fetching feeds...")
	articles, errs := feed.FetchAll(sources)
	for _, e := range errs {
		log.Warn("warning: %v", e)
	}
	log.Info("Fetched %d articles from %d sources", len(articles), len(sources))

	// build per-source limits and the display order from config
	limits := make(map[string]int)
	var sourceOrder []string
	for _, s := range sources {
		limits[s.Name] = s.MaxArticles
		sourceOrder = append(sourceOrder, s.Name)
	}

	// filter pipeline: keywords → max age → remove seen → deduplicate → top N per source
	fresh := filter.FilterByKeywords(articles, cfg.Filter.AllowList, cfg.Filter.BlockList)
	fresh = filter.FilterByMaxAge(fresh, cfg.Filter.MaxAgeDuration)
	if seen != nil {
		fresh = filter.RemoveSeen(fresh, seen)
	}
	fresh = filter.Deduplicate(fresh)
	result := filter.TopNPerSourceMap(fresh, limits)

	// mark kept articles as seen
	if seen != nil {
		for _, a := range result {
			seen.Mark(a.Link)
		}
	}

	if len(result) == 0 {
		log.Info("No new articles today.")
		return
	}

	log.Info("Digest: %d articles across sources", len(result))

	// group articles by source for templates
	groups := notify.GroupArticles(result, sourceOrder)
	date := time.Now().Format("Monday, January 2, 2006")
	data := notify.DigestData{
		Date:     date,
		Articles: groups,
	}

	// --dry-run mode: render to stdout instead of sending
	if *dryRun {
		tmplPath := "template/email.html"
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			log.Fatal("template: %v", err)
		}
		if err := tmpl.Execute(os.Stdout, data); err != nil {
			log.Fatal("template: %v", err)
		}
		return
	}

	// send notifications
	for _, ch := range cfg.Notifications {
		switch ch {
		case "email":
			log.Info("Sending email...")
			if err := notify.SendEmail(cfg.Email, *secrets, data, "template"); err != nil {
				log.Error("email: %v", err)
			} else {
				log.Info("Email sent.")
			}
		}
	}
}
