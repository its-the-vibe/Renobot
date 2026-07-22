package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

// PoppitPayload is the JSON notification pushed to Poppit's Redis input list.
// Poppit pops this payload and executes each command in Dir sequentially,
// publishing the output to its configured Redis channel.
type PoppitPayload struct {
	Repo     string                 `json:"repo"`
	Branch   string                 `json:"branch"`
	Type     string                 `json:"type"`
	Dir      string                 `json:"dir"`
	Commands []string               `json:"commands"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PoppitOutput is the JSON message Poppit publishes to its output channel
// for each executed command when the payload includes a metadata field.
type PoppitOutput struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Type     string                 `json:"type"`
	Command  string                 `json:"command"`
	Output   string                 `json:"output"`
	Stderr   string                 `json:"stderr"`
}

// publishPoppitCommand serialises payload and pushes it to Poppit's Redis
// input list using RPUSH (FIFO queue; Poppit pops with BLPOP from the left).
func publishPoppitCommand(ctx context.Context, rdb *redis.Client, listKey string, payload PoppitPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling Poppit payload: %w", err)
	}
	if err := rdb.RPush(ctx, listKey, string(data)).Err(); err != nil {
		return fmt.Errorf("pushing to Poppit list %q: %w", listKey, err)
	}
	return nil
}

// listenPoppitOutput subscribes to Poppit's Redis output channel and processes
// each command-output message until ctx is cancelled.
func listenPoppitOutput(ctx context.Context, cfg *Config, rdb *redis.Client) {
	sub := rdb.Subscribe(ctx, cfg.Poppit.OutputChannel)
	defer sub.Close()

	log.Printf("Listening for Poppit output on channel %q", cfg.Poppit.OutputChannel)

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handlePoppitOutput(ctx, cfg, rdb, msg.Payload)
		case <-ctx.Done():
			return
		}
	}
}

// handlePoppitOutput processes a single JSON message from Poppit's output
// channel. It dispatches further Poppit commands or publishes Slack summaries
// depending on whether the message is from a "--branch" or "--head" invocation.
func handlePoppitOutput(ctx context.Context, cfg *Config, rdb *redis.Client, raw string) {
	var out PoppitOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Printf("Error parsing Poppit output: %v", err)
		return
	}

	// Only handle messages produced by Renobot-originated commands.
	if out.Type != "Renobot" {
		return
	}

	if out.Stderr != "" {
		log.Printf("Poppit command %q produced stderr: %s", out.Command, out.Stderr)
	}

	switch {
	case strings.Contains(out.Command, " merge "):
		branch, _ := out.Metadata["branch"].(string)
		threadTs, _ := out.Metadata["thread_ts"].(string)
		channel, _ := out.Metadata["channel"].(string)
		if threadTs == "" || channel == "" {
			log.Printf("Missing thread_ts or channel in metadata for merge command %q", out.Command)
			return
		}
		text := strings.TrimSpace(out.Output)
		if text == "" {
			text = fmt.Sprintf("Merge completed for branch %s", branch)
		}
		if err := publishThreadReply(ctx, rdb, cfg.Redis.ListKey, channel, threadTs, text); err != nil {
			log.Printf("Error publishing merge reply for branch %s: %v", branch, err)
		}

	case strings.Contains(out.Command, "--branch"):
		headPayloads := buildHeadPayloads(cfg, out.Output)
		if len(headPayloads) == 0 {
			log.Println("No open Renovate branches found in Poppit output")
			return
		}
		for _, p := range headPayloads {
			branch, _ := p.Metadata["branch"].(string)
			if err := publishPoppitCommand(ctx, rdb, cfg.Poppit.InputList, p); err != nil {
				log.Printf("Error publishing head command for branch %s: %v", branch, err)
			}
		}

	case strings.Contains(out.Command, "--head"):
		if strings.HasPrefix(out.Command, fmt.Sprintf("%s summary", cfg.RevampPath)) {
			handleBranchSummary(ctx, cfg, rdb, out)
		}

		if strings.HasPrefix(out.Command, fmt.Sprintf("%s list", cfg.RevampPath)) {
			handleListOutput(ctx, cfg, rdb, out)
		}

	}
}

func handleBranchSummary(ctx context.Context, cfg *Config, rdb *redis.Client, out PoppitOutput) {
	branch, _ := out.Metadata["branch"].(string)
	countFloat, ok := out.Metadata["count"].(float64)
	if branch == "" {
		log.Printf("Missing branch in metadata for command %q", out.Command)
		return
	}
	if !ok {
		log.Printf("Missing or invalid count in metadata for command %q", out.Command)
	}
	summary := BranchSummary{Branch: branch, Count: int(countFloat)}
	repos := parseRepoOutput(out.Output)
	if err := publishSummary(ctx, rdb, cfg.Redis.ListKey, cfg.Channel, summary, repos, cfg.SlackTTL); err != nil {
		log.Printf("Error publishing summary for branch %s: %v", branch, err)
	}
}

func handleListOutput(ctx context.Context, cfg *Config, rdb *redis.Client, out PoppitOutput) {
	type listItem struct {
		URL string `json:"url"`
	}
	var urls []listItem

	if err := json.Unmarshal([]byte(out.Output), &urls); err != nil {
		log.Printf("Error parsing list output: %v", err)
		return
	}

	for _, item := range urls {
		if err := rdb.RPush(ctx, cfg.OrderlyQueue, item.URL).Err(); err != nil {
			log.Printf("Error pushing URL to OrderlyQueue: %v", err)
		}
	}

	log.Printf("Pushed %d URLs to OrderlyQueue", len(urls))

	threadTs, _ := out.Metadata["thread_ts"].(string)
	channel, _ := out.Metadata["channel"].(string)
	if threadTs == "" || channel == "" {
		return
	}

	branch, _ := out.Metadata["branch"].(string)
	rawURLs := make([]string, len(urls))
	for i, item := range urls {
		rawURLs[i] = item.URL
	}
	text := formatListReply(branch, rawURLs)

	if err := publishThreadReply(ctx, rdb, cfg.Redis.ListKey, channel, threadTs, text); err != nil {
		log.Printf("Error publishing list reply for branch %s: %v", branch, err)
	}
}

// formatListReply builds the Slack thread-reply text for a revamp list result.
// It lists each queued PR URL as a Slack hyperlink, or reports that no PRs were found.
func formatListReply(branch string, urls []string) string {
	if len(urls) == 0 {
		return fmt.Sprintf("No Renovate PRs found for branch %s", branch)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d Renovate PR%s queued for branch %s", len(urls), pluralS(len(urls)), branch)
	for _, u := range urls {
		fmt.Fprintf(&sb, "\n• <%s>", u)
	}
	return sb.String()
}

// buildHeadPayloads parses branch-list output and returns one PoppitPayload
// per branch, each carrying the branch name and PR count in its metadata so
// they can be recovered when the corresponding output message arrives.
func buildHeadPayloads(cfg *Config, branchOutput string) []PoppitPayload {
	branches := parseBranchOutput(branchOutput)
	payloads := make([]PoppitPayload, 0, len(branches))
	for _, b := range branches {
		cmd := fmt.Sprintf("%s summary --org %s --head %s", cfg.RevampPath, cfg.Org, b.Branch)
		payloads = append(payloads, PoppitPayload{
			Repo:     cfg.Poppit.Repo,
			Branch:   cfg.Poppit.Branch,
			Type:     "Renobot",
			Dir:      cfg.Poppit.BaseDir,
			Commands: []string{cmd},
			Metadata: map[string]interface{}{
				"type":   "Renobot",
				"branch": b.Branch,
				"count":  float64(b.Count),
			},
		})
	}
	return payloads
}
