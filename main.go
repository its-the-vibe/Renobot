package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/slack-go/slack"
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

	// Start background listener for Poppit command output.
	go listenPoppitOutput(ctx, cfg, rdb)

	// Start background listener for Slack emoji reaction events.
	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackToken == "" {
		log.Println("Warning: SLACK_BOT_TOKEN is not set; emoji reaction handling will not work")
	}
	slackClient := slack.New(slackToken)
	go listenReactionEvents(ctx, cfg, rdb, slackClient)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

// runSummary is the core job: publish a revamp branch-list command to Poppit.
// Poppit executes the command and publishes output to its configured Redis
// channel, where listenPoppitOutput picks it up and drives the rest of the
// summary flow (per-branch repo fetches and Slack publishing).
func runSummary(ctx context.Context, cfg *Config, rdb *redis.Client) {
	log.Printf("Running summary for org %s...", cfg.Org)

	cmd := fmt.Sprintf("%s summary --org %s --branch", cfg.RevampPath, cfg.Org)
	payload := PoppitPayload{
		Repo:     cfg.Poppit.Repo,
		Branch:   cfg.Poppit.Branch,
		Type:     "Renobot",
		Dir:      cfg.Poppit.BaseDir,
		Commands: []string{cmd},
		Metadata: map[string]interface{}{
			"type": "Renobot",
		},
	}

	if err := publishPoppitCommand(ctx, rdb, cfg.Poppit.InputList, payload); err != nil {
		log.Printf("Error publishing branch command to Poppit: %v", err)
		return
	}

	log.Printf("Published revamp branch command to Poppit list %q", cfg.Poppit.InputList)
}
