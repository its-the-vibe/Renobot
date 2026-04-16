package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

// SlackReactionItem represents the Slack message that was reacted to.
type SlackReactionItem struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
}

// SlackReactionEvent is the inner event object from a Slack reaction_added event.
type SlackReactionEvent struct {
	Type     string            `json:"type"`
	Reaction string            `json:"reaction"`
	Item     SlackReactionItem `json:"item"`
	EventTs  string            `json:"event_ts"`
	User     string            `json:"user"`
}

// ReactionEvent is the enriched payload published to the reaction pub/sub
// channel by SlackLiner when a user reacts to a message. It pairs the raw
// Slack reaction event with the metadata of the reacted-to message so that
// Renobot can verify the message origin and dispatch the correct command.
type ReactionEvent struct {
	Event    SlackReactionEvent `json:"event"`
	Metadata *messageMetadata   `json:"metadata,omitempty"`
}

// listenReactionEvents subscribes to the configured Slack reaction pub/sub
// channel and processes each event until ctx is cancelled.
func listenReactionEvents(ctx context.Context, cfg *Config, rdb *redis.Client) {
	sub := rdb.Subscribe(ctx, cfg.Slack.ReactionChannel)
	defer sub.Close()

	log.Printf("Listening for Slack reaction events on channel %q", cfg.Slack.ReactionChannel)

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handleReactionEvent(ctx, cfg, rdb, msg.Payload)
		case <-ctx.Done():
			return
		}
	}
}

// handleReactionEvent processes a single reaction event from the pub/sub
// channel. It verifies the event is for a Renobot-generated message and
// dispatches the appropriate revamp merge command via Poppit.
func handleReactionEvent(ctx context.Context, cfg *Config, rdb *redis.Client, raw string) {
	var evt ReactionEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		log.Printf("Error parsing reaction event: %v", err)
		return
	}

	if evt.Event.Type != "reaction_added" {
		return
	}

	if evt.Metadata == nil {
		return
	}

	msgType, _ := evt.Metadata.EventPayload["type"].(string)
	if !strings.EqualFold(msgType, "renobot") {
		return
	}

	branch, _ := evt.Metadata.EventPayload["branch"].(string)
	if branch == "" {
		log.Printf("Reaction event missing branch in metadata, skipping")
		return
	}

	threadTs := evt.Event.Item.Ts
	channel := evt.Event.Item.Channel

	cmd, err := buildMergeCommand(cfg, evt.Event.Reaction, branch)
	if err != nil {
		log.Printf("Unsupported reaction %q for branch %s, skipping", evt.Event.Reaction, branch)
		return
	}

	poppitPayload := PoppitPayload{
		Repo:     cfg.Poppit.Repo,
		Branch:   cfg.Poppit.Branch,
		Type:     "Renobot",
		Dir:      cfg.Poppit.BaseDir,
		Commands: []string{cmd},
		Metadata: map[string]interface{}{
			"type":      "Renobot",
			"branch":    branch,
			"thread_ts": threadTs,
			"channel":   channel,
		},
	}

	if err := publishPoppitCommand(ctx, rdb, cfg.Poppit.InputList, poppitPayload); err != nil {
		log.Printf("Error dispatching merge command for branch %s: %v", branch, err)
		return
	}

	log.Printf("Dispatched merge command %q for branch %s (thread_ts: %s)", cmd, branch, threadTs)
}

// buildMergeCommand constructs the revamp merge command string for a given
// reaction emoji name and branch. Returns an error for unrecognised reactions.
func buildMergeCommand(cfg *Config, reaction, branch string) (string, error) {
	if reaction == "heart_eyes_cat" {
		return fmt.Sprintf("%s merge --org %s --branch %s", cfg.RevampPath, cfg.Org, branch), nil
	}
	if max, ok := reactionToNumber(reaction); ok {
		return fmt.Sprintf("%s merge --org %s --branch %s --max %d", cfg.RevampPath, cfg.Org, branch, max), nil
	}
	return "", fmt.Errorf("unrecognised reaction %q", reaction)
}

// reactionToNumber maps a Slack number-emoji name to its integer value.
// Returns (0, false) for non-numeric reactions.
func reactionToNumber(reaction string) (int, bool) {
	numbers := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
		"five":  5,
		"six":   6,
		"seven": 7,
		"eight": 8,
		"nine":  9,
	}
	n, ok := numbers[reaction]
	return n, ok
}
