package main

import (
	"log"
	"time"

	"firstbyte/config"
	"firstbyte/feed"
	"firstbyte/filter"
	"firstbyte/notify"
	"firstbyte/store"
)

func main() {
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

	// open the seen-article store
	seen, err := store.New(cfg.Store.Path)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() {
		if err := seen.Save(); err != nil {
			log.Printf("store save: %v", err)
		}
	}()

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
	fresh := filter.RemoveSeen(articles, seen)
	result := filter.TopNPerSourceMap(fresh, limits)

	// mark kept articles as seen
	for _, a := range result {
		seen.Mark(a.Link)
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
