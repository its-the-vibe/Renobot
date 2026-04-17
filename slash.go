package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// SlackCommand is the slash command payload published to the Redis pub/sub
// channel by SlackCommandRelay when a user issues a slash command in Slack.
type SlackCommand struct {
	Command   string `json:"command"`
	Text      string `json:"text"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// listenSlashCommands subscribes to the configured Slack slash command pub/sub
// channel and processes each event until ctx is cancelled.
func listenSlashCommands(ctx context.Context, cfg *Config, rdb *redis.Client) {
	sub := rdb.Subscribe(ctx, cfg.Slack.SlashCommandChannel)
	defer sub.Close()

	log.Printf("Listening for Slack slash commands on channel %q", cfg.Slack.SlashCommandChannel)

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handleSlashCommand(ctx, cfg, rdb, msg.Payload)
		case <-ctx.Done():
			return
		}
	}
}

// handleSlashCommand processes a single slash command event from the pub/sub
// channel. It filters for /renobot commands and triggers summary publishing.
func handleSlashCommand(ctx context.Context, cfg *Config, rdb *redis.Client, raw string) {
	var cmd SlackCommand
	if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
		log.Printf("Error parsing slash command: %v", err)
		return
	}

	if cmd.Command != "/renobot" {
		return
	}

	log.Printf("Received /renobot slash command from user %s in channel %s", cmd.UserID, cmd.ChannelID)

	runSummary(ctx, cfg, rdb)
}
