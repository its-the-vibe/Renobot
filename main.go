package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", cfg.Redis.Addr, err)
	}
	log.Printf("Connected to Redis at %s", cfg.Redis.Addr)

	c := cron.New()
	_, err = c.AddFunc(cfg.Cron, func() {
		runSummary(ctx, cfg, rdb)
	})
	if err != nil {
		log.Fatalf("Failed to register cron job: %v", err)
	}
	c.Start()
	defer c.Stop()

	log.Printf("Renobot started. Org: %s, Channel: %s, Schedule: %s", cfg.Org, cfg.Channel, cfg.Cron)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

// runSummary is the core job: fetch branch data via revamp and publish to Slack.
func runSummary(ctx context.Context, cfg *Config, rdb *redis.Client) {
	log.Printf("Running summary for org %s...", cfg.Org)

	branches, err := runRevampBranches(cfg.RevampPath, cfg.Org)
	if err != nil {
		log.Printf("Error fetching branch list: %v", err)
		return
	}

	if len(branches) == 0 {
		log.Println("No open Renovate branches found")
		return
	}

	for _, b := range branches {
		repos, err := runRevampRepos(cfg.RevampPath, cfg.Org, b.Branch)
		if err != nil {
			log.Printf("Error fetching repos for branch %s: %v", b.Branch, err)
			continue
		}

		if err := publishSummary(ctx, rdb, cfg.Redis.ListKey, cfg.Channel, b, repos); err != nil {
			log.Printf("Error publishing summary for branch %s: %v", b.Branch, err)
		}
	}

	log.Printf("Summary run complete. Processed %d branch(es).", len(branches))
}
