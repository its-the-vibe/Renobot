package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// slackMessage is the payload accepted by SlackLiner's Redis queue.
type slackMessage struct {
	Channel  string           `json:"channel"`
	Text     string           `json:"text"`
	Metadata *messageMetadata `json:"metadata,omitempty"`
	TTL      int64            `json:"ttl,omitempty"`
}

// messageMetadata mirrors SlackLiner's MessageMetadata type.
type messageMetadata struct {
	EventType    string                 `json:"event_type"`
	EventPayload map[string]interface{} `json:"event_payload"`
}

// publishSummary formats and pushes a branch summary message to the SlackLiner
// Redis queue. The message text lists the repos associated with the branch, and
// the metadata carries the branch name, type, and TTL for future handling.
func publishSummary(ctx context.Context, rdb *redis.Client, listKey, channel string, summary BranchSummary, repos []string, ttl time.Duration) error {
	text := formatMessage(summary, repos)

	ttlSeconds := int64(ttl.Seconds())

	msg := slackMessage{
		Channel: channel,
		Text:    text,
		TTL:     ttlSeconds,
		Metadata: &messageMetadata{
			EventType: "renobot",
			EventPayload: map[string]interface{}{
				"type":   "renobot",
				"branch": summary.Branch,
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if err := rdb.RPush(ctx, listKey, string(payload)).Err(); err != nil {
		return fmt.Errorf("pushing to Redis list %q: %w", listKey, err)
	}

	log.Printf("Published summary for branch %s (%d repo(s)) to channel %s (TTL: %s)", summary.Branch, len(repos), channel, ttl)
	return nil
}

// formatMessage builds the Slack message text for a branch summary.
func formatMessage(summary BranchSummary, repos []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%s* (%d open PR%s)\n", summary.Branch, summary.Count, pluralS(summary.Count))
	for _, repo := range repos {
		fmt.Fprintf(&sb, "• `%s`\n", repo)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
