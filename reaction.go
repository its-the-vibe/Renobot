package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

// SlackClient is the interface for Slack API calls used by the reaction handler.
// *slack.Client satisfies this interface.
type SlackClient interface {
	GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
}

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

// ReactionEvent is the raw Slack reaction_added event callback payload published
// to the pub/sub channel. It does NOT include message metadata — that must be
// fetched separately via the Slack API using the item channel and timestamp.
type ReactionEvent struct {
	Event SlackReactionEvent `json:"event"`
}

// listenReactionEvents subscribes to the configured Slack reaction pub/sub
// channel and processes each event until ctx is cancelled.
func listenReactionEvents(ctx context.Context, cfg *Config, rdb *redis.Client, slackClient SlackClient) {
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
			handleReactionEvent(ctx, cfg, rdb, slackClient, msg.Payload)
		case <-ctx.Done():
			return
		}
	}
}

// handleReactionEvent processes a single reaction event from the pub/sub
// channel. It looks up the original Slack message metadata via the Slack API
// to verify the message is a Renobot summary, then dispatches the appropriate
// revamp merge command via Poppit.
func handleReactionEvent(ctx context.Context, cfg *Config, rdb *redis.Client, slackClient SlackClient, raw string) {
	var evt ReactionEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		log.Printf("Error parsing reaction event: %v", err)
		return
	}

	if evt.Event.Type != "reaction_added" {
		return
	}

	channel := evt.Event.Item.Channel
	ts := evt.Event.Item.Ts

	meta, err := fetchMessageMetadata(slackClient, channel, ts)
	if err != nil {
		log.Printf("Error fetching message metadata for ts=%s: %v", ts, err)
		return
	}
	if meta == nil {
		return
	}

	msgType, _ := meta.EventPayload["type"].(string)
	if !strings.EqualFold(msgType, "renobot") {
		return
	}

	branch, _ := meta.EventPayload["branch"].(string)
	if branch == "" {
		log.Printf("Reaction event missing branch in metadata, skipping (ts=%s)", ts)
		return
	}

	cmd, err := buildRevampCommand(cfg, evt.Event.Reaction, branch)
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
			"thread_ts": ts,
			"channel":   channel,
		},
	}

	if err := publishPoppitCommand(ctx, rdb, cfg.Poppit.InputList, poppitPayload); err != nil {
		log.Printf("Error dispatching merge command for branch %s: %v", branch, err)
		return
	}

	log.Printf("Dispatched merge command %q for branch %s (thread_ts: %s)", cmd, branch, ts)
}

// fetchMessageMetadata retrieves the Slack message at the given channel and
// timestamp and returns its metadata. Returns nil (no error) if the message
// exists but carries no metadata.
func fetchMessageMetadata(slackClient SlackClient, channel, ts string) (*messageMetadata, error) {
	resp, err := slackClient.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID:          channel,
		Latest:             ts,
		Limit:              1,
		Inclusive:          true,
		IncludeAllMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("getting conversation history: %w", err)
	}

	if len(resp.Messages) == 0 {
		return nil, nil
	}

	msg := resp.Messages[0]
	if msg.Metadata.EventType == "" {
		return nil, nil
	}

	meta := &messageMetadata{
		EventType:    msg.Metadata.EventType,
		EventPayload: make(map[string]interface{}, len(msg.Metadata.EventPayload)),
	}
	for k, v := range msg.Metadata.EventPayload {
		meta.EventPayload[k] = v
	}
	return meta, nil
}

// buildRevampCommand constructs the revamp command string for a given
// reaction emoji name and branch. Returns an error for unrecognised reactions.
func buildRevampCommand(cfg *Config, reaction, branch string) (string, error) {
	if reaction == "heart_eyes_cat" {
		return fmt.Sprintf("%s merge --org %s --branch %s", cfg.RevampPath, cfg.Org, branch), nil
	}
	if reaction == "hourglass" {
		return fmt.Sprintf("%s list --org %s --head %s", cfg.RevampPath, cfg.Org, branch), nil
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
