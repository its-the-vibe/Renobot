package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// slackMessage is the payload accepted by SlackLiner's Redis queue.
type slackMessage struct {
	Channel  string           `json:"channel"`
	Text     string           `json:"text"`
	ThreadTs string           `json:"thread_ts,omitempty"`
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
// Each repository is rendered as a Slack hyperlink pointing directly to the
// open Renovate PRs for that branch in the repository.
func formatMessage(summary BranchSummary, repos []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%s* (%d open PR%s)\n", summary.Branch, summary.Count, pluralS(summary.Count))
	for _, repo := range repos {
		params := url.Values{"q": {"head:" + summary.Branch + " is:open"}}
		prURL := "https://github.com/" + repo + "/pulls?" + params.Encode()
		fmt.Fprintf(&sb, "• <%s|%s>\n", prURL, repo)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// publishThreadReply pushes a thread reply message to the SlackLiner Redis
// queue. The reply is threaded under the message identified by threadTs.
func publishThreadReply(ctx context.Context, rdb *redis.Client, listKey, channel, threadTs, text string) error {
	msg := slackMessage{
		Channel:  channel,
		Text:     text,
		ThreadTs: threadTs,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling thread reply: %w", err)
	}

	if err := rdb.RPush(ctx, listKey, string(payload)).Err(); err != nil {
		return fmt.Errorf("pushing thread reply to Redis list %q: %w", listKey, err)
	}

	log.Printf("Published thread reply to channel %s (thread_ts: %s)", channel, threadTs)
	return nil
}
