package main

import (
	"flag"
	"html/template"
	"log"
	"os"
	"time"

	"firstbyte/config"
	"firstbyte/feed"
	"firstbyte/filter"
	"firstbyte/notify"
	"firstbyte/store"
)

func main() {
	// parse flags
	dryRun := flag.Bool("dry-run", false, "Render the digest to stdout without sending")
	testMode := flag.Bool("test", false, "Send a test message to verify credentials")
	flag.Parse()

	log.SetFlags(0)

	// load and validate configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}

	// load secrets from .env or environment
	secrets, err := config.LoadSecrets(".env")
	if err != nil {
		log.Fatalf("secrets: %v", err)
	}
	if err := secrets.Validate(cfg.Notifications); err != nil {
		log.Fatalf("secrets: %v", err)
	}

	// --test mode: send test message and exit
	if *testMode {
		log.Println("Sending test message...")
		if err := notify.SendTestEmail(cfg.Email, *secrets); err != nil {
			log.Fatalf("test: %v", err)
		}
		log.Println("Test message sent.")
		return
	}

	// open the seen-article store (skip in --dry-run)
	var seen *store.Store
	if !*dryRun {
		s, err := store.New(cfg.Store.Path)
		if err != nil {
			log.Fatalf("store: %v", err)
		}
		seen = s
		defer func() {
			if err := seen.Save(); err != nil {
				log.Printf("store save: %v", err)
			}
		}()
	}

	// fetch all feeds concurrently
	log.Println("Fetching feeds...")
	articles, errs := feed.FetchAll(cfg.Sources)
	for _, e := range errs {
		log.Println("warning:", e)
	}
	log.Printf("Fetched %d articles from %d sources", len(articles), len(cfg.Sources))

	// build per-source limits from config
	limits := make(map[string]int)
	for _, s := range cfg.Sources {
		limits[s.Name] = s.MaxArticles
	}

	// filter pipeline: remove seen → top N per source
	fresh := articles
	if seen != nil {
		fresh = filter.RemoveSeen(articles, seen)
	}
	result := filter.TopNPerSourceMap(fresh, limits)

	// mark kept articles as seen
	if seen != nil {
		for _, a := range result {
			seen.Mark(a.Link)
		}
	}

	if len(result) == 0 {
		log.Println("No new articles today.")
		return
	}

	log.Printf("Digest: %d articles across sources\n", len(result))

	// group articles by source for templates
	groups := notify.GroupArticles(result)
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
			log.Fatalf("template: %v", err)
		}
		if err := tmpl.Execute(os.Stdout, data); err != nil {
			log.Fatalf("template: %v", err)
		}
		return
	}

	// send notifications
	for _, ch := range cfg.Notifications {
		switch ch {
		case "email":
			log.Println("Sending email...")
			if err := notify.SendEmail(cfg.Email, *secrets, data, "template"); err != nil {
				log.Printf("email: %v", err)
			} else {
				log.Println("Email sent.")
			}
		}
	}
}
